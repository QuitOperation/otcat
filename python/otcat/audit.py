"""Read-only auditing helpers for OT asset discovery and exposure
assessment -- the Python-side equivalent of the Go core's own
safety-first ethos (see the accompanying paper, "Safety Is a Default,
Not a Feature"): every function in this module only ever issues reads.
There is no scan-and-write mode, no fuzzer, no credential attack (moot
anyway -- Modbus has no authentication to attack), and no function that
sends more than one request at a time to a single device. That is a
deliberate, permanent design boundary for this module, not a version-1
limitation to be lifted later.

Intended use: legitimate asset inventory, exposure mapping, and
red-team reconnaissance *you are authorized to perform* against
equipment you are authorized to test. Running any scanner, including
this one, against equipment you do not have explicit authorization to
test may be illegal in your jurisdiction and can disrupt a live
physical process regardless of intent -- see the Go core's own
Limitations section for why Modbus's total absence of authentication
means "I can reach it" has never implied "I am allowed to."
"""
from __future__ import annotations

import time
from dataclasses import dataclass, field

from .client import Client
from .exceptions import ConnectionError as OtcatConnectionError
from .exceptions import ProtocolError


@dataclass
class ScanResult:
    table: str
    address: int
    responsive: bool
    exception_code: str | None = None
    latency_ms: float | None = None


@dataclass
class ScanReport:
    endpoint: str
    table: str
    start: int
    count: int
    results: list[ScanResult] = field(default_factory=list)

    @property
    def responsive_addresses(self) -> list[int]:
        return [r.address for r in self.results if r.responsive]

    @property
    def response_rate(self) -> float:
        return len(self.responsive_addresses) / len(self.results) if self.results else 0.0

    def summary(self) -> str:
        lat = [r.latency_ms for r in self.results if r.latency_ms is not None]
        avg_lat = sum(lat) / len(lat) if lat else float("nan")
        return (
            f"{self.endpoint} {self.table}:{self.start}..{self.start + self.count - 1}: "
            f"{len(self.responsive_addresses)}/{len(self.results)} addresses responsive "
            f"({self.response_rate:.0%}), avg latency {avg_lat:.1f}ms"
        )


def scan_range(
    client: Client,
    table: str,
    start: int,
    count: int,
    *,
    delay_seconds: float = 0.0,
) -> ScanReport:
    """Sweep [start, start+count) with individual single-value reads,
    recording which addresses respond, which raise a protocol
    exception (and which one), and per-address latency. Read-only,
    always -- see the module docstring.

    delay_seconds paces requests between addresses; the default (0)
    sends as fast as the client allows, which is fine for otcat's own
    mock server and most modern devices, but consider a small delay
    (e.g. 0.05) against older or resource-constrained field hardware,
    the same courtesy the paper's own write-safety design extends
    throughout: a scanner should not become a denial-of-service tool
    by accident.
    """
    report = ScanReport(endpoint=client.endpoint, table=table, start=start, count=count)
    for offset in range(count):
        addr = start + offset
        t0 = time.perf_counter()
        try:
            client.read(f"{table}:{addr}")
            latency_ms = (time.perf_counter() - t0) * 1000
            report.results.append(ScanResult(table, addr, responsive=True, latency_ms=latency_ms))
        except ProtocolError as e:
            latency_ms = (time.perf_counter() - t0) * 1000
            report.results.append(
                ScanResult(table, addr, responsive=False, exception_code=e.exception_code, latency_ms=latency_ms)
            )
        except OtcatConnectionError:
            report.results.append(ScanResult(table, addr, responsive=False))
        if delay_seconds:
            time.sleep(delay_seconds)
    return report


@dataclass
class DeviceFingerprint:
    endpoint: str
    reachable: bool
    tables_responsive: dict[str, bool] = field(default_factory=dict)
    latency_ms_p50: float | None = None
    latency_ms_p99: float | None = None
    samples: int = 0


def fingerprint(client: Client, *, samples: int = 20, probe_address: int = 0) -> DeviceFingerprint:
    """A lightweight, read-only device fingerprint: which of the four
    Modbus data tables respond at all at probe_address, and the
    round-trip latency distribution over `samples` reads of one
    holding register -- useful groundwork for both an asset inventory
    entry and a latency budget for how aggressively you can safely
    poll this specific device (see the paper's own methodology for why
    latency percentiles, not just a mean, are the right thing to record
    here).
    """
    fp = DeviceFingerprint(endpoint=client.endpoint, reachable=False)

    for table in ("holding", "input", "coil", "discrete"):
        try:
            client.read(f"{table}:{probe_address}")
            fp.tables_responsive[table] = True
            fp.reachable = True
        except ProtocolError:
            # Reached the device; this table/address combination just
            # isn't valid there. Still evidence of reachability.
            fp.tables_responsive[table] = False
            fp.reachable = True
        except OtcatConnectionError:
            fp.tables_responsive[table] = False

    if fp.reachable:
        latencies: list[float] = []
        for _ in range(samples):
            t0 = time.perf_counter()
            try:
                client.read(f"holding:{probe_address}")
                latencies.append((time.perf_counter() - t0) * 1000)
            except (ProtocolError, OtcatConnectionError):
                pass
        if latencies:
            latencies.sort()
            fp.samples = len(latencies)
            fp.latency_ms_p50 = latencies[len(latencies) // 2]
            fp.latency_ms_p99 = latencies[min(len(latencies) - 1, int(len(latencies) * 0.99))]

    return fp
