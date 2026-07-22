package modbus

import (
	"encoding/binary"
	"fmt"
)

// Function codes implemented, per Modbus Application Protocol
// Specification V1.1b3 §6. This is the complete "data access" subset —
// the eight function codes that cover every read and write a monitoring
// or commissioning tool needs. Diagnostics (FC 0x08), file record access
// (FC 0x14/0x15), and the FIFO/queue codes are out of scope by design:
// they see negligible field use and each would drag in its own address
// grammar for a use case otcat's audience rarely hits.
const (
	FuncReadCoils              byte = 0x01
	FuncReadDiscreteInputs     byte = 0x02
	FuncReadHoldingRegisters   byte = 0x03
	FuncReadInputRegisters     byte = 0x04
	FuncWriteSingleCoil        byte = 0x05
	FuncWriteSingleRegister    byte = 0x06
	FuncWriteMultipleCoils     byte = 0x0F
	FuncWriteMultipleRegisters byte = 0x10

	exceptionBit byte = 0x80
)

// coilOn / coilOff are the two wire values FC 0x05 permits for a coil.
// Any other 16-bit value is illegal per §6.5 — Modbus deliberately does
// not define a "toggle" or partial-write encoding here.
const (
	coilOn  uint16 = 0xFF00
	coilOff uint16 = 0x0000
)

// --- Requests -----------------------------------------------------------

// encodeReadRequest builds the 5-byte PDU shared by FC 0x01, 0x02, 0x03,
// and 0x04: function code, start address, quantity.
func encodeReadRequest(fc byte, address, quantity uint16) []byte {
	pdu := make([]byte, 5)
	pdu[0] = fc
	binary.BigEndian.PutUint16(pdu[1:3], address)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)
	return pdu
}

func encodeWriteSingleCoil(address uint16, on bool) []byte {
	pdu := make([]byte, 5)
	pdu[0] = FuncWriteSingleCoil
	binary.BigEndian.PutUint16(pdu[1:3], address)
	v := coilOff
	if on {
		v = coilOn
	}
	binary.BigEndian.PutUint16(pdu[3:5], v)
	return pdu
}

func encodeWriteSingleRegister(address, value uint16) []byte {
	pdu := make([]byte, 5)
	pdu[0] = FuncWriteSingleRegister
	binary.BigEndian.PutUint16(pdu[1:3], address)
	binary.BigEndian.PutUint16(pdu[3:5], value)
	return pdu
}

func encodeWriteMultipleRegisters(address uint16, values []uint16) ([]byte, error) {
	n := len(values)
	if n == 0 {
		return nil, fmt.Errorf("modbus: write requires at least one register")
	}
	if n > MaxWriteRegisters {
		return nil, fmt.Errorf("modbus: %d registers exceeds FC16 limit of %d (PDU budget, §6.12)", n, MaxWriteRegisters)
	}
	byteCount := n * 2
	pdu := make([]byte, 6+byteCount)
	pdu[0] = FuncWriteMultipleRegisters
	binary.BigEndian.PutUint16(pdu[1:3], address)
	binary.BigEndian.PutUint16(pdu[3:5], uint16(n))
	pdu[5] = byte(byteCount)
	for i, v := range values {
		binary.BigEndian.PutUint16(pdu[6+2*i:8+2*i], v)
	}
	return pdu, nil
}

func encodeWriteMultipleCoils(address uint16, values []bool) ([]byte, error) {
	n := len(values)
	if n == 0 {
		return nil, fmt.Errorf("modbus: write requires at least one coil")
	}
	if n > MaxWriteBits {
		return nil, fmt.Errorf("modbus: %d coils exceeds FC15 limit of %d (PDU budget, §6.11)", n, MaxWriteBits)
	}
	byteCount := (n + 7) / 8
	pdu := make([]byte, 6+byteCount)
	pdu[0] = FuncWriteMultipleCoils
	binary.BigEndian.PutUint16(pdu[1:3], address)
	binary.BigEndian.PutUint16(pdu[3:5], uint16(n))
	pdu[5] = byte(byteCount)
	for i, v := range values {
		if v {
			pdu[6+i/8] |= 1 << uint(i%8)
		}
	}
	return pdu, nil
}

// --- Responses ------------------------------------------------------------

// asException reports whether pdu is a well-formed exception response
// and, if so, decodes it. The exception bit (function | 0x80) is the one
// piece of Modbus framing that must be checked before anything else
// about the response is assumed — including its length.
func asException(pdu []byte) (ExceptionCode, bool) {
	if len(pdu) >= 2 && pdu[0]&exceptionBit != 0 {
		return ExceptionCode(pdu[1]), true
	}
	return 0, false
}

func decodeReadBitsResponse(pdu []byte, quantity int) ([]bool, error) {
	if len(pdu) < 2 {
		return nil, fmt.Errorf("modbus: short read-bits response (%d bytes)", len(pdu))
	}
	byteCount := int(pdu[1])
	want := (quantity + 7) / 8
	if byteCount != want {
		return nil, fmt.Errorf("modbus: read-bits byte count mismatch: server said %d, quantity %d implies %d", byteCount, quantity, want)
	}
	if len(pdu) < 2+byteCount {
		return nil, fmt.Errorf("modbus: read-bits response truncated: have %d data bytes, want %d", len(pdu)-2, byteCount)
	}
	out := make([]bool, quantity)
	for i := 0; i < quantity; i++ {
		out[i] = pdu[2+i/8]&(1<<uint(i%8)) != 0
	}
	return out, nil
}

func decodeReadRegistersResponse(pdu []byte, quantity int) ([]uint16, error) {
	if len(pdu) < 2 {
		return nil, fmt.Errorf("modbus: short read-registers response (%d bytes)", len(pdu))
	}
	byteCount := int(pdu[1])
	if byteCount != quantity*2 {
		return nil, fmt.Errorf("modbus: read-registers byte count mismatch: server said %d, quantity %d implies %d", byteCount, quantity, quantity*2)
	}
	if len(pdu) < 2+byteCount {
		return nil, fmt.Errorf("modbus: read-registers response truncated: have %d data bytes, want %d", len(pdu)-2, byteCount)
	}
	out := make([]uint16, quantity)
	for i := 0; i < quantity; i++ {
		out[i] = binary.BigEndian.Uint16(pdu[2+2*i : 4+2*i])
	}
	return out, nil
}

// decodeWriteEcho validates the address+value (or address+quantity)
// echo every write response carries per §6.5/§6.6/§6.11/§6.12, and
// rejects silently-wrong echoes rather than trusting the write happened.
func decodeWriteEcho(pdu []byte, wantAddress, wantValue uint16) error {
	if len(pdu) < 5 {
		return fmt.Errorf("modbus: short write response (%d bytes)", len(pdu))
	}
	gotAddress := binary.BigEndian.Uint16(pdu[1:3])
	gotValue := binary.BigEndian.Uint16(pdu[3:5])
	if gotAddress != wantAddress {
		return fmt.Errorf("modbus: write echo address mismatch: sent %d, server echoed %d", wantAddress, gotAddress)
	}
	if gotValue != wantValue {
		return fmt.Errorf("modbus: write echo value mismatch: sent %d, server echoed %d", wantValue, gotValue)
	}
	return nil
}
