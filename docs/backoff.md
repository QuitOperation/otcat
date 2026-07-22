# Why --watch backs off the way it does

## The problem

`otcat --watch` polls one address on a fixed `--interval`. When a read
fails — the device dropped off the network, a gateway rebooted, a
cable got unplugged during a panel swap — the naive behavior is to keep
retrying at the same `--interval`. Two things go wrong with that:

1. **It hammers a device that is in the middle of recovering.** A PLC
   or gateway rebooting after a power blip is exactly the device least
   able to handle a burst of reconnect attempts arriving every 100 ms.
2. **Retries synchronize.** If two otcat processes (or otcat and some
   other Modbus master) are both polling the same segment and it goes
   down, plain periodic retry means they all come back at the same
   instants, forever, each attempt landing as a synchronized burst
   instead of being spread out.

## The fix: Full Jitter exponential backoff

`internal/watch/poller.go` implements the "Full Jitter" strategy from
Marc Brooker, *"Exponential Backoff And Jitter,"* AWS Architecture
Blog, 2015:

```
delay(attempt) = Uniform(0, min(max_backoff, base * 2^(attempt-1)))
```

Each failed attempt doubles the *ceiling* of the delay, and the actual
delay is drawn uniformly from zero up to that ceiling — not fixed at
the ceiling. This is deliberately different from two more obvious
designs:

- **Plain exponential backoff** (`delay = base * 2^attempt`, no
  randomness) still perfectly synchronizes every client that failed at
  the same time: they all retry at exactly the same instants, just
  spaced further apart. It reduces load but not correlation.
- **"Equal Jitter"** (`delay = ceiling/2 + Uniform(0, ceiling/2)`) fixes
  correlation but never lets the delay approach zero, which wastes the
  fast-recovery case: if the device is actually back after 50 ms, Equal
  Jitter's minimum floor (built into `ceiling/2`) still forces a longer
  wait.

Full Jitter's own analysis (see the post above) shows it minimizes
total client-side work (fewer retries sent, on average, for the same
collision-avoidance benefit) among the three, at the cost of slightly
higher variance in any single client's recovery time — a trade otcat's
use case (a monitoring/commissioning tool, not a hard-real-time control
loop) accepts gladly.

## Why the base resets to `--interval`, not zero

The exponent resets to 0 after every *successful* read (see `attempt =
0` in `Run`), so a single transient failure costs at most one
`Uniform(0, --interval)` delay before returning to steady-state polling
— otcat does not "remember" past trouble once the device is answering
again. This matters for the common case of an occasional dropped packet
that is not indicative of a real outage.

## Why `--max-backoff` exists at all

Without a ceiling, `base * 2^attempt` overflows into absurd (multi-year)
wait times after a couple of dozen consecutive failures — the exact
scenario of "otcat has been running unattended against a device that
was decommissioned three weeks ago." `--max-backoff` (default 30s)
keeps otcat checking at a bounded, human-scale cadence indefinitely
instead of effectively giving up.
