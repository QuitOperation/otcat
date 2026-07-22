package codec

import (
	"io"
	"testing"
	"time"

	"github.com/QuitOperation/otcat/internal/protocol"
)

func benchValue() protocol.Value {
	return protocol.Value{
		Address:   "holding:40001",
		Type:      "float32",
		Value:     float32(3.14159),
		Quality:   protocol.QualityGood,
		Timestamp: time.Now().UTC(),
		Raw:       []uint16{0x4049, 0x0FDB},
	}
}

func BenchmarkJSONEncode(b *testing.B) {
	v := benchValue()
	enc := NewJSONEncoder(io.Discard)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = enc.Encode(v)
	}
}

func BenchmarkCSVEncode(b *testing.B) {
	v := benchValue()
	enc := NewCSVEncoder(io.Discard)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = enc.Encode(v)
	}
}

func BenchmarkRawEncode(b *testing.B) {
	v := benchValue()
	enc := NewRawEncoder(io.Discard)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = enc.Encode(v)
	}
}
