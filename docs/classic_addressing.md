# Classic addressing: the off-by-one that outlived the company that made it

Every otcat address spec looks like `holding:40001`. The `holding:` part
is unambiguous. The `40001` part is a small piece of living history, and
getting it wrong is the single most common mistake in Modbus tooling —
so it is worth explaining exactly what otcat does with it, and why.

## Where "40001" comes from

Modicon's original PLC documentation (1979) needed a compact way to
refer to a location in one of four separate memory areas without
writing "holding register, offset 0" on every wiring diagram. The
convention that stuck was a five-digit *reference number* whose leading
digit names the table:

| leading digit | table            | range         |
|---------------|------------------|---------------|
| `0`            | coil             | 00001–09999   |
| `1`            | discrete input   | 10001–19999   |
| `3`            | input register   | 30001–39999   |
| `4`            | holding register | 40001–49999   |

Two details trip people up every time:

1. **The leading `0` for coils is almost always dropped in practice.**
   Nobody writes "coil 00001"; they write "coil 1". otcat follows that
   convention: `coil:1` means the first coil, not `coil:0`.
2. **The reference number is 1-indexed; the wire offset is 0-indexed.**
   `40001` is not holding register "40001" — it is holding register
   offset `0`, in a table that happens to be *named* with a leading 4.
   Reference number *N* in table *T* maps to wire offset `N - base(T)`.

## What otcat actually does

`ParseSpec` (`internal/modbus/address.go`) checks whether the numeral in
an address spec falls inside its table's classic five-digit band
(`base(T) .. base(T)+9998`). If it does, otcat subtracts the base and
uses the result as the wire offset. If it does not — because it is
smaller (someone really did mean raw offset 42) or larger (someone
addressed past the classic 9999-register ceiling, which the wire
protocol allows even though the classic notation cannot express it) —
otcat uses the numeral as a literal offset, unchanged.

This dual reading is a deliberate default, not an accident: it makes
the exact examples every Modbus reference manual, wiring diagram, and
PLC vendor's documentation already uses ("40001", "30012", "coil 5")
work with zero translation. `--raw-address` turns it off entirely for
the (rarer) case of a genuinely literal offset that happens to collide
with the classic band — e.g., a device whose documentation already
gives 0-based offsets and separately mentions holding registers, where
applying the classic translation a second time would silently be wrong.

## Why not guess harder

An alternative design would try to be "smarter" — e.g., assume any
number under some threshold is raw and anything larger is classic. That
just moves the ambiguity to a different, less-documented threshold.
otcat's rule has exactly one edge, it is the actual boundary of the
convention it is reproducing (the classic band's own width), and it is
the same rule for every table. A predictable rule that is wrong in a
rare, flag-escapable case beats a clever rule that is wrong in a way
nobody can predict without reading the source.
