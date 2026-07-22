// Command otcat streams industrial register and tag values to stdout,
// and writes stdin to industrial registers and tags — the netcat model,
// applied to OT. See internal/cliapp for everything past flag parsing.
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
