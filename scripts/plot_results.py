#!/usr/bin/env python3
"""Generate paper figures from real otcat benchmark/demo output.

Every input file here was produced by actually running otcat, its mock
PLC, or its latency probe (see benchmarks/README.md for the exact
commands) — nothing in this script fabricates a data point.
"""
import json
import datetime as dt
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker

ROOT = Path(__file__).resolve().parent.parent
BENCH = ROOT / "benchmarks"
FIG = ROOT / "paper" / "figures"
FIG.mkdir(parents=True, exist_ok=True)

# IEEE-friendly styling: serif fonts, restrained palette, thin lines.
plt.rcParams.update({
    "font.family": "serif",
    "font.size": 9,
    "axes.titlesize": 9,
    "axes.labelsize": 9,
    "xtick.labelsize": 8,
    "ytick.labelsize": 8,
    "legend.fontsize": 8,
    "axes.linewidth": 0.6,
    "grid.linewidth": 0.4,
    "lines.linewidth": 1.1,
    "figure.dpi": 300,
})

INK = "#1a1a1a"
ACCENT = "#a6192e"   # single restrained accent (deep red), used sparingly
GRID = "#d9d9d9"


# --- Figure 1: tank-level watch stream, real 30 s capture -----------------
def fig_tank_watch():
    rows = []
    with open(BENCH / "tank_watch.ndjson") as f:
        for line in f:
            d = json.loads(line)
            rows.append(d)
    t0 = dt.datetime.fromisoformat(rows[0]["ts"].replace("Z", "+00:00"))
    xs = [(dt.datetime.fromisoformat(r["ts"].replace("Z", "+00:00")) - t0).total_seconds() for r in rows]
    ys = [r["value"] / 100.0 for r in rows]  # x100 fixed point -> percent

    fig, ax = plt.subplots(figsize=(3.4, 2.1))
    ax.plot(xs, ys, color=INK, linewidth=1.0)
    ax.axhline(60.0, color=ACCENT, linewidth=0.8, linestyle="--", label="setpoint (60.0%)")
    ax.set_xlabel("time since first sample (s)")
    ax.set_ylabel("tank level (%)")
    ax.set_title("holding:0 via otcat --watch --interval 100ms", loc="left", fontsize=8, style="italic")
    ax.grid(True, color=GRID, linewidth=0.4)
    ax.legend(frameon=False, loc="lower right")
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)
    fig.tight_layout(pad=0.3)
    fig.savefig(FIG / "tank_watch.pdf")
    plt.close(fig)


# --- Figure 2: latency distribution, real 10k-sample capture --------------
def fig_latency_hist():
    samples = [float(x) for x in open(BENCH / "latency_samples_us.txt")]
    summary = json.load(open(BENCH / "latency_loopback.json"))

    fig, ax = plt.subplots(figsize=(3.4, 2.1))
    # Clip the display range at p99.9 so the rare high-latency tail
    # (GC pauses, OS scheduling) doesn't crush the main histogram body;
    # the tail is reported numerically instead, not hidden.
    clip = summary["p999_us"] * 1.5
    shown = [s for s in samples if s <= clip]
    ax.hist(shown, bins=60, color=INK, alpha=0.85, linewidth=0)
    for label, key, style in [("p50", "p50_us", ":"), ("p99", "p99_us", "--")]:
        ax.axvline(summary[key], color=ACCENT, linestyle=style, linewidth=0.9)
        ax.text(summary[key], ax.get_ylim()[1] * 0.92, label, color=ACCENT, fontsize=7,
                ha="left", va="top", rotation=90)
    ax.set_xlabel("read latency, 10 holding registers, loopback (µs)")
    ax.set_ylabel("count")
    ax.set_title(f"n={summary['samples']}, mean={summary['mean_us']:.1f}µs, "
                 f"p99.9={summary['p999_us']:.0f}µs", loc="left", fontsize=8, style="italic")
    ax.grid(True, color=GRID, linewidth=0.4, axis="y")
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)
    fig.tight_layout(pad=0.3)
    fig.savefig(FIG / "latency_hist.pdf")
    plt.close(fig)


# --- Figure 3: per-operation cost breakdown, real benchmark means ---------
def fig_op_costs():
    summary = json.load(open(BENCH / "summary.json"))
    by_name = {r["name"]: r for r in summary}

    labels_order = [
        ("BenchmarkEncodeReadRequest", "encode request"),
        ("BenchmarkDecodeReadRegistersResponse", "decode 10 regs"),
        ("BenchmarkCodecDecodeFloat32", "decode float32"),
        ("BenchmarkParseSpecClassic", "parse address"),
        ("BenchmarkJSONEncode", "encode ndjson"),
    ]
    names = [lbl for _, lbl in labels_order]
    values = [by_name[k]["ns_op"] for k, _ in labels_order]

    fig, ax = plt.subplots(figsize=(3.4, 2.1))
    y = range(len(names))
    ax.barh(list(y), values, color=INK, height=0.55)
    ax.set_yticks(list(y))
    ax.set_yticklabels(names)
    ax.invert_yaxis()
    ax.set_xscale("log")
    ax.set_xlabel("time (ns/op, log scale)")
    ax.set_title("pure CPU cost per stage (no I/O)", loc="left", fontsize=8, style="italic")
    for yi, v in zip(y, values):
        ax.text(v * 1.15, yi, f"{v:,.0f} ns", va="center", fontsize=7)
    ax.grid(True, color=GRID, linewidth=0.4, axis="x", which="both")
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)
    fig.tight_layout(pad=0.3)
    fig.savefig(FIG / "op_costs.pdf")
    plt.close(fig)


# --- Figure 4: throughput vs. register count, real round-trip benchmark ---
def fig_throughput():
    summary = json.load(open(BENCH / "summary.json"))
    by_name = {r["name"]: r for r in summary}
    counts = [1, 10, 125]
    keys = [f"BenchmarkClientReadHoldingRegisters{c}" for c in counts]
    ns = [by_name[k]["ns_op"] for k in keys]
    throughput = [1e9 / v for v in ns]

    fig, ax = plt.subplots(figsize=(3.4, 2.1))
    ax.plot(counts, throughput, color=INK, marker="o", markersize=3.5)
    ax.set_xlabel("registers per read")
    ax.set_ylabel("reads/sec (single connection, loopback)")
    ax.set_xticks(counts)
    ax.yaxis.set_major_formatter(mticker.FuncFormatter(lambda v, _: f"{v/1000:.0f}k"))
    ax.set_title("round-trip throughput vs. read width", loc="left", fontsize=8, style="italic")
    ax.grid(True, color=GRID, linewidth=0.4)
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)
    fig.tight_layout(pad=0.3)
    fig.savefig(FIG / "throughput.pdf")
    plt.close(fig)


if __name__ == "__main__":
    fig_tank_watch()
    fig_latency_hist()
    fig_op_costs()
    fig_throughput()
    print("wrote figures to", FIG)
