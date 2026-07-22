package modbus

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ByteOrder governs the two bytes *within* one 16-bit register. The spec
// fixes this at big-endian, high byte first (Modbus Application Protocol
// Specification V1.1b3 §4.2: "data is packed... with the most significant
// bit sent first"), but enough legacy and semi-compliant field devices
// byte-swap individual registers that this must be a knob, not an
// assumption.
type ByteOrder uint8

const (
	BigEndian ByteOrder = iota
	LittleEndian
)

// WordOrder governs which of two consecutive registers holds the more
// significant 16 bits of a 32-bit value. Nothing in the Modbus spec
// fixes this — 32-bit types are a convention layered on top of the
// 16-bit register model, and vendors split roughly evenly. otcat names
// the two options after what they are, not after a vendor:
type WordOrder uint8

const (
	HighWordFirst WordOrder = iota // register[0] = most-significant word
	LowWordFirst                   // register[0] = least-significant word ("word-swapped")
)

// DataType is the scalar interpretation applied to one or two raw
// registers. Bit tables (coils, discrete inputs) are handled separately
// in driver.go — they are not "registers" and gain nothing from a codec
// built around word combination.
type DataType uint8

const (
	TypeUint16 DataType = iota
	TypeInt16
	TypeUint32
	TypeInt32
	TypeFloat32
)

// RegisterCount is how many consecutive 16-bit registers a value of this
// type consumes on the wire.
func (t DataType) RegisterCount() int {
	switch t {
	case TypeUint16, TypeInt16:
		return 1
	case TypeUint32, TypeInt32, TypeFloat32:
		return 2
	default:
		return 0
	}
}

func (t DataType) String() string {
	switch t {
	case TypeUint16:
		return "uint16"
	case TypeInt16:
		return "int16"
	case TypeUint32:
		return "uint32"
	case TypeInt32:
		return "int32"
	case TypeFloat32:
		return "float32"
	default:
		return "unknown"
	}
}

func ParseDataType(s string) (DataType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "uint16", "u16", "word":
		return TypeUint16, nil
	case "int16", "i16", "short":
		return TypeInt16, nil
	case "uint32", "u32", "dword":
		return TypeUint32, nil
	case "int32", "i32", "dint", "long":
		return TypeInt32, nil
	case "float32", "f32", "float", "real":
		return TypeFloat32, nil
	default:
		return 0, fmt.Errorf("%w: unknown type %q (want uint16|int16|uint32|int32|float32)", ErrInvalidInput, s)
	}
}

func ParseByteOrder(s string) (ByteOrder, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "big":
		return BigEndian, nil
	case "little":
		return LittleEndian, nil
	default:
		return 0, fmt.Errorf("%w: unknown byte order %q (want big|little)", ErrInvalidInput, s)
	}
}

func ParseWordOrder(s string) (WordOrder, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "high", "high-first":
		return HighWordFirst, nil
	case "low", "low-first", "swap", "swapped":
		return LowWordFirst, nil
	default:
		return 0, fmt.Errorf("%w: unknown word order %q (want high|low)", ErrInvalidInput, s)
	}
}

func swapBytes(r uint16) uint16 { return (r << 8) | (r >> 8) }

// Codec applies one (ByteOrder, WordOrder) pair to convert between raw
// registers and Go scalars. Zero value is the spec-compliant default:
// big-endian bytes, high word first.
type Codec struct {
	ByteOrder ByteOrder
	WordOrder WordOrder
}

func (c Codec) normalize(regs []uint16) []uint16 {
	out := make([]uint16, len(regs))
	copy(out, regs)
	if c.ByteOrder == LittleEndian {
		for i, r := range out {
			out[i] = swapBytes(r)
		}
	}
	if c.WordOrder == LowWordFirst && len(out) == 2 {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

func (c Codec) denormalize(regs []uint16) []uint16 {
	out := make([]uint16, len(regs))
	copy(out, regs)
	if c.WordOrder == LowWordFirst && len(out) == 2 {
		out[0], out[1] = out[1], out[0]
	}
	if c.ByteOrder == LittleEndian {
		for i, r := range out {
			out[i] = swapBytes(r)
		}
	}
	return out
}

// Decode turns raw wire registers into a typed Go scalar.
func (c Codec) Decode(t DataType, regs []uint16) (interface{}, error) {
	want := t.RegisterCount()
	if len(regs) != want {
		return nil, fmt.Errorf("%w: type %s needs %d register(s), got %d", ErrInvalidInput, t, want, len(regs))
	}
	r := c.normalize(regs)
	switch t {
	case TypeUint16:
		return r[0], nil
	case TypeInt16:
		return int16(r[0]), nil
	case TypeUint32:
		return uint32(r[0])<<16 | uint32(r[1]), nil
	case TypeInt32:
		return int32(uint32(r[0])<<16 | uint32(r[1])), nil
	case TypeFloat32:
		bits := uint32(r[0])<<16 | uint32(r[1])
		return math.Float32frombits(bits), nil
	default:
		return nil, fmt.Errorf("modbus: unhandled type %s", t)
	}
}

// Encode turns a literal string into raw wire registers ready to send in
// a write request. Accepts decimal, 0x-prefixed hex, and (for float32)
// standard decimal/scientific float syntax.
func (c Codec) Encode(t DataType, literal string) ([]uint16, error) {
	literal = strings.TrimSpace(literal)
	switch t {
	case TypeUint16:
		v, err := parseUint(literal, 16)
		if err != nil {
			return nil, err
		}
		return c.denormalize([]uint16{uint16(v)}), nil
	case TypeInt16:
		v, err := strconv.ParseInt(literal, 0, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a valid int16: %v", ErrInvalidInput, literal, err)
		}
		if v < math.MinInt16 || v > math.MaxInt16 {
			return nil, fmt.Errorf("%w: %d out of int16 range", ErrInvalidInput, v)
		}
		return c.denormalize([]uint16{uint16(int16(v))}), nil
	case TypeUint32:
		v, err := parseUint(literal, 32)
		if err != nil {
			return nil, err
		}
		return c.denormalize([]uint16{uint16(v >> 16), uint16(v)}), nil
	case TypeInt32:
		v, err := strconv.ParseInt(literal, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a valid int32: %v", ErrInvalidInput, literal, err)
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return nil, fmt.Errorf("%w: %d out of int32 range", ErrInvalidInput, v)
		}
		u := uint32(int32(v))
		return c.denormalize([]uint16{uint16(u >> 16), uint16(u)}), nil
	case TypeFloat32:
		v, err := strconv.ParseFloat(literal, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a valid float32: %v", ErrInvalidInput, literal, err)
		}
		bits := math.Float32bits(float32(v))
		return c.denormalize([]uint16{uint16(bits >> 16), uint16(bits)}), nil
	default:
		return nil, fmt.Errorf("modbus: unhandled type %s", t)
	}
}

func parseUint(s string, bits int) (uint64, error) {
	v, err := strconv.ParseUint(s, 0, bits)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a valid unsigned %d-bit integer: %v", ErrInvalidInput, s, bits, err)
	}
	return v, nil
}
