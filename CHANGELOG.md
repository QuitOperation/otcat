# Changelog

## v1.0.0 — first stable baseline

This is the first release otcat is designed to be depended on: the
Modbus TCP driver, CLI surface, exit code contract, and output formats
below are meant to stay backward compatible going forward. Breaking
changes to any of them will be a major version bump, not a patch.

### Added

- Modbus TCP driver: coils, discrete inputs, holding registers, input
  registers; function codes 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x0F,
  0x10; classic five-digit addressing with `--raw-address` escape;
  `uint16`/`int16`/`uint32`/`int32`/`float32` with configurable byte
  and word order.
- `--read`, `--watch` (with full-jitter exponential backoff on
  failure), `--write` (with a mandatory confirmation gate and
  `--dry-run`).
- Output formats: newline-delimited JSON (default), CSV, raw scalar.
- Stable exit code contract (0/1/2/3/4/5/130) for scripting.
- `otcat-mockplc` and `otcat-latencyprobe` companion tools.
- Driver architecture with registered-but-stubbed EtherNet/IP, S7comm,
  and BACnet slots (`protocol.ErrNotImplemented`).

### Known limitations (see `docs/driver_roadmap.md`)

- EtherNet/IP, S7comm, and BACnet do not yet speak their wire
  protocols.
- Modbus RTU/ASCII (serial) transport is not implemented — TCP only.
- No connection pooling / concurrent-request pipelining by design (see
  `internal/modbus/client.go` doc comment).
