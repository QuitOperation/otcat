// Command otcat-latencyprobe answers the question every commissioning
// engineer eventually asks: "how much of my scan-cycle budget does one
// otcat read actually cost against this device?" It performs N
// sequential ReadHoldingRegisters calls and reports the full latency
// distribution, not just a mean — means hide exactly the tail latency
// that matters when otcat is driving a control loop's timing budget.
//
// With no --modbus given, it starts the same in-memory mock server the
// test suite uses, so `otcat-latencyprobe` alone produces a meaningful
// (if optimistic — loopback has no real network) baseline.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/QuitOperation/otcat/internal/mock"
	"github.com/QuitOperation/otcat/internal/modbus"
)

type result struct {
	Samples          int     `json:"samples"`
	Quantity         int     `json:"registers_per_read"`
	MinUs            float64 `json:"min_us"`
	P50Us            float64 `json:"p50_us"`
	P90Us            float64 `json:"p90_us"`
	P99Us            float64 `json:"p99_us"`
	P999Us           float64 `json:"p999_us"`
	MaxUs            float64 `json:"max_us"`
	MeanUs           float64 `json:"mean_us"`
	StdDevUs         float64 `json:"stddev_us"`
	ThroughputPerSec float64 `json:"throughput_reads_per_sec"`
}

func main() {
	addr := flag.String("modbus", "", "Modbus TCP endpoint; if empty, an internal mock server is used")
	n := flag.Int("n", 2000, "number of sequential reads to sample")
	quantity := flag.Int("registers", 10, "holding registers per read")
	warmup := flag.Int("warmup", 100, "untimed warm-up reads before sampling (lets TCP slow-start and connection setup settle)")
	asJSON := flag.Bool("json", false, "print result as JSON instead of a text table")
	samplesOut := flag.String("samples-out", "", "if set, write one raw latency sample (microseconds) per line to this file")
	flag.Parse()

	target := *addr
	if target == "" {
		s, err := mock.New()
		if err != nil {
			fmt.Fprintln(os.Stderr, "otcat-latencyprobe:", err)
			os.Exit(1)
		}
		defer s.Close()
		target = s.Addr()
		fmt.Fprintf(os.Stderr, "otcat-latencyprobe: no --modbus given, using built-in mock at %s (loopback baseline only)\n", target)
	}

	c := modbus.NewClient(target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "otcat-latencyprobe: connect:", err)
		os.Exit(1)
	}
	defer c.Close()

	for i := 0; i < *warmup; i++ {
		if _, err := c.ReadHoldingRegisters(context.Background(), 0, *quantity); err != nil {
			fmt.Fprintln(os.Stderr, "otcat-latencyprobe: warm-up read:", err)
			os.Exit(1)
		}
	}

	samples := make([]float64, *n)
	start := time.Now()
	for i := 0; i < *n; i++ {
		t0 := time.Now()
		if _, err := c.ReadHoldingRegisters(context.Background(), 0, *quantity); err != nil {
			fmt.Fprintln(os.Stderr, "otcat-latencyprobe: read:", err)
			os.Exit(1)
		}
		samples[i] = float64(time.Since(t0).Microseconds())
	}
	wall := time.Since(start)

	if *samplesOut != "" {
		f, err := os.Create(*samplesOut)
		if err != nil {
			fmt.Fprintln(os.Stderr, "otcat-latencyprobe: samples-out:", err)
			os.Exit(1)
		}
		for _, v := range samples {
			fmt.Fprintf(f, "%.0f\n", v)
		}
		f.Close()
	}

	sort.Float64s(samples)
	res := result{
		Samples:          *n,
		Quantity:         *quantity,
		MinUs:            samples[0],
		P50Us:            percentile(samples, 0.50),
		P90Us:            percentile(samples, 0.90),
		P99Us:            percentile(samples, 0.99),
		P999Us:           percentile(samples, 0.999),
		MaxUs:            samples[len(samples)-1],
		MeanUs:           mean(samples),
		StdDevUs:         stddev(samples),
		ThroughputPerSec: float64(*n) / wall.Seconds(),
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return
	}
	fmt.Printf("otcat-latencyprobe: %d reads x %d holding registers against %s\n", *n, *quantity, target)
	fmt.Printf("  min    %8.1f us\n", res.MinUs)
	fmt.Printf("  p50    %8.1f us\n", res.P50Us)
	fmt.Printf("  p90    %8.1f us\n", res.P90Us)
	fmt.Printf("  p99    %8.1f us\n", res.P99Us)
	fmt.Printf("  p99.9  %8.1f us\n", res.P999Us)
	fmt.Printf("  max    %8.1f us\n", res.MaxUs)
	fmt.Printf("  mean   %8.1f us (stddev %.1f us)\n", res.MeanUs, res.StdDevUs)
	fmt.Printf("  throughput: %.0f reads/sec (sequential, single connection)\n", res.ThroughputPerSec)
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func mean(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func stddev(xs []float64) float64 {
	m := mean(xs)
	var sumSq float64
	for _, x := range xs {
		d := x - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(xs)))
}
