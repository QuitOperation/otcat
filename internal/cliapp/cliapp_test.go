package cliapp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuitOperation/otcat/internal/mock"
)

func newServer(t *testing.T) *mock.Server {
	t.Helper()
	s, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func run(args []string, stdin string) (code int, stdout, stderr string) {
	var out, errBuf bytes.Buffer
	code = Run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestCLIReadJSON(t *testing.T) {
	s := newServer(t)
	s.SetHolding(0, 42)

	code, out, errOut := run([]string{"--modbus", s.Addr(), "--read", "holding:0", "--raw-address"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, out)
	}
	if v["value"].(float64) != 42 {
		t.Fatalf("value = %v, want 42", v["value"])
	}
}

func TestCLIReadRaw(t *testing.T) {
	s := newServer(t)
	s.SetHolding(0, 123)
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--read", "holding:0", "--raw-address", "--raw"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	if strings.TrimSpace(out) != "123" {
		t.Fatalf("got %q, want \"123\"", out)
	}
}

func TestCLIReadCSV(t *testing.T) {
	s := newServer(t)
	s.SetHolding(0, 7)
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--read", "holding:0", "--raw-address", "--csv"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "timestamp,") {
		t.Fatalf("unexpected CSV output: %q", out)
	}
}

func TestCLINoEndpoint(t *testing.T) {
	code, _, errOut := run([]string{"--read", "holding:0"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage; stderr=%q", code, errOut)
	}
}

func TestCLIMultipleEndpoints(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1", "--eip", "127.0.0.1:1", "--read", "holding:0"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLINoAction(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLIMultipleActions(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1", "--read", "holding:0", "--watch", "holding:1"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLIConnectionRefused(t *testing.T) {
	code, _, errOut := run([]string{"--modbus", "127.0.0.1:1", "--read", "holding:0"}, "")
	if code != ExitConnection {
		t.Fatalf("exit=%d, want ExitConnection; stderr=%q", code, errOut)
	}
}

func TestCLIWriteWithoutConfirmNonInteractive(t *testing.T) {
	s := newServer(t)
	code, _, errOut := run([]string{"--modbus", s.Addr(), "--write", "holding:0", "--raw-address", "--value", "1"}, "")
	if code != ExitWriteAborted {
		t.Fatalf("exit=%d, want ExitWriteAborted; stderr=%q", code, errOut)
	}
	if s.GetHolding(0) != 0 {
		t.Fatalf("write should have been refused, but holding[0] = %d", s.GetHolding(0))
	}
}

func TestCLIWriteWithConfirm(t *testing.T) {
	s := newServer(t)
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--write", "holding:0", "--raw-address", "--value", "77", "--confirm"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	if s.GetHolding(0) != 77 {
		t.Fatalf("holding[0] = %d, want 77", s.GetHolding(0))
	}
	if !strings.Contains(out, `"ack"`) {
		t.Fatalf("expected an ack record in output, got %q", out)
	}
}

func TestCLIWriteFromStdin(t *testing.T) {
	s := newServer(t)
	stdin := "10\n20\n# a comment, skip me\n\n30\n"
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--write", "holding:0", "--raw-address", "--from-stdin", "--confirm"}, stdin)
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	if s.GetHolding(0) != 30 { // last write wins, sequentially
		t.Fatalf("holding[0] = %d, want 30 (last of the stream)", s.GetHolding(0))
	}
	acks := strings.Count(out, `"ack"`)
	if acks != 3 {
		t.Fatalf("got %d acks, want 3 (comments/blank lines skipped)", acks)
	}
}

func TestCLIWriteFromStdinRefusesConfirmPrompt(t *testing.T) {
	// --from-stdin consumes stdin for write values, so a confirmation
	// prompt reading from the same stream is impossible; otcat must
	// refuse rather than attempt to prompt.
	s := newServer(t)
	code, _, errOut := run([]string{"--modbus", s.Addr(), "--write", "holding:0", "--raw-address", "--from-stdin"}, "10\n")
	if code != ExitWriteAborted {
		t.Fatalf("exit=%d, want ExitWriteAborted; stderr=%q", code, errOut)
	}
}

func TestCLIDryRunTouchesNoNetwork(t *testing.T) {
	// Point at an address nothing listens on; dry-run must still
	// succeed, proving it never dials.
	code, out, errOut := run([]string{"--modbus", "127.0.0.1:1", "--write", "holding:40001", "--value", "999", "--dry-run"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &plan); err != nil {
		t.Fatalf("dry-run output not valid JSON: %v (%q)", err, out)
	}
	if plan["driver"] != "modbus" || plan["address"] != "holding:0" {
		t.Fatalf("unexpected plan: %v", plan)
	}
}

func TestCLIDryRunFromStdinReportsEachLine(t *testing.T) {
	stdin := "1\n2\nnot-a-number\n"
	code, out, _ := run([]string{"--modbus", "127.0.0.1:1", "--write", "holding:0", "--raw-address", "--from-stdin", "--dry-run"}, stdin)
	if code != ExitUsage { // one bad line among the three
		t.Fatalf("exit=%d, want ExitUsage due to one invalid literal", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 { // two valid plans printed; the bad line errored to stderr instead
		t.Fatalf("got %d plan lines, want 2: %q", len(lines), out)
	}
}

func TestCLIWatchCount(t *testing.T) {
	s := newServer(t)
	s.SetHolding(0, 5)
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--watch", "holding:0", "--raw-address", "--count", "3", "--interval", "1ms"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), out)
	}
}

func TestCLIHelp(t *testing.T) {
	code, out, _ := run([]string{"--help"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d, want ExitOK", code)
	}
	if !strings.Contains(out, "otcat") {
		t.Fatalf("help output missing program name: %q", out)
	}
}

func TestCLIVersion(t *testing.T) {
	code, out, _ := run([]string{"--version"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d, want ExitOK", code)
	}
	if !strings.Contains(out, "otcat") {
		t.Fatalf("version output missing program name: %q", out)
	}
}

func TestCLIUnknownFlag(t *testing.T) {
	code, _, _ := run([]string{"--this-flag-does-not-exist"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLIStubDriverReadIsUsageError(t *testing.T) {
	// Stub drivers fail with protocol.ErrNotImplemented; that is a
	// capability gap, not a network problem, and must be reported as
	// such (ExitUsage), not conflated with ExitConnection.
	code, _, errOut := run([]string{"--eip", "127.0.0.1:1", "--read", "SomeTag"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage; stderr=%q", code, errOut)
	}
	if !strings.Contains(errOut, "not implemented") {
		t.Fatalf("expected a not-implemented message, got %q", errOut)
	}
}

func TestCLIDryRunUnsupportedDriverIsUsageError(t *testing.T) {
	code, _, errOut := run([]string{"--s7comm", "127.0.0.1:1", "--write", "DB1.DBX0.0", "--value", "1", "--dry-run"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage; stderr=%q", code, errOut)
	}
	if !strings.Contains(errOut, "dry-run is not available") {
		t.Fatalf("expected a dry-run-unavailable message, got %q", errOut)
	}
}

func TestCLIBadModbusAddress(t *testing.T) {
	s := newServer(t)
	code, _, errOut := run([]string{"--modbus", s.Addr(), "--read", "bogus:0"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage (a malformed address spec is operator error, not I/O failure); stderr=%q", code, errOut)
	}
}

func TestCLIValueWithoutWriteRejected(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1", "--read", "holding:0", "--value", "5"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLIWriteRequiresValueOrStdin(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1", "--write", "holding:0"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLIValueAndFromStdinMutuallyExclusive(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1", "--write", "holding:0", "--value", "1", "--from-stdin"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLICsvAndRawMutuallyExclusive(t *testing.T) {
	code, _, _ := run([]string{"--modbus", "127.0.0.1:1", "--read", "holding:0", "--csv", "--raw"}, "")
	if code != ExitUsage {
		t.Fatalf("exit=%d, want ExitUsage", code)
	}
}

func TestCLIFloat32EndToEnd(t *testing.T) {
	s := newServer(t)
	s.SetHoldingRange(0, []uint16{0x4049, 0x0FDB})
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--read", "holding:0:2", "--raw-address", "--type", "float32", "--raw"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	got := strings.TrimSpace(out)
	if !strings.HasPrefix(got, "3.14159") {
		t.Fatalf("got %q, want a value starting 3.14159", got)
	}
}

func TestCLIWordOrderLowFlips(t *testing.T) {
	s := newServer(t)
	s.SetHoldingRange(0, []uint16{0x0FDB, 0x4049}) // swapped vs. high-word-first
	code, out, errOut := run([]string{"--modbus", s.Addr(), "--read", "holding:0:2", "--raw-address", "--type", "float32", "--word-order", "low", "--raw"}, "")
	if code != ExitOK {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	got := strings.TrimSpace(out)
	if !strings.HasPrefix(got, "3.14159") {
		t.Fatalf("got %q, want ~3.14159 with word-order=low compensating for the swap", got)
	}
}
