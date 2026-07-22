// Package eip is the reserved slot for an EtherNet/IP (CIP) driver.
//
// EtherNet/IP layers CIP (Common Industrial Protocol) explicit messaging
// over an encapsulation protocol (ODVA "EtherNet/IP Specification",
// Volume 2). Unlike Modbus's fixed 16-bit register model, CIP addresses
// are symbolic tags resolved through a class/instance/attribute service
// request, with per-vendor quirks in how structured tags (Rockwell
// UDTs in particular) serialize. A correct client needs, at minimum:
// a Register Session / UnRegister Session encapsulation handshake, the
// Unconnected Send (0x52) and Forward Open connected-messaging paths,
// CIP Get_Attribute_Single / Get_Attribute_List services, and tag-name
// symbolic segment encoding. Each of those is independently easy to get
// subtly wrong in a way that compiles, runs, and returns a plausible
// but incorrect value against real hardware — which for a write path
// touching a live controller is worse than refusing outright.
//
// This package exists so the driver surface (registration, --help,
// flag parsing, error taxonomy) is real and stable now; the wire
// protocol is deliberately not implemented in this build. See
// docs/driver_roadmap.md.
package eip

import (
	"context"

	"github.com/QuitOperation/otcat/internal/protocol"
)

type Driver struct {
	addr string
}

func NewDriver(addr string) *Driver { return &Driver{addr: addr} }

func (d *Driver) Name() string { return "eip" }

func (d *Driver) Connect(ctx context.Context) error { return protocol.ErrNotImplemented }
func (d *Driver) Close() error                      { return nil }

func (d *Driver) Read(ctx context.Context, spec string) (protocol.Value, error) {
	return protocol.Value{}, protocol.ErrNotImplemented
}

func (d *Driver) Write(ctx context.Context, spec string, literal string) error {
	return protocol.ErrNotImplemented
}

var _ protocol.Driver = (*Driver)(nil)
