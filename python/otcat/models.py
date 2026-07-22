"""Typed models mirroring the JSON otcat's Go core actually emits
(``internal/protocol.Value`` and ``internal/protocol.WritePlan``).
Nothing here is reimplemented protocol logic -- it's just a typed
Python view of exactly the bytes the Go binary printed.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any


@dataclass(frozen=True, slots=True)
class Value:
    """One reading. Field-for-field the same shape as the ndjson
    otcat --json prints, so ``Value(**json.loads(line))`` (after
    renaming ``ts``) is a valid way to think about this type even
    though the real parsing goes through :func:`from_json`."""

    address: str
    type: str
    value: Any
    quality: str
    ts: datetime
    raw: list[int] | None = None

    @property
    def good(self) -> bool:
        return self.quality == "good"

    @classmethod
    def from_json(cls, obj: dict) -> "Value":
        ts_raw = obj["ts"]
        # Go's RFC3339Nano sometimes omits the fractional seconds
        # entirely (whole-second timestamps); Python's fromisoformat
        # (3.11+) handles the 'Z' suffix natively, but we support 3.9+
        # so normalize it ourselves.
        ts_norm = ts_raw.replace("Z", "+00:00")
        try:
            ts = datetime.fromisoformat(ts_norm)
        except ValueError:
            ts = datetime.now(timezone.utc)
        return cls(
            address=obj["address"],
            type=obj["type"],
            value=obj["value"],
            quality=obj["quality"],
            ts=ts,
            raw=obj.get("raw"),
        )


@dataclass(frozen=True, slots=True)
class WritePlan:
    """The output of Client.dry_run(): exactly what a write would
    send, computed with zero network I/O (mirrors
    internal/protocol.WritePlan)."""

    driver: str
    address: str
    literal: str
    type: str | None = None
    registers: list[int] = field(default_factory=list)
    coils: list[bool] = field(default_factory=list)

    @classmethod
    def from_json(cls, obj: dict) -> "WritePlan":
        return cls(
            driver=obj["driver"],
            address=obj["address"],
            literal=obj["literal"],
            type=obj.get("type"),
            registers=obj.get("registers", []) or [],
            coils=obj.get("coils", []) or [],
        )
