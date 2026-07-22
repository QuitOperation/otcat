package cliapp

import (
	"github.com/QuitOperation/otcat/internal/bacnet"
	"github.com/QuitOperation/otcat/internal/eip"
	"github.com/QuitOperation/otcat/internal/protocol"
	"github.com/QuitOperation/otcat/internal/s7"
)

// newStubDriver returns the registered (but not wire-implemented)
// driver for name. Each constructor already satisfies protocol.Driver
// and fails every call with protocol.ErrNotImplemented — see each
// package's doc comment for why.
func newStubDriver(name, addr string) protocol.Driver {
	switch name {
	case "eip":
		return eip.NewDriver(addr)
	case "s7":
		return s7.NewDriver(addr)
	case "bacnet":
		return bacnet.NewDriver(addr)
	default:
		panic("otcat: unknown stub driver " + name) // unreachable: caller controls name
	}
}
