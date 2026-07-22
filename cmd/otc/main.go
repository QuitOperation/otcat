// Command otc is otcat under its short name — the same relationship
// "nc" has to "netcat". It is not a different tool: same flags, same
// behavior, same exit codes, just less typing for the case this
// project is built around, a command someone reaches for constantly
// in a terminal. See cmd/otcat and internal/cliapp for everything
// else; this file is deliberately a byte-for-byte twin of
// cmd/otcat/main.go, kept in its own package only because `go
// install .../cmd/otc@latest` is how Go names the resulting binary.
package main

import (
	"os"

	"github.com/QuitOperation/otcat/internal/cliapp"
)

// version and commit are overwritten at build time:
//
//	go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git rev-parse --short HEAD)"
var (
	version = "dev"
	commit  = "none"
)

func main() {
	cliapp.Version = version + " (" + commit + ")"
	os.Exit(cliapp.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
