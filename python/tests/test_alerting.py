from __future__ import annotations

from otcat import Client
from otcat.alerting import AlertEngine, Rule, Threshold


def test_threshold_triggers_after_debounce(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    c.write("holding:0", 50)  # below any real tank value, deterministic start point

    triggered = []
    engine = AlertEngine(c)
    engine.threshold("holding:0", above=40, on_trigger=triggered.append, debounce=2, name="high")

    # First poll: 50 > 40, breach #1 -- must NOT trigger yet (debounce=2)
    engine.poll_once()
    assert triggered == []

    # Second poll: breach #2 -- now it must trigger
    engine.poll_once()
    assert len(triggered) == 1
    assert triggered[0].value == 50


def test_threshold_clears(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    c.write("holding:0", 100)

    triggered, cleared = [], []
    engine = AlertEngine(c)
    engine.threshold(
        "holding:0", above=40, on_trigger=triggered.append, on_clear=cleared.append, debounce=1
    )
    engine.poll_once()
    assert len(triggered) == 1

    c.write("holding:0", 0)
    engine.poll_once()
    assert len(cleared) == 1


def test_custom_predicate_rule(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    c.write("coil:5", "true")

    triggered = []
    engine = AlertEngine(c)
    engine.add_rule(
        Rule(spec="coil:5", predicate=lambda v: v is True, on_trigger=triggered.append, debounce=1)
    )
    engine.poll_once()
    assert len(triggered) == 1


def test_run_with_bounded_iterations(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    c.write("holding:0", 999)
    triggered = []
    engine = AlertEngine(c)
    engine.threshold("holding:0", above=1, on_trigger=triggered.append, debounce=1)
    engine.run(interval_seconds=0.01, iterations=3)
    assert len(triggered) == 1  # fires once, stays active, doesn't re-fire


def test_failed_read_does_not_crash_engine(otcat_env):
    # points at nothing listening; poll_once must swallow the read
    # error and move on rather than propagating it out of the engine.
    c = Client("127.0.0.1:1", raw_address=True, timeout=1.0)
    engine = AlertEngine(c)
    engine.threshold("holding:0", above=1, on_trigger=lambda v: None, debounce=1)
    engine.poll_once()  # must not raise
