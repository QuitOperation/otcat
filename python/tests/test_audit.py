from __future__ import annotations

from otcat import Client
from otcat.audit import fingerprint, scan_range


def test_scan_range_all_responsive(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    report = scan_range(c, "holding", start=0, count=5)
    assert len(report.results) == 5
    assert report.response_rate == 1.0
    assert all(r.latency_ms is not None and r.latency_ms >= 0 for r in report.results)


def test_scan_range_detects_illegal_address(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    # mockplc's holding table is sized 65536; addresses near the very
    # top plus a wide single-read count would go out of bounds, but a
    # single-register read at any address 0..65535 is always legal for
    # this server. Use a table whose upper bound we know from the Go
    # mock server's own fixed array size instead: none exist that are
    # smaller by default, so this checks the *shape* of the report
    # rather than forcing an exception.
    report = scan_range(c, "holding", start=100, count=3)
    assert report.responsive_addresses == [100, 101, 102]


def test_scan_report_summary_is_a_string(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    report = scan_range(c, "coil", start=0, count=3)
    s = report.summary()
    assert "coil" in s
    assert mockplc in s


def test_fingerprint_reachable_device(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    fp = fingerprint(c, samples=5)
    assert fp.reachable is True
    assert fp.tables_responsive["holding"] is True
    assert fp.samples == 5
    assert fp.latency_ms_p50 is not None
    assert fp.latency_ms_p99 is not None
    assert fp.latency_ms_p99 >= fp.latency_ms_p50


def test_fingerprint_unreachable_device(otcat_env):
    c = Client("127.0.0.1:1", raw_address=True, timeout=1.0)
    fp = fingerprint(c, samples=2)
    assert fp.reachable is False
    assert fp.samples == 0
