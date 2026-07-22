package modbus

import (
	"bytes"
	"testing"
)

func TestMBAPRoundTrip(t *testing.T) {
	cases := []mbapHeader{
		{transactionID: 0, protocolID: 0, length: 6, unitID: 1},
		{transactionID: 65535, protocolID: 0, length: 2, unitID: 255},
		{transactionID: 1, protocolID: 0, length: 254, unitID: 0},
	}
	for _, h := range cases {
		enc := encodeMBAP(h)
		if len(enc) != mbapLen {
			t.Fatalf("encodeMBAP: got %d bytes, want %d", len(enc), mbapLen)
		}
		got, err := decodeMBAP(enc)
		if err != nil {
			t.Fatalf("decodeMBAP(%v): unexpected error: %v", h, err)
		}
		if got != h {
			t.Fatalf("round trip mismatch: sent %+v, got %+v", h, got)
		}
	}
}

func TestDecodeMBAPMalformed(t *testing.T) {
	valid := encodeMBAP(mbapHeader{transactionID: 1, protocolID: 0, length: 6, unitID: 1})

	cases := []struct {
		name string
		buf  []byte
	}{
		{"empty", nil},
		{"too short", valid[:6]},
		{"single byte", []byte{0x01}},
		{"wrong protocol id", func() []byte {
			b := bytes.Clone(valid)
			b[3] = 0x01 // protocol ID becomes 0x0001
			return b
		}()},
		{"length zero", func() []byte {
			b := bytes.Clone(valid)
			b[4], b[5] = 0x00, 0x00
			return b
		}()},
		{"length one (no PDU room)", func() []byte {
			b := bytes.Clone(valid)
			b[4], b[5] = 0x00, 0x01
			return b
		}()},
		{"length exceeds PDU budget", func() []byte {
			b := bytes.Clone(valid)
			// length-1 must be <= maxPDU(253); 255 => PDU claim of 254
			b[4], b[5] = 0x00, 0xFF
			return b
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeMBAP(tc.buf); err == nil {
				t.Fatalf("decodeMBAP(%s): expected error, got nil", tc.name)
			}
		})
	}
}

func TestDecodeMBAPLengthBoundary(t *testing.T) {
	// length-1 == maxPDU (253) must be accepted; maxPDU+1 must not.
	ok := encodeMBAP(mbapHeader{transactionID: 1, length: uint16(maxPDU + 1), unitID: 1})
	if _, err := decodeMBAP(ok); err != nil {
		t.Fatalf("length at exact PDU budget rejected: %v", err)
	}
	bad := encodeMBAP(mbapHeader{transactionID: 1, length: uint16(maxPDU + 2), unitID: 1})
	if _, err := decodeMBAP(bad); err == nil {
		t.Fatalf("length one past PDU budget was accepted")
	}
}
