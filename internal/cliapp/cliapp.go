// Package cliapp is otcat's command-line orchestration layer: parse
// flags, pick a driver, dispatch to read/watch/write, map errors to
// scriptable exit codes. It touches no protocol bytes itself — that is
// entirely internal/modbus (and, in the future, internal/eip etc.).
package cliapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/QuitOperation/otcat/internal/codec"
	"github.com/QuitOperation/otcat/internal/modbus"
	"github.com/QuitOperation/otcat/internal/protocol"
)

// Exit codes are part of otcat's contract with the scripts and cron
// jobs that will wrap it — stable and specific enough that `otcat ...;
// case $? in ...)` is a reasonable thing for an operator to write.
const (
	ExitOK           = 0
	ExitUsage        = 1
	ExitConnection   = 2
	ExitProtocol     = 3
	ExitWriteAborted = 4
	ExitIO           = 5
	ExitInterrupted  = 130 // --watch stopped cleanly by SIGINT/SIGTERM; matches the 128+SIGINT convention operators already expect
)

// Version is set at build time via -ldflags "-X .../cliapp.Version=...".
var Version = "dev"

// Run is the entire program, parameterized over args/streams so tests
// never touch the real process's stdin/stdout/os.Args.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c, err := parseArgs(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage
	}

	if c.showHelp {
		fmt.Fprint(stdout, usageHeader)
		return ExitOK
	}
	if c.showVersion {
		fmt.Fprintf(stdout, "otcat %s\n", Version)
		return ExitOK
	}

	if err := validate(c); err != nil {
		fmt.Fprintf(stderr, "otcat: %v\n\n", err)
		fmt.Fprint(stderr, usageHeader)
		return ExitUsage
	}

	driver, addr, err := buildDriver(c)
	if err != nil {
		fmt.Fprintf(stderr, "otcat: %v\n", err)
		return ExitUsage
	}

	enc, err := buildEncoder(c.format, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "otcat: %v\n", err)
		return ExitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --dry-run never opens a socket, by construction: the check
	// happens before Connect is ever called.
	if c.writeSpec != "" && c.dryRun {
		return runDryRun(driver, c, stdin, stdout, stderr)
	}

	dialCtx, cancel := context.WithTimeout(ctx, protocol.DialTimeout)
	defer cancel()
	if err := driver.Connect(dialCtx); err != nil {
		fmt.Fprintf(stderr, "otcat: connecting to %s: %v\n", addr, err)
		if errors.Is(err, protocol.ErrNotImplemented) {
			return ExitUsage
		}
		return ExitConnection
	}
	defer driver.Close()

	switch {
	case c.readSpec != "":
		return runRead(ctx, driver, enc, c, stderr)
	case c.watchSpec != "":
		return runWatch(ctx, driver, enc, c, stderr)
	case c.writeSpec != "":
		return runWrite(ctx, driver, enc, c, stdin, stderr)
	default:
		// unreachable: validate() already enforced exactly one action
		return ExitUsage
	}
}

func validate(c *config) error {
	endpoints := countNonEmpty(c.modbusAddr, c.eipAddr, c.s7Addr, c.bacnetAddr)
	if endpoints == 0 {
		return fmt.Errorf("no endpoint given: pass one of --modbus --eip --s7comm --bacnet")
	}
	if endpoints > 1 {
		return fmt.Errorf("pass exactly one endpoint flag, not several")
	}

	actions := countNonEmpty(c.readSpec, c.watchSpec, c.writeSpec)
	if actions == 0 {
		return fmt.Errorf("no action given: pass one of --read --watch --write")
	}
	if actions > 1 {
		return fmt.Errorf("pass exactly one action flag, not several")
	}

	if c.writeSpec != "" {
		if c.fromStdin && c.value != "" {
			return fmt.Errorf("--value and --from-stdin are mutually exclusive")
		}
		if !c.fromStdin && c.value == "" {
			return fmt.Errorf("--write requires --value or --from-stdin")
		}
	} else {
		if c.value != "" {
			return fmt.Errorf("--value only applies to --write")
		}
		if c.fromStdin {
			return fmt.Errorf("--from-stdin only applies to --write")
		}
		if c.dryRun {
			return fmt.Errorf("--dry-run only applies to --write")
		}
		if c.confirm {
			return fmt.Errorf("--confirm only applies to --write")
		}
	}

	if c.timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
	}
	if c.watchSpec != "" && c.interval <= 0 {
		return fmt.Errorf("--interval must be positive")
	}
	if c.unitID > 255 {
		return fmt.Errorf("--unit must be 0-255")
	}
	return nil
}

func countNonEmpty(ss ...string) int {
	n := 0
	for _, s := range ss {
		if s != "" {
			n++
		}
	}
	return n
}

func buildDriver(c *config) (protocol.Driver, string, error) {
	switch {
	case c.modbusAddr != "":
		addr := c.modbusAddr
		if !strings.Contains(addr, ":") {
			addr += ":502" // 502 is Modbus TCP's IANA-assigned port
		}
		dt, err := modbus.ParseDataType(c.dataType)
		if err != nil {
			return nil, "", err
		}
		bo, err := modbus.ParseByteOrder(c.byteOrder)
		if err != nil {
			return nil, "", err
		}
		wo, err := modbus.ParseWordOrder(c.wordOrder)
		if err != nil {
			return nil, "", err
		}
		opt := modbus.Options{
			UnitID:     byte(c.unitID),
			Timeout:    c.timeout,
			Type:       dt,
			ByteOrder:  bo,
			WordOrder:  wo,
			RawAddress: c.rawAddress,
		}
		return modbus.NewDriver(addr, opt), addr, nil

	case c.eipAddr != "":
		return newStubDriver("eip", c.eipAddr), c.eipAddr, nil
	case c.s7Addr != "":
		return newStubDriver("s7", c.s7Addr), c.s7Addr, nil
	case c.bacnetAddr != "":
		return newStubDriver("bacnet", c.bacnetAddr), c.bacnetAddr, nil
	default:
		return nil, "", fmt.Errorf("no endpoint selected") // unreachable after validate()
	}
}

func buildEncoder(format string, w io.Writer) (codec.Encoder, error) {
	switch format {
	case "json":
		return codec.NewJSONEncoder(w), nil
	case "csv":
		return codec.NewCSVEncoder(w), nil
	case "raw":
		return codec.NewRawEncoder(w), nil
	default:
		return nil, fmt.Errorf("unknown output format %q", format)
	}
}

// classifyErr maps a returned error to an exit code so that scripts can
// branch on *why* otcat failed, not just *that* it failed.
func classifyErr(err error) int {
	var exc *modbus.ExceptionError
	if errors.As(err, &exc) {
		return ExitProtocol
	}
	if errors.Is(err, protocol.ErrNotImplemented) {
		return ExitUsage
	}
	if errors.Is(err, modbus.ErrInvalidInput) {
		return ExitUsage
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) {
		return ExitConnection
	}
	return ExitIO
}
