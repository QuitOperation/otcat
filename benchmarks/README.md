# Benchmark data provenance

Every file in this directory is machine output, reproducible with the
commands below. Nothing here is hand-edited or estimated.

| file | produced by |
|------|-------------|
| `raw_results.txt` | `go test ./... -run=^$ -bench=. -benchmem -benchtime=1s -count=3` |
| `summary.json` | mean of the three runs in `raw_results.txt`, computed by `scripts/plot_results.py`'s inline summarizer |
| `latency_loopback.json` | `otcat-latencyprobe -n 10000 -registers 10 -json -samples-out latency_samples_us.txt` |
| `latency_samples_us.txt` | raw per-request latency in microseconds, one per line, from the same run |
| `tank_watch.ndjson` | `otcat --modbus 127.0.0.1:PORT --watch holding:0 --raw-address --interval 100ms --count 300 --json`, against a running `otcat-mockplc`, i.e. the actual CLI binary talking over an actual TCP socket to an actual (if simulated-physics) Modbus TCP server |

## Environment

All numbers were collected on a single-core, 2.10 GHz Intel Xeon
sandbox VM (`nproc` = 1) over TCP loopback (`127.0.0.1`) — see
`paper/otcat.tex` §Evaluation for what that does and does not tell you.
Loopback numbers measure otcat's own encode/decode/dispatch overhead
with real network latency approximately zeroed out; they are a ceiling
on achievable throughput, not a prediction of performance against a
real PLC on a real LAN, where request/response latency will be
dominated by the device's own response time (commonly 1–20 ms for a
serial-backed TCP gateway) rather than by otcat.

## Reproducing

```sh
go test ./... -run=^$ -bench=. -benchmem -benchtime=1s -count=3 | tee benchmarks/raw_results.txt

go build -o /tmp/otcat-latencyprobe ./cmd/otcat-latencyprobe
/tmp/otcat-latencyprobe -n 10000 -registers 10 -json \
  -samples-out benchmarks/latency_samples_us.txt \
  > benchmarks/latency_loopback.json

go build -o /tmp/otcat ./cmd/otcat
go build -o /tmp/otcat-mockplc ./cmd/otcat-mockplc
/tmp/otcat-mockplc --addr 127.0.0.1:15021 &
/tmp/otcat --modbus 127.0.0.1:15021 --watch holding:0 --raw-address \
  --interval 100ms --count 300 --json > benchmarks/tank_watch.ndjson
kill %1

python3 scripts/plot_results.py   # writes paper/figures/*.pdf
```
