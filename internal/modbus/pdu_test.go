package modbus

import (
	"reflect"
	"testing"
)

func TestEncodeReadRequest(t *testing.T) {
	got := encodeReadRequest(FuncReadHoldingRegisters, 0x0064, 0x000A)
	want := []byte{0x03, 0x00, 0x64, 0x00, 0x0A}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got % X, want % X", got, want)
	}
}

func TestEncodeWriteSingleCoil(t *testing.T) {
	on := encodeWriteSingleCoil(0x00AC, true)
	if !reflect.DeepEqual(on, []byte{0x05, 0x00, 0xAC, 0xFF, 0x00}) {
		t.Fatalf("ON encoding wrong: % X", on)
	}
	off := encodeWriteSingleCoil(0x00AC, false)
	if !reflect.DeepEqual(off, []byte{0x05, 0x00, 0xAC, 0x00, 0x00}) {
		t.Fatalf("OFF encoding wrong: % X", off)
	}
}

func TestEncodeWriteMultipleRegistersLimits(t *testing.T) {
	if _, err := encodeWriteMultipleRegisters(0, nil); err == nil {
		t.Fatal("zero registers should be rejected")
	}
	ok := make([]uint16, MaxWriteRegisters)
	if _, err := encodeWriteMultipleRegisters(0, ok); err != nil {
		t.Fatalf("at-limit write rejected: %v", err)
	}
	tooMany := make([]uint16, MaxWriteRegisters+1)
	if _, err := encodeWriteMultipleRegisters(0, tooMany); err == nil {
		t.Fatal("over-limit write should be rejected")
	}
}

func TestEncodeWriteMultipleCoilsLimits(t *testing.T) {
	if _, err := encodeWriteMultipleCoils(0, nil); err == nil {
		t.Fatal("zero coils should be rejected")
	}
	ok := make([]bool, MaxWriteBits)
	if _, err := encodeWriteMultipleCoils(0, ok); err != nil {
		t.Fatalf("at-limit write rejected: %v", err)
	}
	tooMany := make([]bool, MaxWriteBits+1)
	if _, err := encodeWriteMultipleCoils(0, tooMany); err == nil {
		t.Fatal("over-limit write should be rejected")
	}
}

func TestEncodeWriteMultipleCoilsPacking(t *testing.T) {
	// 10 coils: 1010101011 (LSB-first within each byte per §6.11 example)
	values := []bool{true, false, true, false, true, false, true, false, true, true}
	pdu, err := encodeWriteMultipleCoils(0x0013, values)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x0F, 0x00, 0x13, 0x00, 0x0A, 0x02, 0x55, 0x03}
	if !reflect.DeepEqual(pdu, want) {
		t.Fatalf("got % X, want % X", pdu, want)
	}
}

func TestAsException(t *testing.T) {
	code, ok := asException([]byte{0x83, 0x02})
	if !ok || code != ExIllegalDataAddress {
		t.Fatalf("got (%v,%v), want (ExIllegalDataAddress,true)", code, ok)
	}
	if _, ok := asException([]byte{0x03, 0x02, 0x00}); ok {
		t.Fatal("normal response misidentified as exception")
	}
	if _, ok := asException([]byte{0x83}); ok {
		t.Fatal("truncated exception frame should not be treated as valid")
	}
}

func TestDecodeReadBitsResponse(t *testing.T) {
	// FC01 example from §6.1: 10 coils, byte count 2, bits 0xCD,0x01
	pdu := []byte{0x01, 0x02, 0xCD, 0x01}
	got, err := decodeReadBitsResponse(pdu, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, true, true, false, false, true, true, true, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDecodeReadBitsResponseByteCountMismatch(t *testing.T) {
	pdu := []byte{0x01, 0x03, 0xCD, 0x01, 0x00} // byteCount=3 but quantity=10 implies 2
	if _, err := decodeReadBitsResponse(pdu, 10); err == nil {
		t.Fatal("byte count mismatch should error")
	}
}

func TestDecodeReadBitsResponseTruncated(t *testing.T) {
	pdu := []byte{0x01, 0x02, 0xCD} // claims 2 bytes, has 1
	if _, err := decodeReadBitsResponse(pdu, 10); err == nil {
		t.Fatal("truncated response should error")
	}
}

func TestDecodeReadRegistersResponse(t *testing.T) {
	// FC03 example from §6.3: 2 registers, values 0x022B, 0x0000
	pdu := []byte{0x03, 0x04, 0x02, 0x2B, 0x00, 0x00}
	got, err := decodeReadRegistersResponse(pdu, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint16{0x022B, 0x0000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDecodeReadRegistersResponseMismatch(t *testing.T) {
	pdu := []byte{0x03, 0x02, 0x00, 0x01} // says 1 register, caller wants 2
	if _, err := decodeReadRegistersResponse(pdu, 2); err == nil {
		t.Fatal("quantity mismatch should error")
	}
}

func TestDecodeWriteEcho(t *testing.T) {
	pdu := []byte{0x06, 0x00, 0x01, 0x00, 0x03}
	if err := decodeWriteEcho(pdu, 1, 3); err != nil {
		t.Fatalf("valid echo rejected: %v", err)
	}
	if err := decodeWriteEcho(pdu, 2, 3); err == nil {
		t.Fatal("address mismatch not detected")
	}
	if err := decodeWriteEcho(pdu, 1, 4); err == nil {
		t.Fatal("value mismatch not detected")
	}
	if err := decodeWriteEcho([]byte{0x06, 0x00}, 1, 3); err == nil {
		t.Fatal("short echo not detected")
	}
}
