"""Threshold alerting: page/log/trigger an automation when a tank
level crosses a limit, with debounce so noise near the threshold
doesn't spam.

    otcat-mockplc --addr 127.0.0.1:15020 &
    python examples/alerting_example.py 127.0.0.1:15020
"""
import sys

from otcat import Client
from otcat.alerting import AlertEngine


def page_oncall(value) -> None:
    print(f"[ALERT] {value.address} = {value.value} at {value.ts} -- paging on-call")


def clear_alert(value) -> None:
    print(f"[CLEAR] {value.address} = {value.value} at {value.ts} -- back to normal")


def trigger_automation(value) -> None:
    print(f"[AUTOMATION] {value.address} crossed threshold -- running downstream action")


def main() -> None:
    endpoint = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1:15020"
    client = Client(endpoint, raw_address=True)

    engine = AlertEngine(client)
    engine.threshold(
        "holding:0", above=25_00,  # tank level is x100 fixed point; 25.00%
        on_trigger=page_oncall, on_clear=clear_alert, debounce=3,
        name="tank-high",
    )
    engine.threshold(
        "holding:0", above=21_00,
        on_trigger=trigger_automation, debounce=2,
        name="tank-automation-threshold",
    )

    print(f"watching {endpoint} holding:0, polling every 500ms (Ctrl+C to stop)")
    engine.run(interval_seconds=0.5)


if __name__ == "__main__":
    main()
