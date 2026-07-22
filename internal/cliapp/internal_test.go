package cliapp

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/QuitOperation/otcat/internal/modbus"
	"github.com/QuitOperation/otcat/internal/protocol"
)

func TestIsInteractiveNonFileReader(t *testing.T) {
	if isInteractive(bytes.NewReader(nil)) {
		t.Fatal("a bytes.Reader is never an interactive terminal")
	}
	if isInteractive(strings.NewReader("")) {
		t.Fatal("a strings.Reader is never an interactive terminal")
	}
}

func TestIsInteractiveRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "otcat-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// A regular file satisfies the *os.File type assertion but is not
	// a character device, so it must read as non-interactive too —
	// this is exactly the `cmd < file.txt` shell redirection case.
	if isInteractive(f) {
		t.Fatal("a regular file should not be treated as an interactive terminal")
	}
}

func TestPromptYesNoVariants(t *testing.T) {
	cases := map[string]bool{
		"y\n":     true,
		"Y\n":     true,
		"yes\n":   true,
		"YES\n":   true,
		"  y  \n": true,
		"n\n":     false,
		"no\n":    false,
		"\n":      false,
		"maybe\n": false,
	}
	for input, want := range cases {
		var errBuf bytes.Buffer
		got, err := promptYesNo(strings.NewReader(input), &errBuf, "prompt: ")
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if got != want {
			t.Fatalf("input %q: got %v, want %v", input, got, want)
		}
		if !strings.Contains(errBuf.String(), "prompt: ") {
			t.Fatalf("prompt should be written to stderr, got %q", errBuf.String())
		}
	}
}

func TestPromptYesNoEOF(t *testing.T) {
	var errBuf bytes.Buffer
	got, err := promptYesNo(strings.NewReader(""), &errBuf, "prompt: ")
	if err != nil {
		t.Fatalf("EOF should not itself be an error: %v", err)
	}
	if got {
		t.Fatal("EOF with no input should not be treated as confirmation")
	}
}

type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "fake timeout" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return true }

func TestClassifyErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"exception", &modbus.ExceptionError{Function: 0x03, Exception: modbus.ExIllegalDataAddress}, ExitProtocol},
		{"not implemented", protocol.ErrNotImplemented, ExitUsage},
		{"text merely mentions not-implemented, not %w-wrapped", errors.New("connecting: " + protocol.ErrNotImplemented.Error()), ExitIO},
		{"invalid input", modbus.ErrInvalidInput, ExitUsage},
		{"timeout", fakeTimeoutErr{}, ExitConnection},
		{"generic", errors.New("disk on fire"), ExitIO},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyErr(tc.err); got != tc.want {
				t.Fatalf("classifyErr(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestBuildDriverEndpointSelection(t *testing.T) {
	cases := []struct {
		cfg      config
		wantName string
		wantAddr string
	}{
		{config{modbusAddr: "10.0.0.1:502", dataType: "uint16", byteOrder: "big", wordOrder: "high"}, "modbus", "10.0.0.1:502"},
		{config{modbusAddr: "10.0.0.1", dataType: "uint16", byteOrder: "big", wordOrder: "high"}, "modbus", "10.0.0.1:502"}, // default port
		{config{eipAddr: "10.0.0.2"}, "eip", "10.0.0.2"},
		{config{s7Addr: "10.0.0.3"}, "s7", "10.0.0.3"},
		{config{bacnetAddr: "10.0.0.4"}, "bacnet", "10.0.0.4"},
	}
	for _, tc := range cases {
		drv, addr, err := buildDriver(&tc.cfg)
		if err != nil {
			t.Fatalf("buildDriver(%+v): %v", tc.cfg, err)
		}
		if drv.Name() != tc.wantName {
			t.Fatalf("driver name = %q, want %q", drv.Name(), tc.wantName)
		}
		if addr != tc.wantAddr {
			t.Fatalf("addr = %q, want %q", addr, tc.wantAddr)
		}
	}
}

func TestBuildDriverBadModbusOptions(t *testing.T) {
	cases := []config{
		{modbusAddr: "10.0.0.1:502", dataType: "not-a-type", byteOrder: "big", wordOrder: "high"},
		{modbusAddr: "10.0.0.1:502", dataType: "uint16", byteOrder: "sideways", wordOrder: "high"},
		{modbusAddr: "10.0.0.1:502", dataType: "uint16", byteOrder: "big", wordOrder: "diagonal"},
	}
	for _, c := range cases {
		if _, _, err := buildDriver(&c); err == nil {
			t.Fatalf("config %+v: expected an error", c)
		}
	}
}

func TestNewStubDriverPanicsOnUnknownName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unrecognized stub driver name")
		}
	}()
	newStubDriver("does-not-exist", "addr")
}

func TestBuildEncoderUnknownFormat(t *testing.T) {
	if _, err := buildEncoder("xml", &bytes.Buffer{}); err == nil {
		t.Fatal("unknown format should error")
	}
}
