// Package s7 is the reserved slot for a Siemens S7comm driver.
//
// S7comm rides inside COTP (ISO-on-TCP, RFC 1006) inside a TPKT framing
// layer, and Siemens has never published it — every existing
// implementation (snap7, python-snap7, Sharp7) is derived from packet
// capture and reverse engineering, not a specification document. Getting
// the COTP connection-request parameters and S7 PDU negotiation wrong
// tends to fail loudly (no data); getting the item-address encoding
// wrong for a DB write (area code, DB number, byte/bit offset, transport
// size) can fail quietly and write to the wrong memory cell on a live
// controller. That is a correctness bar this build does not clear, so
// it does not ship a write path — or a read path — for S7comm.
//
// The driver surface below is real and wired into the CLI now so that
// adding the wire implementation later is additive, not a rearchitect.
// See docs/driver_roadmap.md.
package s7

import (
	"context"

	"github.com/QuitOperation/otcat/internal/protocol"
)

type Driver struct {
	addr string
}

func NewDriver(addr string) *Driver { return &Driver{addr: addr} }

func (d *Driver) Name() string { return "s7" }

func (d *Driver) Connect(ctx context.Context) error { return protocol.ErrNotImplemented }
func (d *Driver) Close() error                      { return nil }

func (d *Driver) Read(ctx context.Context, spec string) (protocol.Value, error) {
	return protocol.Value{}, protocol.ErrNotImplemented
}

func (d *Driver) Write(ctx context.Context, spec string, literal string) error {
	return protocol.ErrNotImplemented
}

var _ protocol.Driver = (*Driver)(nil)
