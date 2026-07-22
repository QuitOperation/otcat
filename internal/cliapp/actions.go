package cliapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/QuitOperation/otcat/internal/codec"
	"github.com/QuitOperation/otcat/internal/protocol"
	"github.com/QuitOperation/otcat/internal/watch"
)

func runRead(ctx context.Context, driver protocol.Driver, enc codec.Encoder, c *config, stderr io.Writer) int {
	rctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	v, err := driver.Read(rctx, c.readSpec)
	if err != nil {
		fmt.Fprintf(stderr, "otcat: read %s: %v\n", c.readSpec, err)
		return classifyErr(err)
	}
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "otcat: writing output: %v\n", err)
		return ExitIO
	}
	return ExitOK
}

func runWatch(ctx context.Context, driver protocol.Driver, enc codec.Encoder, c *config, stderr io.Writer) int {
	opt := watch.Options{Interval: c.interval, MaxBackoff: c.maxBackoff, Count: int(c.count)}

	readFn := func(ctx context.Context) (protocol.Value, error) {
		rctx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()
		return driver.Read(rctx, c.watchSpec)
	}
	emitFn := func(v protocol.Value) error { return enc.Encode(v) }
	onErr := func(err error) {
		fmt.Fprintf(stderr, "otcat: watch %s: %v (retrying)\n", c.watchSpec, err)
	}

	err := watch.Run(ctx, opt, readFn, emitFn, onErr)
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// The only context that cancels here is the one Run() derived
		// from signal.NotifyContext: a clean, operator-requested stop.
		return ExitInterrupted
	default:
		fmt.Fprintf(stderr, "otcat: watch: %v\n", err)
		return ExitIO
	}
}

func runWrite(ctx context.Context, driver protocol.Driver, enc codec.Encoder, c *config, stdin io.Reader, stderr io.Writer) int {
	if code, ok := gateWrite(c, stdin, stderr); !ok {
		return code
	}

	if c.fromStdin {
		return runWriteStream(ctx, driver, enc, c, stdin, stderr)
	}

	wctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if err := driver.Write(wctx, c.writeSpec, c.value); err != nil {
		fmt.Fprintf(stderr, "otcat: write %s: %v\n", c.writeSpec, err)
		return classifyErr(err)
	}
	return emitAck(enc, c.writeSpec, c.value, stderr)
}

func runWriteStream(ctx context.Context, driver protocol.Driver, enc codec.Encoder, c *config, stdin io.Reader, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	line := 0
	for scanner.Scan() {
		line++
		lit := strings.TrimSpace(scanner.Text())
		if lit == "" || strings.HasPrefix(lit, "#") {
			continue
		}
		wctx, cancel := context.WithTimeout(ctx, c.timeout)
		err := driver.Write(wctx, c.writeSpec, lit)
		cancel()
		if err != nil {
			fmt.Fprintf(stderr, "otcat: write %s <- %q (stdin line %d): %v\n", c.writeSpec, lit, line, err)
			return classifyErr(err)
		}
		if code := emitAck(enc, c.writeSpec, lit, stderr); code != ExitOK {
			return code
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "otcat: reading stdin: %v\n", err)
		return ExitIO
	}
	return ExitOK
}

func emitAck(enc codec.Encoder, address, literal string, stderr io.Writer) int {
	ack := protocol.Value{
		Address:   address,
		Type:      "ack",
		Value:     literal,
		Quality:   protocol.QualityGood,
		Timestamp: time.Now().UTC(),
	}
	if err := enc.Encode(ack); err != nil {
		fmt.Fprintf(stderr, "otcat: writing output: %v\n", err)
		return ExitIO
	}
	return ExitOK
}

// gateWrite enforces otcat's one hard safety rule: a write is never
// sent unless it was explicitly confirmed. --confirm skips the prompt
// for scripted use. Otherwise, an interactive terminal gets a y/N
// prompt on stderr (stdout stays a clean data stream even here); a
// non-interactive caller — a cron job, a CI step, anything piping stdin
// that isn't a human at a keyboard — gets refused outright, because a
// silent default-yes on a write path touching live plant equipment is
// the one mistake this tool cannot make.
func gateWrite(c *config, stdin io.Reader, stderr io.Writer) (int, bool) {
	if c.confirm {
		return ExitOK, true
	}
	if c.fromStdin {
		fmt.Fprintln(stderr, "otcat: refusing to write without --confirm (stdin is already consumed by --from-stdin, so no prompt is possible)")
		return ExitWriteAborted, false
	}
	if !isInteractive(stdin) {
		fmt.Fprintln(stderr, "otcat: refusing to write without --confirm (input is not an interactive terminal)")
		return ExitWriteAborted, false
	}
	ok, err := promptYesNo(stdin, stderr, fmt.Sprintf("otcat: write %q to %s? [y/N]: ", c.value, c.writeSpec))
	if err != nil {
		fmt.Fprintf(stderr, "otcat: reading confirmation: %v\n", err)
		return ExitIO, false
	}
	if !ok {
		fmt.Fprintln(stderr, "otcat: write aborted (not confirmed)")
		return ExitWriteAborted, false
	}
	return ExitOK, true
}

func isInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func promptYesNo(stdin io.Reader, stderr io.Writer, prompt string) (bool, error) {
	fmt.Fprint(stderr, prompt)
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return ans == "y" || ans == "yes", nil
}

func runDryRun(driver protocol.Driver, c *config, stdin io.Reader, stdout, stderr io.Writer) int {
	dr, ok := driver.(protocol.DryRunner)
	if !ok {
		fmt.Fprintf(stderr, "otcat: dry-run is not available for this driver (it does not implement its wire protocol yet)\n")
		return ExitUsage
	}
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)

	explain := func(literal string) int {
		plan, err := dr.ExplainWrite(c.writeSpec, literal)
		if err != nil {
			fmt.Fprintf(stderr, "otcat: dry-run %s <- %q: %v\n", c.writeSpec, literal, err)
			return ExitUsage
		}
		if err := enc.Encode(plan); err != nil {
			fmt.Fprintf(stderr, "otcat: writing output: %v\n", err)
			return ExitIO
		}
		return ExitOK
	}

	if !c.fromStdin {
		return explain(c.value)
	}

	scanner := bufio.NewScanner(stdin)
	line, hadErr := 0, false
	for scanner.Scan() {
		line++
		lit := strings.TrimSpace(scanner.Text())
		if lit == "" || strings.HasPrefix(lit, "#") {
			continue
		}
		if code := explain(lit); code != ExitOK {
			hadErr = true
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "otcat: reading stdin: %v\n", err)
		return ExitIO
	}
	if hadErr {
		return ExitUsage
	}
	return ExitOK
}
