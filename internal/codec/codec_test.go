package codec

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QuitOperation/otcat/internal/protocol"
)

func sampleValue() protocol.Value {
	return protocol.Value{
		Address:   "holding:0",
		Type:      "uint16",
		Value:     uint16(42),
		Quality:   protocol.QualityGood,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestJSONEncoderIsLineDelimited(t *testing.T) {
	var buf bytes.Buffer
	enc := NewJSONEncoder(&buf)
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (ndjson), got %d: %q", len(lines), buf.String())
	}
	for _, line := range lines {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatalf("line is not valid standalone JSON: %v (%q)", err, line)
		}
	}
}

func TestJSONEncoderFields(t *testing.T) {
	var buf bytes.Buffer
	enc := NewJSONEncoder(&buf)
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"address", "type", "value", "quality", "ts"} {
		if _, ok := v[key]; !ok {
			t.Fatalf("missing key %q in %v", key, v)
		}
	}
	if v["quality"] != "good" {
		t.Fatalf("quality = %v, want \"good\"", v["quality"])
	}
}

func TestJSONEncoderRawOmittedWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	enc := NewJSONEncoder(&buf)
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"raw"`) {
		t.Fatalf("raw field should be omitted when empty (omitempty): %q", buf.String())
	}
}

func TestCSVEncoderHeaderOnce(t *testing.T) {
	var buf bytes.Buffer
	enc := NewCSVEncoder(&buf)
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\r\n"), "\n")
	if len(lines) != 3 { // 1 header + 2 rows
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "timestamp,address,type,value,quality") {
		t.Fatalf("unexpected header: %q", lines[0])
	}
}

func TestCSVEncoderArrayFlattening(t *testing.T) {
	var buf bytes.Buffer
	enc := NewCSVEncoder(&buf)
	v := sampleValue()
	v.Value = []interface{}{uint16(1), uint16(2), uint16(3)}
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "1;2;3") {
		t.Fatalf("expected semicolon-joined array in CSV row, got %q", buf.String())
	}
}

func TestCSVEncoderBoolArrayFlattening(t *testing.T) {
	var buf bytes.Buffer
	enc := NewCSVEncoder(&buf)
	v := sampleValue()
	v.Value = []bool{true, false, true}
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "true;false;true") {
		t.Fatalf("expected semicolon-joined bool array, got %q", buf.String())
	}
}

func TestRawEncoderBareScalar(t *testing.T) {
	var buf bytes.Buffer
	enc := NewRawEncoder(&buf)
	if err := enc.Encode(sampleValue()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "42\n" {
		t.Fatalf("got %q, want \"42\\n\"", got)
	}
}

// TestEncodersFlushImmediately proves the streaming contract: after
// Encode returns, the bytes are already in the underlying writer, not
// held in an internal buffer waiting for a batch. `otcat --watch | grep`
// depends on this being true.
func TestEncodersFlushImmediately(t *testing.T) {
	var buf bytes.Buffer
	tests := []Encoder{
		NewJSONEncoder(&buf),
		NewCSVEncoder(&buf),
		NewRawEncoder(&buf),
	}
	for _, enc := range tests {
		buf.Reset()
		if err := enc.Encode(sampleValue()); err != nil {
			t.Fatal(err)
		}
		if buf.Len() == 0 {
			t.Fatalf("%T: nothing written after Encode returned", enc)
		}
	}
}
