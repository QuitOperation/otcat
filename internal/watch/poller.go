// Package watch drives the --watch polling loop shared by every driver.
package watch

import (
	"context"
	"math/rand"
	"time"

	"github.com/QuitOperation/otcat/internal/protocol"
)

type ReadFunc func(ctx context.Context) (protocol.Value, error)
type EmitFunc func(protocol.Value) error
type ErrFunc func(error)

type Options struct {
	Interval   time.Duration // steady-state cadence between successful reads
	MaxBackoff time.Duration // ceiling on retry delay after failures
	Count      int           // 0 = run until ctx is cancelled
}

// Run polls fn every Interval while it succeeds. On failure it retries
// with full-jitter exponential backoff instead of hammering Interval
// forever: a device that drops off the network should be probed less
// often the longer it stays gone, and — with more than one otcat on the
// same segment, or one otcat re-polling several tags — jitter keeps
// retries from synchronizing into a periodic burst against a device
// that is in the middle of recovering. See docs/backoff.md for the
// full derivation and citation.
func Run(ctx context.Context, opt Options, fn ReadFunc, emit EmitFunc, onErr ErrFunc) error {
	if opt.Interval <= 0 {
		opt.Interval = time.Second
	}
	if opt.MaxBackoff <= 0 {
		opt.MaxBackoff = 30 * time.Second
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	attempt := 0
	emitted := 0

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		v, err := fn(ctx)
		if err != nil {
			if onErr != nil {
				onErr(err)
			}
			attempt++
			if !sleep(ctx, fullJitterBackoff(rng, opt.Interval, opt.MaxBackoff, attempt)) {
				return ctx.Err()
			}
			continue
		}

		attempt = 0
		if err := emit(v); err != nil {
			return err
		}
		emitted++
		if opt.Count > 0 && emitted >= opt.Count {
			return nil
		}

		if !sleep(ctx, opt.Interval) {
			return ctx.Err()
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// fullJitterBackoff implements delay = Uniform(0, min(max, base*2^(a-1)))
// for attempt a >= 1 — the "Full Jitter" strategy from Brooker,
// "Exponential Backoff And Jitter," AWS Architecture Blog, 2015, chosen
// over plain exponential backoff (which is fully synchronized across
// clients: every failed client retries at exactly the same instants) and
// over "Equal Jitter" (which never lets delay reach zero, wasting the
// low end of the distribution). Full Jitter minimizes expected total
// client-side wait for a given amount of collision avoidance.
func fullJitterBackoff(rng *rand.Rand, base, max time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exp := attempt - 1
	const expCeiling = 20 // 2^20 * base is already far beyond any sane max; clamp avoids int64 overflow
	if exp > expCeiling {
		exp = expCeiling
	}
	upper := base * time.Duration(int64(1)<<uint(exp))
	if upper <= 0 || upper > max { // overflow (<=0) or past ceiling both saturate to max
		upper = max
	}
	if upper <= 0 {
		return 0
	}
	return time.Duration(rng.Int63n(int64(upper) + 1))
}
