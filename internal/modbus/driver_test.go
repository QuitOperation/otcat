package modbus

import (
	"context"
	"testing"

	"github.com/QuitOperation/otcat/internal/mock"
	"github.com/QuitOperation/otcat/internal/protocol"
)

func newTestDriver(t *testing.T, opt Options) (*Driver, *mock.Server) {
	t.Helper()
	s := newTestServer(t)
	if opt == (Options{}) {
		opt = DefaultOptions()
	}
	d := NewDriver(s.Addr(), opt)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutForTests())
	defer cancel()
	if err := d.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d, s
}

func TestDriverReadScalarUint16(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	d, s := newTestDriver(t, opt)
	s.SetHolding(10, 4660)

	v, err := d.Read(context.Background(), "holding:10")
	if err != nil {
		t.Fatal(err)
	}
	if v.Value.(uint16) != 4660 || v.Type != "uint16" {
		t.Fatalf("got %+v", v)
	}
	if v.Quality != protocol.QualityGood {
		t.Fatalf("got quality %v, want good", v.Quality)
	}
}

func TestDriverReadScalarFloat32(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	opt.Type = TypeFloat32
	d, s := newTestDriver(t, opt)
	s.SetHoldingRange(0, []uint16{0x4049, 0x0FDB}) // ~3.14159

	v, err := d.Read(context.Background(), "holding:0:2")
	if err != nil {
		t.Fatal(err)
	}
	f := v.Value.(float32)
	if f < 3.1415 || f > 3.1416 {
		t.Fatalf("got %v, want ~3.14159", f)
	}
	if len(v.Raw) != 2 {
		t.Fatalf("Raw should carry the 2 backing registers, got %v", v.Raw)
	}
}

func TestDriverReadArray(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	d, s := newTestDriver(t, opt)
	s.SetHoldingRange(0, []uint16{1, 2, 3, 4, 5})

	v, err := d.Read(context.Background(), "holding:0:5")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := v.Value.([]interface{})
	if !ok || len(arr) != 5 {
		t.Fatalf("expected 5-element array, got %+v", v.Value)
	}
	if v.Type != "uint16[]" {
		t.Fatalf("got type %q, want uint16[]", v.Type)
	}
}

func TestDriverReadArrayNotMultipleOfWidth(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	opt.Type = TypeFloat32 // width 2
	d, _ := newTestDriver(t, opt)

	_, err := d.Read(context.Background(), "holding:0:3") // 3 is not a multiple of 2
	if err == nil {
		t.Fatal("expected an error: count not a multiple of type width")
	}
}

func TestDriverReadCoilArray(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	d, s := newTestDriver(t, opt)
	s.SetCoil(0, true)
	s.SetCoil(1, false)
	s.SetCoil(2, true)

	v, err := d.Read(context.Background(), "coil:0:3")
	if err != nil {
		t.Fatal(err)
	}
	bits, ok := v.Value.([]bool)
	if !ok || len(bits) != 3 {
		t.Fatalf("expected []bool of length 3, got %+v", v.Value)
	}
	if v.Type != "bool[]" {
		t.Fatalf("got type %q, want bool[]", v.Type)
	}
}

func TestDriverWriteReadOnlyTableRejected(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	d, _ := newTestDriver(t, opt)

	if err := d.Write(context.Background(), "input:0", "5"); err == nil {
		t.Fatal("writing to input registers should be rejected: they are read-only")
	}
	if err := d.Write(context.Background(), "discrete:0", "1"); err == nil {
		t.Fatal("writing to discrete inputs should be rejected: they are read-only")
	}
}

func TestDriverWriteCoilLiteralForms(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	d, s := newTestDriver(t, opt)

	for _, lit := range []string{"1", "true", "on", "yes"} {
		if err := d.Write(context.Background(), "coil:0", lit); err != nil {
			t.Fatalf("write %q: %v", lit, err)
		}
		if !s.GetCoil(0) {
			t.Fatalf("literal %q should have set the coil true", lit)
		}
	}
	for _, lit := range []string{"0", "false", "off", "no"} {
		if err := d.Write(context.Background(), "coil:0", lit); err != nil {
			t.Fatalf("write %q: %v", lit, err)
		}
		if s.GetCoil(0) {
			t.Fatalf("literal %q should have set the coil false", lit)
		}
	}
	if err := d.Write(context.Background(), "coil:0", "maybe"); err == nil {
		t.Fatal("invalid coil literal should be rejected")
	}
}

func TestDriverWriteMultiValueCSV(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	d, s := newTestDriver(t, opt)

	if err := d.Write(context.Background(), "holding:0", "1,2,3"); err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint16{1, 2, 3} {
		if got := s.GetHolding(uint16(i)); got != want {
			t.Fatalf("holding[%d] = %d, want %d", i, got, want)
		}
	}
}

func TestDriverExplainWriteMatchesWrite(t *testing.T) {
	opt := DefaultOptions()
	opt.RawAddress = true
	opt.Type = TypeFloat32
	d, s := newTestDriver(t, opt)

	plan, err := d.ExplainWrite("holding:0", "3.14159")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Registers) != 2 {
		t.Fatalf("expected a 2-register plan for float32, got %v", plan.Registers)
	}
	if plan.Driver != "modbus" {
		t.Fatalf("plan.Driver = %q, want modbus", plan.Driver)
	}

	// The plan must not have touched the network.
	if got := s.GetHolding(0); got != 0 {
		t.Fatalf("ExplainWrite performed an actual write: holding[0] = %d", got)
	}

	// And the same registers, actually written, must decode back to
	// the same value ExplainWrite predicted.
	if err := d.Write(context.Background(), "holding:0", "3.14159"); err != nil {
		t.Fatal(err)
	}
	if got0, got1 := s.GetHolding(0), s.GetHolding(1); got0 != plan.Registers[0] || got1 != plan.Registers[1] {
		t.Fatalf("actual write %04X %04X did not match planned %04X %04X", got0, got1, plan.Registers[0], plan.Registers[1])
	}
}

func TestDriverUnitID(t *testing.T) {
	// The mock server does not police unit IDs, so this test confirms
	// only that a non-default unit ID is accepted end-to-end without
	// error, i.e. that the option actually reaches the wire.
	opt := DefaultOptions()
	opt.RawAddress = true
	opt.UnitID = 7
	d, s := newTestDriver(t, opt)
	s.SetHolding(0, 1)
	if _, err := d.Read(context.Background(), "holding:0"); err != nil {
		t.Fatalf("read with non-default unit id failed: %v", err)
	}
}
