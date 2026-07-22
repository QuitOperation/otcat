"""A small rule-based alerting engine for turning a watch() stream into
callbacks -- the building block for "page someone when the tank gets
above 90%" or "trigger a downstream automation when a coil flips."

Deliberately minimal: no rule DSL, no persistence, no distributed
state. A Rule is a predicate and two callbacks (on_trigger, on_clear);
the engine's only real logic is debouncing so a value oscillating
right at a threshold doesn't fire on every single sample.
"""
from __future__ import annotations

import logging
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from .client import Client
from .models import Value

logger = logging.getLogger("otcat.alerting")


@dataclass
class Rule:
    """spec: address to watch. predicate: called with the decoded
    value; return True when the alert condition is met. on_trigger is
    called once when the rule transitions False->True (after
    `debounce` consecutive breaching samples, not on the first one --
    see AlertEngine's docstring for why). on_clear, if given, is
    called once on the True->False transition."""

    spec: str
    predicate: Callable[[Any], bool]
    on_trigger: Callable[[Value], None]
    on_clear: Callable[[Value], None] | None = None
    debounce: int = 1
    name: str = ""

    # internal state
    _consecutive: int = field(default=0, repr=False, compare=False)
    _active: bool = field(default=False, repr=False, compare=False)

    def __post_init__(self):
        self.name = self.name or self.spec


@dataclass
class Threshold:
    """A Rule built from a plain numeric threshold, for the common
    case that doesn't need a custom predicate.

    >>> Threshold(spec="holding:0", above=9000).as_rule(on_trigger=page_oncall)
    """

    spec: str
    above: float | None = None
    below: float | None = None
    debounce: int = 3
    name: str = ""

    def as_rule(
        self,
        on_trigger: Callable[[Value], None],
        on_clear: Callable[[Value], None] | None = None,
    ) -> Rule:
        above, below = self.above, self.below

        def predicate(value: Any) -> bool:
            v = float(value)
            if above is not None and v > above:
                return True
            if below is not None and v < below:
                return True
            return False

        return Rule(
            spec=self.spec,
            predicate=predicate,
            on_trigger=on_trigger,
            on_clear=on_clear,
            debounce=self.debounce,
            name=self.name or self.spec,
        )


class AlertEngine:
    """Polls a set of Rules, one otcat --watch subprocess per unique
    spec, and dispatches on_trigger/on_clear callbacks with debounce.

    Debounce exists because a raw threshold on a live signal will
    otherwise fire, clear, and re-fire every time noise crosses the
    line -- exactly the alert-fatigue failure mode any real automation
    or paging system needs to avoid. `debounce=N` requires N
    consecutive breaching samples before on_trigger fires, and (by the
    same logic, for the same reason) N consecutive non-breaching
    samples before on_clear fires.

    This runs rules sequentially in one thread by design: it is meant
    for a handful of rules on a slow-moving industrial process (seconds
    between readings, not microseconds), not a high-frequency
    multiplexed data feed. For dozens of independent high-frequency
    watches, run separate AlertEngine instances (or Client.watch()
    loops) in separate threads/processes instead of adding concurrency
    complexity to this class.
    """

    def __init__(self, client: Client):
        self.client = client
        self._rules: list[Rule] = []

    def add_rule(self, rule: Rule) -> "AlertEngine":
        self._rules.append(rule)
        return self

    def threshold(
        self,
        spec: str,
        *,
        above: float | None = None,
        below: float | None = None,
        on_trigger: Callable[[Value], None],
        on_clear: Callable[[Value], None] | None = None,
        debounce: int = 3,
        name: str = "",
    ) -> "AlertEngine":
        """Convenience: build and add a Threshold rule in one call."""
        rule = Threshold(spec=spec, above=above, below=below, debounce=debounce, name=name).as_rule(
            on_trigger=on_trigger, on_clear=on_clear
        )
        return self.add_rule(rule)

    def poll_once(self) -> None:
        """Read every rule's spec once and evaluate it. Useful for
        testing a rule set, or for driving the engine from your own
        scheduler instead of run()'s built-in loop."""
        for rule in self._rules:
            try:
                value = self.client.read(rule.spec)
            except Exception:
                logger.exception("otcat alerting: read failed for rule %r", rule.name)
                continue
            self._evaluate(rule, value)

    def _evaluate(self, rule: Rule, value: Value) -> None:
        breached = False
        try:
            breached = rule.predicate(value.value)
        except Exception:
            logger.exception("otcat alerting: predicate failed for rule %r", rule.name)
            return

        if breached:
            rule._consecutive += 1
            if not rule._active and rule._consecutive >= rule.debounce:
                rule._active = True
                rule.on_trigger(value)
        else:
            rule._consecutive = 0
            if rule._active:
                rule._active = False
                if rule.on_clear:
                    rule.on_clear(value)

    def run(self, *, interval_seconds: float = 1.0, iterations: int | None = None) -> None:
        """Blocking loop: poll_once() every interval_seconds. Run this
        in a background thread for a long-lived service; iterations
        (mainly for tests) bounds the loop instead of running forever."""
        i = 0
        while iterations is None or i < iterations:
            self.poll_once()
            i += 1
            if iterations is None or i < iterations:
                time.sleep(interval_seconds)
