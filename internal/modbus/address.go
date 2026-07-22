package modbus

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidInput wraps every error caused by malformed operator input
// — an address spec, a table name, a write literal — as opposed to a
// device or network problem. Wrapping (via %w, so errors.Is still
// finds it) lets callers like cliapp map "you typo'd the address" and
// "the PLC is unreachable" to different exit codes without this
// package knowing anything about exit codes itself.
var ErrInvalidInput = errors.New("modbus: invalid input")

// Table identifies one of the four independent data spaces Modbus
// defines. They are genuinely separate address spaces — "holding
// register 5" and "coil 5" share nothing but the number 5 — which is
// why every otcat address spec names its table explicitly instead of
// inferring it from a bare number.
type Table uint8

const (
	TableCoil Table = iota
	TableDiscreteInput
	TableHoldingRegister
	TableInputRegister
)

func (t Table) String() string {
	switch t {
	case TableCoil:
		return "coil"
	case TableDiscreteInput:
		return "discrete"
	case TableHoldingRegister:
		return "holding"
	case TableInputRegister:
		return "input"
	default:
		return "unknown"
	}
}

// IsBits reports whether the table stores single-bit values (coils,
// discrete inputs) as opposed to 16-bit registers.
func (t Table) IsBits() bool {
	return t == TableCoil || t == TableDiscreteInput
}

func (t Table) Writable() bool {
	return t == TableCoil || t == TableHoldingRegister
}

func ParseTable(s string) (Table, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "coil", "coils", "co", "c":
		return TableCoil, nil
	case "discrete", "discreteinput", "discrete_input", "di", "d":
		return TableDiscreteInput, nil
	case "holding", "holdingregister", "holding_register", "hr", "h":
		return TableHoldingRegister, nil
	case "input", "inputregister", "input_register", "ir", "i":
		return TableInputRegister, nil
	default:
		return 0, fmt.Errorf("%w: unknown table %q (want coil|discrete|holding|input)", ErrInvalidInput, s)
	}
}

// classicBase is the leading digit of the five-digit Modicon 984
// reference number field engineers have written on drawings for four
// decades: 0xxxx coils, 1xxxx discrete inputs, 3xxxx input registers,
// 4xxxx holding registers. See classic_addressing.md for the derivation
// and why otcat resolves it the way it does.
func (t Table) classicBase() int {
	switch t {
	case TableCoil:
		return 1
	case TableDiscreteInput:
		return 10001
	case TableInputRegister:
		return 30001
	case TableHoldingRegister:
		return 40001
	default:
		return 0
	}
}

const classicBandWidth = 9999 // a classic reference number is 5 digits: base .. base+9998

// Spec is a fully resolved address: a table, a 0-based wire offset, and
// an optional explicit count (0 = "let the caller decide the default,
// usually 1 or the width of the requested type").
type Spec struct {
	Table   Table
	Address uint16
	Count   int
}

// ParseSpec parses "table:address[:count]". Unless raw is true, a
// numeral that falls inside its table's classic five-digit band is
// translated to a 0-based offset (40001 -> holding register offset 0);
// a numeral outside that band is taken as a literal offset already.
// This dual reading is deliberate — see classic_addressing.md — and
// --raw-address disables it entirely for the rare case of a true wire
// offset that happens to collide with the classic band.
func ParseSpec(s string, raw bool) (Spec, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return Spec{}, fmt.Errorf("%w: bad address %q, want table:address[:count]", ErrInvalidInput, s)
	}
	table, err := ParseTable(parts[0])
	if err != nil {
		return Spec{}, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || n < 0 {
		return Spec{}, fmt.Errorf("%w: bad address number %q in %q", ErrInvalidInput, parts[1], s)
	}

	offset := n
	if !raw {
		base := table.classicBase()
		if n >= base && n <= base+classicBandWidth-1 {
			offset = n - base
		}
	}
	if offset > 0xFFFF {
		return Spec{}, fmt.Errorf("%w: resolved offset %d in %q exceeds 16-bit address range; pass -raw-address if %d was meant literally", ErrInvalidInput, offset, s, n)
	}

	count := 0
	if len(parts) == 3 {
		c, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil || c <= 0 {
			return Spec{}, fmt.Errorf("%w: bad count %q in %q", ErrInvalidInput, parts[2], s)
		}
		count = c
	}
	return Spec{Table: table, Address: uint16(offset), Count: count}, nil
}

func (sp Spec) String() string {
	if sp.Count > 0 {
		return fmt.Sprintf("%s:%d:%d", sp.Table, sp.Address, sp.Count)
	}
	return fmt.Sprintf("%s:%d", sp.Table, sp.Address)
}
