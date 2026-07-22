package modbus

import "testing"

// FuzzDecodeMBAP proves decodeMBAP either succeeds or returns an error
// for any input — never panics, never hangs, never reads past the
// slice it was given. This matters more here than almost anywhere else
// in the codebase: this is the first function to touch bytes that came
// off a real socket, from a device otcat does not control and cannot
// trust to be well-behaved (or benign).
func FuzzDecodeMBAP(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x01})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add(make([]byte, 300))
	f.Fuzz(func(t *testing.T, b []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("decodeMBAP panicked on input % X: %v", b, r)
			}
		}()
		_, _ = decodeMBAP(b)
	})
}

// FuzzDecodeReadRegistersResponse targets the response parser a
// malicious or simply broken Modbus server (or a MITM on the OT
// segment) controls directly: byteCount is attacker-controlled and
// used to index into the buffer.
func FuzzDecodeReadRegistersResponse(f *testing.F) {
	f.Add([]byte{0x03, 0x04, 0x00, 0x01, 0x00, 0x02}, 2)
	f.Add([]byte{0x03, 0xFF}, 1)
	f.Add([]byte{}, 1)
	f.Add([]byte{0x03, 0x02, 0x00}, 1)
	f.Fuzz(func(t *testing.T, pdu []byte, quantity int) {
		if quantity < 0 || quantity > 10000 {
			return // out of any meaningful/realistic range; caller-side limits already reject this before reaching here
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("decodeReadRegistersResponse panicked on pdu=% X quantity=%d: %v", pdu, quantity, r)
			}
		}()
		_, _ = decodeReadRegistersResponse(pdu, quantity)
	})
}

// FuzzDecodeReadBitsResponse mirrors FuzzDecodeReadRegistersResponse
// for the bit-packing path, which does its own byte/bit index math.
func FuzzDecodeReadBitsResponse(f *testing.F) {
	f.Add([]byte{0x01, 0x02, 0xCD, 0x01}, 10)
	f.Add([]byte{0x01}, 1)
	f.Add([]byte{}, 0)
	f.Fuzz(func(t *testing.T, pdu []byte, quantity int) {
		if quantity < 0 || quantity > 100000 {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("decodeReadBitsResponse panicked on pdu=% X quantity=%d: %v", pdu, quantity, r)
			}
		}()
		_, _ = decodeReadBitsResponse(pdu, quantity)
	})
}

// FuzzParseSpec proves the address grammar parser is safe against
// arbitrary operator (or config-file, or shell-injected) input — this
// is the one parser in otcat that sees a human-authored string instead
// of wire bytes, and humans typo creatively.
func FuzzParseSpec(f *testing.F) {
	seeds := []string{
		"holding:40001",
		"coil:1:10",
		"", "::", "holding:", ":1", "a:b:c:d",
		"holding:99999999999999999999999999",
		"holding:-1:-1",
		"HOLDING:40001",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseSpec panicked on input %q: %v", s, r)
			}
		}()
		_, _ = ParseSpec(s, false)
		_, _ = ParseSpec(s, true)
	})
}

// FuzzCodecEncode proves literal parsing for writes — the path that
// takes operator- or file-supplied text and turns it into bytes headed
// for a live controller — never panics regardless of what text it is
// handed.
func FuzzCodecEncode(f *testing.F) {
	seeds := []string{"0", "-1", "65535", "0x1234", "3.14", "1e300", "NaN", "Inf", "", "abc", "999999999999999999999999"}
	for _, s := range seeds {
		f.Add(s)
	}
	types := []DataType{TypeUint16, TypeInt16, TypeUint32, TypeInt32, TypeFloat32}
	f.Fuzz(func(t *testing.T, s string) {
		c := Codec{}
		for _, dt := range types {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Codec.Encode(%v, %q) panicked: %v", dt, s, r)
					}
				}()
				_, _ = c.Encode(dt, s)
			}()
		}
	})
}
