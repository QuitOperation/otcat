<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/otcat-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/otcat-light.png">
    <img alt="otcat logo" src="assets/otcat-light.png" width="180">
  </picture>
</p>

# otcat — the netcat for industrial I/O

Read, write, and watch PLC registers from the shell, the way `nc`
lets you read, write, and watch a TCP socket.

```sh
# read a holding register, print it as JSON
otcat --modbus 192.168.1.10:502 --read holding:40001 --json

# watch a register twice a second and pipe the bare number into awk
otcat --modbus 192.168.1.10:502 --watch holding:40001 --interval 500ms --raw | awk '{print $1*0.1}'

# write, with an explicit confirmation gate
otcat --modbus 192.168.1.10:502 --write holding:40001 --value 100 --confirm

# stream a column of setpoints from a file straight into a register
cat setpoints.txt | otcat --modbus 192.168.1.10:502 --write holding:40001 --from-stdin --confirm
```

No config file, no server, no vendor client. One static binary, stdin
and stdout, exit codes a script can branch on.

**Less typing:** `otc` is otcat under its short name, the same
relationship `nc` has to netcat — identical flags, identical behavior,
just three fewer characters for a command you'll type constantly.

```sh
go install github.com/QuitOperation/otcat/cmd/otc@latest
otc --modbus 192.168.1.10:502 --read holding:40001 --json
```

If you'd rather build only `otcat` and keep the short name as a shell
alias instead of a second binary:

```sh
echo 'alias otc=otcat' >> ~/.bashrc   # or ~/.zshrc
```

<p align="center">
  <img alt="otcat quickstart demo" src="demo/gifs/quickstart.gif" width="720">
</p>

## What actually works right now

**Modbus TCP is complete**: all four data tables (coils, discrete
inputs, holding registers, input registers), all eight data-access
function codes, classic `40001`-style addressing *and* raw offsets,
`uint16`/`int16`/`uint32`/`int32`/`float32` with configurable byte and
word order, single and multi-register/coil reads and writes, exception
decoding, and a `--watch` mode with jittered exponential backoff on
failure.

**EtherNet/IP, S7comm, and BACnet are registered but not implemented.**
`--eip`, `--s7comm`, and `--bacnet` parse, appear in `--help`, and fail
immediately and explicitly with a clear "not implemented" error. This
is deliberate: getting a write path to a live controller subtly wrong
is worse than not shipping it. See [`docs/driver_roadmap.md`](docs/driver_roadmap.md)
for exactly what each one needs and in what order they'd be built.

## Install / build

**Debian/Ubuntu, Fedora/RHEL, Alpine** — native packages via
Cloudsmith, **any OS with Go** — `go install`, **everyone else** —
prebuilt archives for linux/macOS/windows × amd64/arm64 from the
[Releases page](https://github.com/QuitOperation/otcat/releases),
built by [GoReleaser](https://goreleaser.com). Full instructions for
all of these: [`docs/releasing.md`](docs/releasing.md).

```sh
go install github.com/QuitOperation/otcat/cmd/otcat@latest
```

Building from source requires Go 1.22+. Zero external dependencies —
the whole tree, Modbus included, is standard library only.

```sh
git clone https://github.com/QuitOperation/otcat.git
cd otcat
go build -o otcat ./cmd/otcat
go build -o otc ./cmd/otc          # same binary, short name — see below
```

Or, once a release is tagged:

```sh
go install github.com/QuitOperation/otcat/cmd/otcat@latest
```

Two more binaries ship alongside it, useful for trying otcat without
owning a PLC:

```sh
go build -o otcat-mockplc ./cmd/otcat-mockplc          # an in-memory Modbus TCP server with a simulated tank-level loop
go build -o otcat-latencyprobe ./cmd/otcat-latencyprobe # measures real read-latency percentiles against any Modbus endpoint
```

```sh
./otcat-mockplc --addr 127.0.0.1:15020 &
./otcat --modbus 127.0.0.1:15020 --watch holding:0 --raw-address --interval 500ms
```

## Address spec

```
table:address[:count]
```

- `table` is one of `coil`, `discrete`, `holding`, `input`.
- `address` is either a classic five-digit reference (`40001`) or, with
  `--raw-address`, a literal 0-based wire offset. See
  [`docs/classic_addressing.md`](docs/classic_addressing.md) for exactly
  how otcat resolves the two.
- `count` is optional; it defaults to the register width of `--type`
  (1 for the 16-bit types, 2 for the 32-bit types).

## Write safety

Every write is refused unless one of the following is true:

- `--confirm` was passed (the flag for scripts and automation), or
- stdin is an interactive terminal and the operator answers `y` to a
  prompt printed on stderr (stdout stays a clean data stream even here).

`--dry-run` reports the exact registers/coils a write *would* send —
including validating every line of a `--from-stdin` file — without
opening a connection. There is no flag that makes otcat write silently
by default; that is intentional.

<p align="center">
  <img alt="otcat write-safety demo" src="demo/gifs/write_safety.gif" width="720">
</p>

## Output formats

- `--json` (default): newline-delimited JSON, one object per line —
  streamable, `jq`-able, never buffered across values.
- `--csv`: header row once, one row per value; array-valued reads are
  semicolon-joined into a single field.
- `--raw`: the bare scalar, nothing else — for piping straight into
  `awk`, `bc`, or a shell arithmetic expansion.

<p align="center">
  <img alt="otcat piping into jq, awk, and grep" src="demo/gifs/pipes.gif" width="720">
</p>

## Exit codes

| code | meaning |
|------|---------|
| 0    | success |
| 1    | usage error — bad flags, malformed address spec, bad literal |
| 2    | connection error — dial/timeout/refused |
| 3    | protocol error — the device returned a Modbus exception |
| 4    | write aborted — not confirmed |
| 5    | I/O error — e.g. broken output pipe |
| 130  | `--watch` stopped cleanly by SIGINT/SIGTERM |

## Python bindings

For pandas/ML/dashboards/FastAPI/alerting/OT auditing work, `python/`
ships a typed Python package wrapping this same Go core over a
subprocess boundary — every read and write still goes through the
real, tested Modbus client, not a Python reimplementation.

```python
from otcat import Client
c = Client("192.168.1.10:502")
v = c.read("holding:40001")
```

```sh
pip install otcat                 # core client, zero extra dependencies
pip install otcat[pandas]         # + DataFrame helpers
pip install otcat[fastapi]        # + async client's natural home
```

Covers `pandas` DataFrame conversion and a rolling-window buffer, an
`asyncio`-native client for FastAPI/WebSockets, a threshold-based
alerting engine with debounce, and a read-only OT asset-discovery/
fingerprinting module. Full docs, examples (pandas, FastAPI,
Streamlit, alerting, audit), and 36 passing integration tests against
a real mock server: [`python/README.md`](python/README.md).

## Testing and benchmarks

```sh
go test ./...                      # full suite
go test ./... -race -cover         # race detector + coverage
go test ./... -bench=. -benchmem   # benchmarks (see benchmarks/README.md for how the paper's numbers were produced)
go test ./internal/modbus/ -fuzz=FuzzDecodeMBAP -fuzztime=60s   # coverage-guided fuzzing, one target at a time
```

No PLC on hand? `docs/interop_testing.md` covers running otcat against
two independent, widely-used open-source Modbus implementations
(Python's pymodbus, Node's modbus-serial) instead of otcat's own mock
server — real cross-implementation agreement, not self-confirmation.

## Project layout

```
cmd/otcat              the CLI binary
cmd/otc                same binary, short name (nc : netcat :: otc : otcat)
cmd/otcat-mockplc       standalone mock Modbus TCP server (used above)
cmd/otcat-latencyprobe  latency-percentile measurement tool
internal/protocol       the Driver contract every backend implements
internal/modbus         the complete Modbus TCP driver
internal/eip            EtherNet/IP driver stub (not implemented)
internal/s7             S7comm driver stub (not implemented)
internal/bacnet         BACnet/IP driver stub (not implemented)
internal/codec          output formatters (json/csv/raw)
internal/watch          the --watch polling loop and backoff
internal/mock           in-memory Modbus TCP server used by tests, benchmarks, and demos
internal/cliapp         flag parsing, dispatch, safety gating, exit codes
docs/                    design-decision write-ups referenced from code comments
paper/                   the accompanying IEEE-format technical paper and its source data
demo/                    terminal recordings (demo/gifs) and their source scripts/VHS tapes
assets/                  logo (dark/light)
python/                  Python bindings (pandas/FastAPI/alerting/audit) -- see python/README.md
```

## License

MIT — see [`LICENSE`](LICENSE).
