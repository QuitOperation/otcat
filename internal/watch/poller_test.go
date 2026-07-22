package watch

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/QuitOperation/otcat/internal/protocol"
)

func TestRunStopsAtCount(t *testing.T) {
	n := 0
	fn := func(ctx context.Context) (protocol.Value, error) {
		n++
		return protocol.Value{}, nil
	}
	emitted := 0
	err := Run(context.Background(), Options{Interval: time.Millisecond, Count: 5}, fn,
		func(protocol.Value) error { emitted++; return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if emitted != 5 {
		t.Fatalf("emitted %d values, want 5", emitted)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fn := func(ctx context.Context) (protocol.Value, error) { return protocol.Value{}, nil }
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	err := Run(ctx, Options{Interval: 5 * time.Millisecond}, fn,
		func(protocol.Value) error { return nil }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestRunPropagatesEmitError(t *testing.T) {
	sentinel := errors.New("broken pipe")
	fn := func(ctx context.Context) (protocol.Value, error) { return protocol.Value{}, nil }
	err := Run(context.Background(), Options{Interval: time.Millisecond}, fn,
		func(protocol.Value) error { return sentinel }, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want sentinel emit error", err)
	}
}

func TestRunRetriesOnReadError(t *testing.T) {
	attempts := 0
	fn := func(ctx context.Context) (protocol.Value, error) {
		attempts++
		if attempts < 3 {
			return protocol.Value{}, errors.New("transient")
		}
		return protocol.Value{}, nil
	}
	errCount := 0
	err := Run(context.Background(), Options{Interval: time.Millisecond, MaxBackoff: 5 * time.Millisecond, Count: 1}, fn,
		func(protocol.Value) error { return nil },
		func(error) { errCount++ })
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3 (2 failures + 1 success)", attempts)
	}
	if errCount != 2 {
		t.Fatalf("onErr called %d times, want 2", errCount)
	}
}

func TestFullJitterBackoffBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	base := 100 * time.Millisecond
	max := 2 * time.Second

	for attempt := 1; attempt <= 15; attempt++ {
		expectedCeiling := base * time.Duration(int64(1)<<uint(min(attempt-1, 20)))
		if expectedCeiling > max || expectedCeiling <= 0 {
			expectedCeiling = max
		}
		for i := 0; i < 200; i++ {
			d := fullJitterBackoff(rng, base, max, attempt)
			if d < 0 {
				t.Fatalf("attempt %d: negative backoff %v", attempt, d)
			}
			if d > expectedCeiling {
				t.Fatalf("attempt %d: backoff %v exceeds ceiling %v", attempt, d, expectedCeiling)
			}
			if d > max {
				t.Fatalf("attempt %d: backoff %v exceeds global max %v", attempt, d, max)
			}
		}
	}
}

func TestFullJitterBackoffFirstAttemptCanBeZero(t *testing.T) {
	// Full Jitter's defining property vs. "Equal Jitter": the first
	// retry's delay distribution must include values arbitrarily close
	// to zero, not be bounded away from it.
	rng := rand.New(rand.NewSource(1))
	base := 100 * time.Millisecond
	sawSmall := false
	for i := 0; i < 2000; i++ {
		if fullJitterBackoff(rng, base, time.Second, 1) < 5*time.Millisecond {
			sawSmall = true
			break
		}
	}
	if !sawSmall {
		t.Fatal("expected at least one near-zero backoff among 2000 samples at attempt=1")
	}
}

func TestFullJitterBackoffZeroAttempt(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if d := fullJitterBackoff(rng, time.Second, time.Minute, 0); d < 0 {
		t.Fatalf("attempt=0 produced negative duration %v", d)
	}
}
