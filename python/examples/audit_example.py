"""A read-only asset audit: fingerprint a device, then sweep a table
range and report which addresses respond. Never writes -- see
otcat.audit's module docstring for why that's a permanent property of
this module, not a flag you can turn off.

Only run this against equipment you are authorized to test.

    otcat-mockplc --addr 127.0.0.1:15020 &
    python examples/audit_example.py 127.0.0.1:15020
"""
import sys

from otcat import Client
from otcat.audit import fingerprint, scan_range


def main() -> None:
    endpoint = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1:15020"
    client = Client(endpoint, raw_address=True, timeout=2.0)

    print(f"fingerprinting {endpoint} ...")
    fp = fingerprint(client, samples=20)
    if not fp.reachable:
        print("device unreachable")
        return
    print(f"  reachable: {fp.reachable}")
    print(f"  tables responsive: {fp.tables_responsive}")
    print(f"  latency p50={fp.latency_ms_p50:.2f}ms p99={fp.latency_ms_p99:.2f}ms "
          f"({fp.samples} samples)")

    print()
    print("scanning holding:0..49 ...")
    report = scan_range(client, "holding", start=0, count=50, delay_seconds=0.0)
    print("  " + report.summary())
    print(f"  responsive addresses: {report.responsive_addresses[:10]}"
          f"{' ...' if len(report.responsive_addresses) > 10 else ''}")


if __name__ == "__main__":
    main()
