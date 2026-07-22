package modbus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuitOperation/otcat/internal/protocol"
)

// Options configures a Driver's interpretation of every address it
// reads or writes: which unit to talk to, how long to wait, how to
// decode multi-register scalars, and whether classic addressing applies.
type Options struct {
	UnitID     byte
	Timeout    time.Duration
	Type       DataType
	ByteOrder  ByteOrder
	WordOrder  WordOrder
	RawAddress bool
}

func DefaultOptions() Options {
	return Options{
		UnitID:    1,
		Timeout:   5 * time.Second,
		Type:      TypeUint16,
		ByteOrder: BigEndian,
		WordOrder: HighWordFirst,
	}
}

// Driver adapts the Modbus Client to protocol.Driver.
type Driver struct {
	addr   string
	opt    Options
	client *Client
}

func NewDriver(addr string, opt Options) *Driver {
	return &Driver{addr: addr, opt: opt}
}

func (d *Driver) Name() string { return "modbus" }

func (d *Driver) Connect(ctx context.Context) error {
	d.client = NewClient(d.addr, WithUnitID(d.opt.UnitID), WithTimeout(d.opt.Timeout))
	return d.client.Connect(ctx)
}

func (d *Driver) Close() error {
	if d.client == nil {
		return nil
	}
	return d.client.Close()
}

func (d *Driver) Read(ctx context.Context, spec string) (protocol.Value, error) {
	sp, err := ParseSpec(spec, d.opt.RawAddress)
	if err != nil {
		return protocol.Value{}, err
	}

	if sp.Table.IsBits() {
		return d.readBits(ctx, sp)
	}
	return d.readRegisters(ctx, sp)
}

func (d *Driver) readBits(ctx context.Context, sp Spec) (protocol.Value, error) {
	count := sp.Count
	if count == 0 {
		count = 1
	}
	var bits []bool
	var err error
	if sp.Table == TableCoil {
		bits, err = d.client.ReadCoils(ctx, sp.Address, count)
	} else {
		bits, err = d.client.ReadDiscreteInputs(ctx, sp.Address, count)
	}
	if err != nil {
		return protocol.Value{}, err
	}

	var val interface{} = bits
	typ := "bool[]"
	if count == 1 {
		val, typ = bits[0], "bool"
	}
	return protocol.Value{
		Address:   sp.String(),
		Type:      typ,
		Value:     val,
		Quality:   protocol.QualityGood,
		Timestamp: time.Now().UTC(),
	}, nil
}

func (d *Driver) readRegisters(ctx context.Context, sp Spec) (protocol.Value, error) {
	width := d.opt.Type.RegisterCount()
	count := sp.Count
	if count == 0 {
		count = width
	}

	var regs []uint16
	var err error
	if sp.Table == TableHoldingRegister {
		regs, err = d.client.ReadHoldingRegisters(ctx, sp.Address, count)
	} else {
		regs, err = d.client.ReadInputRegisters(ctx, sp.Address, count)
	}
	if err != nil {
		return protocol.Value{}, err
	}

	codec := Codec{ByteOrder: d.opt.ByteOrder, WordOrder: d.opt.WordOrder}

	if count != width {
		if count%width != 0 {
			return protocol.Value{}, fmt.Errorf("%w: requested count %d is not a multiple of %s's width (%d registers)", ErrInvalidInput, count, d.opt.Type, width)
		}
		n := count / width
		vals := make([]interface{}, n)
		for i := 0; i < n; i++ {
			v, err := codec.Decode(d.opt.Type, regs[i*width:(i+1)*width])
			if err != nil {
				return protocol.Value{}, err
			}
			vals[i] = v
		}
		return protocol.Value{
			Address:   sp.String(),
			Type:      d.opt.Type.String() + "[]",
			Value:     vals,
			Quality:   protocol.QualityGood,
			Timestamp: time.Now().UTC(),
			Raw:       regs,
		}, nil
	}

	val, err := codec.Decode(d.opt.Type, regs)
	if err != nil {
		return protocol.Value{}, err
	}
	return protocol.Value{
		Address:   sp.String(),
		Type:      d.opt.Type.String(),
		Value:     val,
		Quality:   protocol.QualityGood,
		Timestamp: time.Now().UTC(),
		Raw:       regs,
	}, nil
}

func (d *Driver) Write(ctx context.Context, spec string, literal string) error {
	sp, coils, regs, err := d.encodeWrite(spec, literal)
	if err != nil {
		return err
	}

	if sp.Table == TableCoil {
		if len(coils) == 1 {
			return d.client.WriteSingleCoil(ctx, sp.Address, coils[0])
		}
		return d.client.WriteMultipleCoils(ctx, sp.Address, coils)
	}
	if len(regs) == 1 {
		return d.client.WriteSingleRegister(ctx, sp.Address, regs[0])
	}
	return d.client.WriteMultipleRegisters(ctx, sp.Address, regs)
}

// ExplainWrite performs every step of Write except opening a connection
// and sending bytes, satisfying protocol.DryRunner.
func (d *Driver) ExplainWrite(spec, literal string) (protocol.WritePlan, error) {
	sp, coils, regs, err := d.encodeWrite(spec, literal)
	if err != nil {
		return protocol.WritePlan{}, err
	}
	plan := protocol.WritePlan{
		Driver:  "modbus",
		Address: sp.String(),
		Literal: literal,
	}
	if sp.Table == TableCoil {
		plan.Coils = coils
	} else {
		plan.Type = d.opt.Type.String()
		plan.Registers = regs
	}
	return plan, nil
}

// encodeWrite parses spec and literal into a resolved address plus
// either the coil values or the register words a write would send. It
// performs no I/O, which is what lets ExplainWrite reuse it verbatim.
func (d *Driver) encodeWrite(spec, literal string) (Spec, []bool, []uint16, error) {
	sp, err := ParseSpec(spec, d.opt.RawAddress)
	if err != nil {
		return Spec{}, nil, nil, err
	}
	if !sp.Table.Writable() {
		return Spec{}, nil, nil, fmt.Errorf("%w: table %q is read-only: only coil and holding registers accept writes (Modbus assigns no write function code to discrete inputs or input registers)", ErrInvalidInput, sp.Table)
	}

	if sp.Table == TableCoil {
		values, err := parseBoolList(literal)
		if err != nil {
			return Spec{}, nil, nil, err
		}
		return sp, values, nil, nil
	}

	codec := Codec{ByteOrder: d.opt.ByteOrder, WordOrder: d.opt.WordOrder}
	var allRegs []uint16
	for _, lit := range strings.Split(literal, ",") {
		regs, err := codec.Encode(d.opt.Type, strings.TrimSpace(lit))
		if err != nil {
			return Spec{}, nil, nil, err
		}
		allRegs = append(allRegs, regs...)
	}
	return sp, nil, allRegs, nil
}

func parseBoolList(s string) ([]bool, error) {
	parts := strings.Split(s, ",")
	out := make([]bool, 0, len(parts))
	for _, p := range parts {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "1", "true", "on", "yes":
			out = append(out, true)
		case "0", "false", "off", "no":
			out = append(out, false)
		default:
			return nil, fmt.Errorf("%w: %q is not a valid coil value (want 0|1|true|false|on|off)", ErrInvalidInput, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty coil literal", ErrInvalidInput)
	}
	return out, nil
}

var _ protocol.Driver = (*Driver)(nil)
var _ protocol.DryRunner = (*Driver)(nil)
