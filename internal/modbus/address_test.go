package modbus

import "testing"

func TestParseSpecClassicTranslation(t *testing.T) {
	cases := []struct {
		spec       string
		wantTable  Table
		wantOffset uint16
		wantCount  int
	}{
		{"holding:40001", TableHoldingRegister, 0, 0},
		{"holding:40010", TableHoldingRegister, 9, 0},
		{"holding:49999", TableHoldingRegister, 9998, 0}, // top of classic band
		{"input:30001", TableInputRegister, 0, 0},
		{"discrete:10001", TableDiscreteInput, 0, 0},
		{"coil:1", TableCoil, 0, 0}, // classic coil numbering starts at 1, not 0
		{"coil:16", TableCoil, 15, 0},
		{"holding:40001:10", TableHoldingRegister, 0, 10},
	}
	for _, tc := range cases {
		sp, err := ParseSpec(tc.spec, false)
		if err != nil {
			t.Fatalf("ParseSpec(%q): %v", tc.spec, err)
		}
		if sp.Table != tc.wantTable || sp.Address != tc.wantOffset || sp.Count != tc.wantCount {
			t.Fatalf("ParseSpec(%q) = %+v, want table=%v addr=%v count=%v", tc.spec, sp, tc.wantTable, tc.wantOffset, tc.wantCount)
		}
	}
}

func TestParseSpecBelowClassicBandIsRaw(t *testing.T) {
	// 100 is below holding's classic band (40001-49999), so even with
	// classic translation *enabled*, it must be read as a literal offset.
	sp, err := ParseSpec("holding:100", false)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Address != 100 {
		t.Fatalf("got offset %d, want 100 (raw, band did not apply)", sp.Address)
	}
}

func TestParseSpecAboveClassicBandIsRaw(t *testing.T) {
	// 50000 is one past holding's classic band (ends at 49999), so it
	// is a literal offset, not (incorrectly) 50000-40001.
	sp, err := ParseSpec("holding:50000", false)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Address != 50000 {
		t.Fatalf("got offset %d, want 50000 (raw, past classic band)", sp.Address)
	}
}

func TestParseSpecRawAddressDisablesTranslation(t *testing.T) {
	sp, err := ParseSpec("holding:40001", true)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Address != 40001 {
		t.Fatalf("raw mode: got offset %d, want literal 40001", sp.Address)
	}
}

func TestParseSpecMalformed(t *testing.T) {
	cases := []string{
		"",
		"holding",       // missing address
		"holding:1:2:3", // too many parts
		"nope:1",        // unknown table
		"holding:-1",    // negative address
		"holding:abc",   // non-numeric address
		"holding:1:abc", // non-numeric count
		"holding:1:0",   // count must be positive when given
		"holding:1:-5",  // negative count
	}
	for _, s := range cases {
		if _, err := ParseSpec(s, false); err == nil {
			t.Fatalf("ParseSpec(%q): expected error, got none", s)
		}
	}
}

func TestParseSpecOffsetOverflow(t *testing.T) {
	// classic translation off the top of a 16-bit space: e.g. an
	// absurd raw literal beyond 65535 must be rejected outright.
	if _, err := ParseSpec("holding:99999999", true); err == nil {
		t.Fatal("offset beyond 16-bit range should error")
	}
}

func TestTableWritable(t *testing.T) {
	if !TableCoil.Writable() {
		t.Fatal("coil should be writable")
	}
	if !TableHoldingRegister.Writable() {
		t.Fatal("holding register should be writable")
	}
	if TableDiscreteInput.Writable() {
		t.Fatal("discrete input must not be writable")
	}
	if TableInputRegister.Writable() {
		t.Fatal("input register must not be writable")
	}
}

func TestTableIsBits(t *testing.T) {
	if !TableCoil.IsBits() || !TableDiscreteInput.IsBits() {
		t.Fatal("coil and discrete input are bit tables")
	}
	if TableHoldingRegister.IsBits() || TableInputRegister.IsBits() {
		t.Fatal("holding and input are register tables, not bit tables")
	}
}

func TestParseTableAliases(t *testing.T) {
	for _, s := range []string{"coil", "coils", "co", "c"} {
		if tb, err := ParseTable(s); err != nil || tb != TableCoil {
			t.Fatalf("ParseTable(%q) = %v, %v; want TableCoil", s, tb, err)
		}
	}
	if _, err := ParseTable("garbage"); err == nil {
		t.Fatal("unknown table alias should error")
	}
}
