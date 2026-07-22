// Package bacnet is the reserved slot for a BACnet/IP driver.
//
// BACnet (ASHRAE 135) is UDP-native, object-oriented (object-type,
// instance, property, not a flat register), and its read/write services
// are carried inside APDUs built from a tag-length-value encoding
// (application vs. context tags, opening/closing tags for constructed
// data) that is a materially bigger parser than Modbus's fixed 5-byte
// request PDU. It also normally requires Who-Is/I-Am discovery before
// an address is even resolvable to a device. That is a full protocol
// stack, not a client wrapper, and out of scope for this build.
//
// The driver surface below is real and wired into the CLI now; the
// wire protocol is not. See docs/driver_roadmap.md.
package bacnet

import (
	"context"

	"github.com/QuitOperation/otcat/internal/protocol"
)

type Driver struct {
	addr string
}

func NewDriver(addr string) *Driver { return &Driver{addr: addr} }

func (d *Driver) Name() string { return "bacnet" }

func (d *Driver) Connect(ctx context.Context) error { return protocol.ErrNotImplemented }
func (d *Driver) Close() error                      { return nil }

func (d *Driver) Read(ctx context.Context, spec string) (protocol.Value, error) {
	return protocol.Value{}, protocol.ErrNotImplemented
}

func (d *Driver) Write(ctx context.Context, spec string, literal string) error {
	return protocol.ErrNotImplemented
}

var _ protocol.Driver = (*Driver)(nil)
