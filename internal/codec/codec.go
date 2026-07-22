// Package codec turns a protocol.Value into a line on stdout. Every
// encoder here writes and flushes one Value at a time — no buffering
// across values — because otcat's entire value proposition is that
// `otcat --watch ... | grep ...` sees each reading the instant it
// arrives, not once an internal buffer fills.
package codec

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/QuitOperation/otcat/internal/protocol"
)

// Encoder writes one Value as one line of output.
type Encoder interface {
	Encode(v protocol.Value) error
}

// JSONEncoder emits newline-delimited JSON (ndjson): one compact JSON
// object per line, no enclosing array. ndjson, not a JSON array, is the
// only framing compatible with streaming — an array can't be closed
// until the process is done producing values, which defeats --watch
// entirely.
type JSONEncoder struct{ enc *json.Encoder }

func NewJSONEncoder(w io.Writer) *JSONEncoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONEncoder{enc: enc}
}

func (e *JSONEncoder) Encode(v protocol.Value) error { return e.enc.Encode(v) }

// CSVEncoder emits a header on the first row, then one row per Value.
// Slice-valued reads (array reads, multi-bit coil reads) are flattened
// to a semicolon-joined field so the file stays one-row-per-sample —
// CSV has no native representation for a nested value and pretending
// otherwise (e.g. exploding to N columns) would make the column count
// depend on read arity, breaking every downstream tool that assumes a
// fixed schema.
type CSVEncoder struct {
	w      *csv.Writer
	header bool
}

func NewCSVEncoder(w io.Writer) *CSVEncoder { return &CSVEncoder{w: csv.NewWriter(w)} }

func (e *CSVEncoder) Encode(v protocol.Value) error {
	if !e.header {
		if err := e.w.Write([]string{"timestamp", "address", "type", "value", "quality"}); err != nil {
			return err
		}
		e.header = true
	}
	row := []string{
		v.Timestamp.Format(time.RFC3339Nano),
		v.Address,
		v.Type,
		formatValue(v.Value),
		v.Quality.String(),
	}
	if err := e.w.Write(row); err != nil {
		return err
	}
	e.w.Flush()
	return e.w.Error()
}

func formatValue(v interface{}) string {
	if s, ok := v.([]interface{}); ok {
		out := ""
		for i, item := range s {
			if i > 0 {
				out += ";"
			}
			out += fmt.Sprintf("%v", item)
		}
		return out
	}
	if s, ok := v.([]bool); ok {
		out := ""
		for i, item := range s {
			if i > 0 {
				out += ";"
			}
			out += fmt.Sprintf("%v", item)
		}
		return out
	}
	return fmt.Sprintf("%v", v)
}

// RawEncoder emits only the scalar value, one per line, nothing else.
// This is the pipe-into-anything format: `otcat --read holding:40001
// --raw | awk '{print $1 * 0.1}'` sees a bare number.
type RawEncoder struct{ w io.Writer }

func NewRawEncoder(w io.Writer) *RawEncoder { return &RawEncoder{w: w} }

func (e *RawEncoder) Encode(v protocol.Value) error {
	_, err := fmt.Fprintf(e.w, "%v\n", v.Value)
	return err
}
