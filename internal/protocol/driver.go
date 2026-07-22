// Package protocol defines the one contract every industrial backend must
// satisfy to plug into otcat. otcat's job is streams and bytes; a Driver's
// only job is turning a protocol-specific address into a Value, and a
// Value back into wire bytes. Nothing else crosses this boundary.
//
// Deliberately absent: no shared "universal address" struct across
// protocols. Modbus addresses a 16-bit register table. CIP (EtherNet/IP)
// addresses symbolic tags. S7comm addresses a DB/offset/bit triple.
// BACnet addresses an (object-type, instance, property) triple. Forcing
// those into one struct produces a leaky lowest-common-denominator that
// fits none of them well. Each Driver owns its address grammar and is
// free to define it precisely.
package protocol

import (
	"context"
	"errors"
	"time"
)

// Quality mirrors the OPC-style good/bad/stale tri-state every SCADA
// historian already uses, so downstream tools (grep, jq, a historian
// ingester) can filter on it without learning otcat-specific vocabulary.
type Quality uint8

const (
	QualityGood Quality = iota
	QualityBad
	QualityStale
)

func (q Quality) String() string {
	switch q {
	case QualityGood:
		return "good"
	case QualityBad:
		return "bad"
	case QualityStale:
		return "stale"
	default:
		return "unknown"
	}
}

func (q Quality) MarshalJSON() ([]byte, error) {
	return []byte(`"` + q.String() + `"`), nil
}

// Value is one self-describing datum. It is the unit that crosses stdout,
// one per line, one per JSON object: the "packet" of this netcat.
type Value struct {
	Address   string      `json:"address"`
	Type      string      `json:"type"`
	Value     interface{} `json:"value"`
	Quality   Quality     `json:"quality"`
	Timestamp time.Time   `json:"ts"`
	// Raw holds the untyped wire words backing Value, present only when
	// -raw is requested. Kept off by default: most pipelines want the
	// typed scalar, not its register-level anatomy.
	Raw []uint16 `json:"raw,omitempty"`
}

// Driver is implemented once per industrial protocol.
type Driver interface {
	// Name is the short protocol identifier used in logs and errors,
	// e.g. "modbus", "eip", "s7", "bacnet".
	Name() string

	// Connect establishes the underlying session. Drivers are single-
	// connection, single-endpoint, single-owner: one otcat process talks
	// to one device. This matches how most PLCs behave under concurrent
	// masters (badly), and keeps the client free of connection-pool
	// complexity it does not need.
	Connect(ctx context.Context) error

	// Close releases the underlying session. Idempotent.
	Close() error

	// Read resolves one address spec into one Value. spec grammar is
	// driver-specific; see each driver's doc.go.
	Read(ctx context.Context, spec string) (Value, error)

	// Write encodes literal into the wire type implied by spec and
	// writes it. literal is decimal, hex (0x-prefixed), or a
	// comma-separated list for array/multi-register writes.
	Write(ctx context.Context, spec string, literal string) error
}

// WritePlan is what a DryRunner reports instead of performing a write:
// exactly the wire-level payload the write would have sent, so an
// operator (or a pre-flight CI check parsing a config file full of
// setpoints) can catch an encoding mistake — wrong type, wrong word
// order, an out-of-range literal — before it ever reaches a live
// controller.
type WritePlan struct {
	Driver    string   `json:"driver"`
	Address   string   `json:"address"`
	Literal   string   `json:"literal"`
	Type      string   `json:"type,omitempty"`
	Registers []uint16 `json:"registers,omitempty"`
	Coils     []bool   `json:"coils,omitempty"`
}

// DryRunner is implemented by drivers that can compute a WritePlan
// without a live connection. Not part of Driver itself: a driver that
// cannot yet speak its wire protocol (see internal/eip, internal/s7,
// internal/bacnet) still shouldn't claim to know what it would send.
type DryRunner interface {
	ExplainWrite(spec, literal string) (WritePlan, error)
}

// ErrNotImplemented is returned by driver stubs that are architecturally
// wired in (they satisfy Driver, they appear in --help, they fail loudly
// and immediately) but do not yet speak their wire protocol. otcat would
// rather refuse cleanly than emit a byte pattern that merely looks like
// EtherNet/IP, S7comm, or BACnet and lands on a real PLC.
var ErrNotImplemented = errors.New("otcat: driver not implemented in this build")

// DialTimeout is the default TCP handshake budget. Kept short and
// explicit: OT networks are typically flat, low-latency LANs, and a long
// default here only delays the "device unreachable" diagnosis operators
// actually want fast.
const DialTimeout = 3 * time.Second
