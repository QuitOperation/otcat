# otcat (Python)

Python bindings for [otcat](https://github.com/QuitOperation/otcat),
the netcat for industrial I/O. This package wraps the real, compiled
Go binary — every Modbus read and write is still executed by the same
tested, fuzzed Go core the main project's paper describes, not a
Python reimplementation of the protocol.

```python
from otcat import Client

c = Client("127.0.0.1:502")
v = c.read("holding:40001")
print(v.value, v.quality, v.ts)

for v in c.watch("holding:40001", interval="500ms", count=10):
    print(v.ts, v.value)

c.write("holding:40001", 100)  # confirm=True by default -- see "Write safety" below
```

## Install

```sh
pip install otcat                 # core client only, zero extra dependencies
pip install otcat[pandas]         # + DataFrame helpers
pip install otcat[fastapi]        # + the async client's natural home
pip install otcat[dashboard]      # + Streamlit
pip install otcat[all]            # everything
```

The Go binary itself ships bundled inside platform-specific wheels; if
none is available for your platform, install it separately (`go
install github.com/QuitOperation/otcat/cmd/otcat@latest`, or one of the
[Cloudsmith packages](../docs/releasing.md)) and either put it on
`$PATH` or point `OTCAT_BINARY` at it directly. See
[`docs/packaging.md`](docs/packaging.md) for exactly how the bundled
binary is resolved.

## Write safety: one deliberate difference from the CLI

The Go CLI refuses every write by default unless `--confirm` is passed
or an interactive operator answers a y/N prompt — the right default
for a human typing a command who might have a typo. `Client.write()`
defaults to `confirm=True` instead, because a library call is already
the result of a programmer's code deciding, deliberately, to write a
specific value; there is no keystroke left to protect against. Pass
`confirm=False` if you want the stricter behavior and are prepared to
catch `WriteAbortedError`.

## Modules

| Module | What it's for |
|---|---|
| `otcat.Client` | Synchronous read/write/watch/dry_run — the core |
| `otcat.aio.AsyncClient` | Same API, `asyncio`-native — FastAPI, WebSockets |
| `otcat.pandas_ext` | `Value` lists ↔ `DataFrame`, plus a bounded `RollingBuffer` for live dashboards |
| `otcat.alerting` | Threshold rules with debounce → callbacks, for automations and paging |
| `otcat.audit` | Read-only OT asset discovery / device fingerprinting (never writes — see its module docstring) |

## Examples

See [`examples/`](examples/): a pandas time-series pull, a FastAPI
service with a live WebSocket, a Streamlit dashboard, a threshold
alerting script, and a read-only network audit script.

## Exit codes → exceptions

| Go CLI exit code | Python exception |
|---|---|
| 1 | `otcat.UsageError` |
| 2 | `otcat.ConnectionError` |
| 3 | `otcat.ProtocolError` (`.exception_code` holds the Modbus exception, e.g. `"0x02"`) |
| 4 | `otcat.WriteAbortedError` |
| 5 | `otcat.IOFailureError` |
| (n/a) | `otcat.Timeout` — a Python-side watchdog, distinct from otcat's own `--timeout` |

## Testing

```sh
pip install -e ".[dev]"
pytest
```

Every test is an integration test against a freshly spawned
`otcat-mockplc` — see `tests/conftest.py`. Building `otcat-mockplc`
requires a Go 1.22+ toolchain on `$PATH` at test time (the shipped
wheel itself needs no Go toolchain to *use*, only this repo's test
suite needs one to build its own test fixture).

## License

MIT — same as the Go core.
