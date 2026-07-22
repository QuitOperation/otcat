package cliapp

import (
	"flag"
	"fmt"
	"io"
	"time"
)

// config is the fully parsed, fully validated command line. Nothing
// downstream of Parse re-touches os.Args or the flag package; every
// other function in cliapp takes a *config and pure Go values.
type config struct {
	// endpoint selection — exactly one of these four is non-empty
	modbusAddr string
	eipAddr    string
	s7Addr     string
	bacnetAddr string

	// action — exactly one of these three is set
	readSpec  string
	watchSpec string
	writeSpec string

	// write payload
	value     string
	fromStdin bool

	// safety
	confirm bool
	dryRun  bool

	// output
	format string // "json" | "csv" | "raw"

	// modbus-specific interpretation (harmless no-ops for other drivers)
	unitID     uint
	dataType   string
	byteOrder  string
	wordOrder  string
	rawAddress bool

	// timing
	timeout    time.Duration
	interval   time.Duration
	maxBackoff time.Duration
	count      uint

	showVersion bool
	showHelp    bool
}

const usageHeader = `otcat — the netcat for industrial I/O

USAGE
  otcat --modbus HOST:PORT --read TABLE:ADDR[:COUNT] [flags]
  otcat --modbus HOST:PORT --watch TABLE:ADDR[:COUNT] [flags]
  otcat --modbus HOST:PORT --write TABLE:ADDR --value LITERAL --confirm [flags]
  cat values.txt | otcat --modbus HOST:PORT --write TABLE:ADDR --from-stdin --confirm

ENDPOINT (pick one)
  --modbus HOST:PORT     Modbus TCP (fully implemented)
  --eip HOST[:PORT]      EtherNet/IP / CIP     (driver stub, not yet implemented)
  --s7comm HOST[:PORT]   Siemens S7comm         (driver stub, not yet implemented)
  --bacnet HOST[:PORT]   BACnet/IP              (driver stub, not yet implemented)

ACTION (pick one)
  --read SPEC            read once, print one Value, exit
  --watch SPEC           read on a fixed interval until interrupted or --count is reached
  --write SPEC           write --value (or one line per write with --from-stdin)

ADDRESS SPEC (Modbus)
  table:address[:count]  table is one of coil|discrete|holding|input
                          address is a classic 5-digit reference (40001) or,
                          with --raw-address, a literal 0-based offset
                          count defaults to the width of --type (see below)

WRITE SAFETY
  --value LITERAL         decimal, 0x-hex, or float; comma-separated for multi-register writes
  --from-stdin             one literal per line from stdin, one write per line
  --confirm                required to actually perform a write; otherwise otcat refuses
  --dry-run                print what would be sent, touch no network, exit 0

OUTPUT
  --json (default) | --csv | --raw

MODBUS OPTIONS
  --type TYPE             uint16|int16|uint32|int32|float32 (default uint16)
  --byte-order ORDER      big|little (default big; §4.2 of the spec, rarely needed)
  --word-order ORDER      high|low   (default high; "low" = word-swapped 32-bit floats)
  --unit ID                Modbus unit/slave id (default 1)
  --raw-address             disable classic 40001-style address translation

TIMING
  --timeout DURATION       per-request deadline (default 5s)
  --interval DURATION      --watch poll period (default 1s)
  --count N                 stop --watch after N successful reads (default: unbounded)
  --max-backoff DURATION   --watch retry ceiling after failures (default 30s)

  --version   --help

EXAMPLES
  otcat --modbus 192.168.1.10:502 --read holding:40001 --json
  otcat --modbus 192.168.1.10:502 --watch holding:40001 --interval 500ms --raw | awk '{print $1*0.1}'
  otcat --modbus 192.168.1.10:502 --write holding:40001 --value 100 --confirm
  otcat --modbus 192.168.1.10:502 --read holding:40001:2 --type float32 --word-order low
`

func parseArgs(args []string, stderr io.Writer) (*config, error) {
	fs := flag.NewFlagSet("otcat", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usageHeader) }

	c := &config{}
	fs.StringVar(&c.modbusAddr, "modbus", "", "Modbus TCP endpoint HOST:PORT")
	fs.StringVar(&c.eipAddr, "eip", "", "EtherNet/IP endpoint (stub)")
	fs.StringVar(&c.s7Addr, "s7comm", "", "S7comm endpoint (stub)")
	fs.StringVar(&c.bacnetAddr, "bacnet", "", "BACnet/IP endpoint (stub)")

	fs.StringVar(&c.readSpec, "read", "", "read once: TABLE:ADDR[:COUNT]")
	fs.StringVar(&c.watchSpec, "watch", "", "read repeatedly: TABLE:ADDR[:COUNT]")
	fs.StringVar(&c.writeSpec, "write", "", "write: TABLE:ADDR")

	fs.StringVar(&c.value, "value", "", "literal value to write")
	fs.BoolVar(&c.fromStdin, "from-stdin", false, "read write values from stdin, one per line")

	fs.BoolVar(&c.confirm, "confirm", false, "required to actually perform a write")
	fs.BoolVar(&c.dryRun, "dry-run", false, "print what a write would do, without connecting")

	jsonFlag := fs.Bool("json", false, "output ndjson (default)")
	csvFlag := fs.Bool("csv", false, "output CSV")
	rawFlag := fs.Bool("raw", false, "output bare scalar values")

	fs.UintVar(&c.unitID, "unit", 1, "Modbus unit/slave id")
	fs.StringVar(&c.dataType, "type", "uint16", "uint16|int16|uint32|int32|float32")
	fs.StringVar(&c.byteOrder, "byte-order", "big", "big|little")
	fs.StringVar(&c.wordOrder, "word-order", "high", "high|low")
	fs.BoolVar(&c.rawAddress, "raw-address", false, "disable classic 40001-style address translation")

	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "per-request timeout")
	fs.DurationVar(&c.interval, "interval", time.Second, "--watch poll interval")
	fs.DurationVar(&c.maxBackoff, "max-backoff", 30*time.Second, "--watch retry ceiling")
	fs.UintVar(&c.count, "count", 0, "stop --watch after N reads (0 = unbounded)")

	fs.BoolVar(&c.showVersion, "version", false, "print version and exit")
	fs.BoolVar(&c.showHelp, "help", false, "print usage and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	switch {
	case *csvFlag && *rawFlag:
		return nil, fmt.Errorf("--csv and --raw are mutually exclusive")
	case *csvFlag:
		c.format = "csv"
	case *rawFlag:
		c.format = "raw"
	default:
		c.format = "json" // also covers an explicit --json, which is a no-op by design
	}
	_ = jsonFlag
	return c, nil
}
