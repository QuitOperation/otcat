package modbus

import (
	"encoding/binary"
	"fmt"
)

// mbapLen is the fixed 7-byte header every Modbus TCP ADU carries ahead
// of its PDU: transaction ID (2) + protocol ID (2) + length (2) + unit
// ID (1). Modbus Messaging on TCP/IP Implementation Guide V1.0b, §4.1.
const mbapLen = 7

// protocolID is always 0 for Modbus; other values are reserved and a
// well-behaved client should refuse to interpret them rather than guess.
const protocolID = 0x0000

type mbapHeader struct {
	transactionID uint16
	protocolID    uint16
	length        uint16 // byte count of everything AFTER this field: unitID + PDU
	unitID        byte
}

func encodeMBAP(h mbapHeader) []byte {
	b := make([]byte, mbapLen)
	binary.BigEndian.PutUint16(b[0:2], h.transactionID)
	binary.BigEndian.PutUint16(b[2:4], h.protocolID)
	binary.BigEndian.PutUint16(b[4:6], h.length)
	b[6] = h.unitID
	return b
}

func decodeMBAP(b []byte) (mbapHeader, error) {
	if len(b) < mbapLen {
		return mbapHeader{}, fmt.Errorf("modbus: short MBAP header: got %d bytes, want %d", len(b), mbapLen)
	}
	h := mbapHeader{
		transactionID: binary.BigEndian.Uint16(b[0:2]),
		protocolID:    binary.BigEndian.Uint16(b[2:4]),
		length:        binary.BigEndian.Uint16(b[4:6]),
		unitID:        b[6],
	}
	if h.protocolID != protocolID {
		return h, fmt.Errorf("modbus: unexpected protocol ID 0x%04X, want 0x0000 (not Modbus, or a mistyped port)", h.protocolID)
	}
	// length counts unitID (1 byte) + PDU. A PDU is at least 1 byte
	// (the function code), so length must be >= 2, and by the same PDU
	// budget as maxPDU, length-1 (the PDU) must not exceed maxPDU.
	if h.length < 2 {
		return h, fmt.Errorf("modbus: MBAP length %d too small to hold a PDU", h.length)
	}
	if int(h.length)-1 > maxPDU {
		return h, fmt.Errorf("modbus: MBAP length %d exceeds max PDU budget (%d)", h.length, maxPDU)
	}
	return h, nil
}
