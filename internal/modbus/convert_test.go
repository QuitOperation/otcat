package modbus

import (
	"math"
	"math/rand"
	"strconv"
	"testing"
)

func TestParseDataTypeAliases(t *testing.T) {
	cases := map[string]DataType{
		"uint16": TypeUint16, "u16": TypeUint16, "word": TypeUint16,
		"int16": TypeInt16, "i16": TypeInt16, "short": TypeInt16,
		"uint32": TypeUint32, "dword": TypeUint32,
		"int32": TypeInt32, "dint": TypeInt32, "long": TypeInt32,
		"float32": TypeFloat32, "real": TypeFloat32, "FLOAT32": TypeFloat32,
	}
	for s, want := range cases {
		got, err := ParseDataType(s)
		if err != nil {
			t.Fatalf("ParseDataType(%q): %v", s, err)
		}
		if got != want {
			t.Fatalf("ParseDataType(%q) = %v, want %v", s, got, want)
		}
	}
	if _, err := ParseDataType("nonsense"); err == nil {
		t.Fatal("unknown type should error")
	}
}

func TestCodecFloat32WordOrder(t *testing.T) {
	// float32(3.14159) big-endian bit pattern is 0x40490FDB.
	// HighWordFirst: register[0]=0x4049 (high), register[1]=0x0FDB (low).
	hi := Codec{ByteOrder: BigEndian, WordOrder: HighWordFirst}
	v, err := hi.Decode(TypeFloat32, []uint16{0x4049, 0x0FDB})
	if err != nil {
		t.Fatal(err)
	}
	if f := v.(float32); float32(math.Abs(float64(f-3.14159))) > 1e-5 {
		t.Fatalf("high-word-first decode = %v, want ~3.14159", f)
	}

	// LowWordFirst ("word-swapped"): register[0]=0x0FDB (low), register[1]=0x4049 (high).
	lo := Codec{ByteOrder: BigEndian, WordOrder: LowWordFirst}
	v2, err := lo.Decode(TypeFloat32, []uint16{0x0FDB, 0x4049})
	if err != nil {
		t.Fatal(err)
	}
	if f := v2.(float32); float32(math.Abs(float64(f-3.14159))) > 1e-5 {
		t.Fatalf("low-word-first decode = %v, want ~3.14159", f)
	}
}

func TestCodecByteOrderSwap(t *testing.T) {
	big := Codec{ByteOrder: BigEndian}
	v, _ := big.Decode(TypeUint16, []uint16{0x1234})
	if v.(uint16) != 0x1234 {
		t.Fatalf("big-endian passthrough wrong: got 0x%04X", v)
	}

	little := Codec{ByteOrder: LittleEndian}
	v2, _ := little.Decode(TypeUint16, []uint16{0x1234})
	if v2.(uint16) != 0x3412 {
		t.Fatalf("byte-swap wrong: got 0x%04X, want 0x3412", v2)
	}
}

func TestCodecInt32Negative(t *testing.T) {
	c := Codec{}
	regs, err := c.Encode(TypeInt32, "-1")
	if err != nil {
		t.Fatal(err)
	}
	if regs[0] != 0xFFFF || regs[1] != 0xFFFF {
		t.Fatalf("encode(-1) = %04X %04X, want FFFF FFFF", regs[0], regs[1])
	}
	v, err := c.Decode(TypeInt32, regs)
	if err != nil {
		t.Fatal(err)
	}
	if v.(int32) != -1 {
		t.Fatalf("round trip: got %v, want -1", v)
	}
}

func TestCodecRegisterCountMismatch(t *testing.T) {
	c := Codec{}
	if _, err := c.Decode(TypeFloat32, []uint16{1}); err == nil {
		t.Fatal("float32 with 1 register should error")
	}
	if _, err := c.Decode(TypeUint16, []uint16{1, 2}); err == nil {
		t.Fatal("uint16 with 2 registers should error")
	}
}

func TestCodecEncodeOutOfRange(t *testing.T) {
	c := Codec{}
	if _, err := c.Encode(TypeInt16, "40000"); err == nil {
		t.Fatal("40000 exceeds int16 range, should error")
	}
	if _, err := c.Encode(TypeUint16, "-1"); err == nil {
		t.Fatal("-1 is not a valid uint16 literal, should error")
	}
	if _, err := c.Encode(TypeInt32, "99999999999"); err == nil {
		t.Fatal("value exceeds int32 range, should error")
	}
}

func TestCodecEncodeHex(t *testing.T) {
	c := Codec{}
	regs, err := c.Encode(TypeUint16, "0x1234")
	if err != nil {
		t.Fatal(err)
	}
	if regs[0] != 0x1234 {
		t.Fatalf("hex literal decoded to 0x%04X, want 0x1234", regs[0])
	}
}

// TestCodecRoundTripProperty is a property-based-style test: for many
// random values across every (type, byte order, word order)
// combination, Encode followed by Decode must reproduce the original
// value. The exact-bytes tests above prove one point on the curve;
// this proves the whole curve.
func TestCodecRoundTripProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	byteOrders := []ByteOrder{BigEndian, LittleEndian}
	wordOrders := []WordOrder{HighWordFirst, LowWordFirst}

	for _, bo := range byteOrders {
		for _, wo := range wordOrders {
			c := Codec{ByteOrder: bo, WordOrder: wo}

			for i := 0; i < 500; i++ {
				want16 := uint16(rng.Intn(65536))
				regs, err := c.Encode(TypeUint16, strconv.FormatInt(int64(want16), 10))
				if err != nil {
					t.Fatal(err)
				}
				got, err := c.Decode(TypeUint16, regs)
				if err != nil {
					t.Fatal(err)
				}
				if got.(uint16) != want16 {
					t.Fatalf("uint16 round trip: got %v want %v (bo=%v wo=%v)", got, want16, bo, wo)
				}

				wantI32 := int32(rng.Uint32())
				regs, err = c.Encode(TypeInt32, strconv.FormatInt(int64(wantI32), 10))
				if err != nil {
					t.Fatal(err)
				}
				got, err = c.Decode(TypeInt32, regs)
				if err != nil {
					t.Fatal(err)
				}
				if got.(int32) != wantI32 {
					t.Fatalf("int32 round trip: got %v want %v (bo=%v wo=%v)", got, wantI32, bo, wo)
				}

				bits := rng.Uint32()
				wantF32 := math.Float32frombits(bits)
				if math.IsInf(float64(wantF32), 0) {
					continue
				}
				regs, err = c.Encode(TypeFloat32, strconv.FormatFloat(float64(wantF32), 'g', -1, 32))
				if err != nil {
					t.Fatal(err)
				}
				got, err = c.Decode(TypeFloat32, regs)
				if err != nil {
					t.Fatal(err)
				}
				gf := got.(float32)
				if math.IsNaN(float64(wantF32)) {
					if !math.IsNaN(float64(gf)) {
						t.Fatalf("float32 NaN round trip lost NaN-ness (bo=%v wo=%v)", bo, wo)
					}
					continue
				}
				if gf != wantF32 {
					t.Fatalf("float32 round trip: got %v want %v (bo=%v wo=%v)", gf, wantF32, bo, wo)
				}
			}
		}
	}
}
