package modbus

import (
	"context"
	"testing"

	"github.com/QuitOperation/otcat/internal/mock"
)

// --- Pure CPU cost: encode/decode with no I/O ----------------------------

func BenchmarkEncodeReadRequest(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = encodeReadRequest(FuncReadHoldingRegisters, 0, 10)
	}
}

func BenchmarkDecodeReadRegistersResponse(b *testing.B) {
	pdu := []byte{0x03, 0x14, 0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0, 8, 0, 9, 0, 10}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = decodeReadRegistersResponse(pdu, 10)
	}
}

func BenchmarkDecodeReadBitsResponse(b *testing.B) {
	pdu := []byte{0x01, 0x02, 0xCD, 0x01}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = decodeReadBitsResponse(pdu, 10)
	}
}

func BenchmarkCodecDecodeFloat32(b *testing.B) {
	c := Codec{}
	regs := []uint16{0x4049, 0x0FDB}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = c.Decode(TypeFloat32, regs)
	}
}

func BenchmarkCodecEncodeFloat32(b *testing.B) {
	c := Codec{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = c.Encode(TypeFloat32, "3.14159")
	}
}

func BenchmarkParseSpecClassic(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ParseSpec("holding:40001:10", false)
	}
}

// --- Full round trip over a real TCP socket (loopback) -------------------
//
// These are the numbers that matter for the paper's benchmark section:
// not "how fast is the parser" (answered above, and it is not the
// bottleneck) but "how many read transactions can otcat actually drive
// against something on the other end of a socket." Against 127.0.0.1
// this measures otcat's own overhead with the network approximately
// zeroed out; docs/benchmarks.md discusses what changes on a real LAN.

func BenchmarkClientReadHoldingRegisters1(b *testing.B) {
	benchmarkRead(b, 1)
}

func BenchmarkClientReadHoldingRegisters10(b *testing.B) {
	benchmarkRead(b, 10)
}

func BenchmarkClientReadHoldingRegisters125(b *testing.B) {
	benchmarkRead(b, 125) // MaxReadRegisters: the worst case for the response parser
}

func benchmarkRead(b *testing.B, quantity int) {
	s, err := mock.New()
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	c := NewClient(s.Addr())
	if err := c.Connect(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.ReadHoldingRegisters(ctx, 0, quantity); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClientWriteSingleRegister(b *testing.B) {
	s, err := mock.New()
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	c := NewClient(s.Addr())
	if err := c.Connect(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.WriteSingleRegister(ctx, 0, uint16(i)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDriverReadEndToEnd measures the full path a CLI invocation
// exercises: spec parse, wire round trip, and typed decode together.
func BenchmarkDriverReadEndToEnd(b *testing.B) {
	s, err := mock.New()
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	s.SetHoldingRange(0, []uint16{0x4049, 0x0FDB})

	opt := DefaultOptions()
	opt.RawAddress = true
	opt.Type = TypeFloat32
	d := NewDriver(s.Addr(), opt)
	if err := d.Connect(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer d.Close()

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := d.Read(ctx, "holding:0:2"); err != nil {
			b.Fatal(err)
		}
	}
}
