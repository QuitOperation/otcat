package mock

import (
	"math"
	"sync"
	"time"
)

// TankProcess simulates one level-controlled tank: a PI controller
// driving an inlet valve against gravity-fed outflow through a fixed
// orifice. It exists so the benchmark and figures in the accompanying
// paper are generated from an actual (if small) control loop rather
// than synthetic noise — this is a simulation, not a capture from real
// plant equipment, and is labelled as such everywhere it is used.
//
// Physics: outflow follows Torricelli's law, Q_out = k * sqrt(2 g h),
// simplified here to Q_out = k' * sqrt(h) with the discharge
// coefficient and 2g folded into k'. Level integrates the net flow over
// tank cross-section A:
//
//	dh/dt = (Q_in(t) - Q_out(t)) / A
//	Q_out(t) = k' * sqrt(max(h, 0))
//	Q_in(t)  = valve(t) * Q_max,  valve(t) = clamp(Kp*e + Ki*∫e dt, 0, 1)
//	e(t) = setpoint - h(t)
//
// Integrated with forward Euler at a fixed dt — the discretization
// itself is the one place this loop is intentionally minimalist: for a
// dt of 100 ms against a tank time constant of tens of seconds, Euler's
// local error is negligible and a stiffer integrator would add
// complexity the demo does not need.
type TankProcess struct {
	mu sync.Mutex

	area      float64 // m^2
	outletK   float64 // discharge coefficient, folded with 2g
	maxInflow float64 // m^3/s at a fully open valve
	fullScale float64 // m, level that reads as 100%
	level     float64 // m

	setpoint float64
	kp, ki   float64
	integral float64

	dt time.Duration
}

func NewTankProcess() *TankProcess {
	return &TankProcess{
		area:      2.0,
		outletK:   0.05,
		maxInflow: 0.05,
		fullScale: 2.5,
		level:     0.5,
		setpoint:  1.5,
		kp:        0.9,
		ki:        0.08,
		dt:        100 * time.Millisecond,
	}
}

func (p *TankProcess) SetSetpoint(meters float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.setpoint = meters
}

// Step advances the simulation by one dt and returns the new level in
// meters.
func (p *TankProcess) Step() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	dt := p.dt.Seconds()

	e := p.setpoint - p.level
	p.integral += e * dt
	valve := p.kp*e + p.ki*p.integral
	valve = math.Max(0, math.Min(1, valve))

	qIn := valve * p.maxInflow
	qOut := p.outletK * math.Sqrt(math.Max(0, p.level))
	p.level += (qIn - qOut) / p.area * dt
	if p.level < 0 {
		p.level = 0
	}
	return p.level
}

// PercentX100 returns the level as a percentage of full scale, fixed
// at two implied decimal places (1523 means 15.23%) — the encoding a
// real PLC commonly uses to carry a fractional analog value in a plain
// uint16 holding register without a float.
func (p *TankProcess) PercentX100() uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()
	pct := p.level / p.fullScale * 100.0
	v := math.Round(pct * 100)
	if v < 0 {
		v = 0
	}
	if v > 65535 {
		v = 65535
	}
	return uint16(v)
}

// Drive steps the simulation on its own ticker and writes the result
// into holding register addr until stop is closed. Run as a goroutine.
func (p *TankProcess) Drive(s *Server, addr uint16, stop <-chan struct{}) {
	t := time.NewTicker(p.dt)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			p.Step()
			s.SetHolding(addr, p.PercentX100())
		}
	}
}
