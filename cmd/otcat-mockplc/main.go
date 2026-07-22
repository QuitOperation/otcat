// Command otcat-mockplc runs the same in-memory Modbus TCP server the
// test suite and benchmarks use, as a standalone process, so a person
// evaluating otcat can point it at something real over a real socket
// without owning a PLC. It seeds a small, documented register map and
// drives one holding register from the physically-modeled TankProcess
// so --watch has something genuinely dynamic to show.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/QuitOperation/otcat/internal/mock"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:15020", "listen address (not 502: that port needs root on most systems)")
	flag.Parse()

	s, err := mock.NewAt(*addr)
	if err != nil {
		log.Fatalf("otcat-mockplc: %v", err)
	}
	defer s.Close()

	// holding:0     — tank level, % x100, live via TankProcess
	// holding:100   — static uint16 demo value (0x1234 = 4660)
	// holding:200:2 — static float32 demo value (3.14159, ABCD word order)
	// coil:0        — static demo coil, ON
	s.SetHolding(100, 0x1234)
	s.SetHoldingRange(200, []uint16{0x4049, 0x0FDB}) // float32(3.14159) big-endian word/byte order
	s.SetCoil(0, true)

	proc := mock.NewTankProcess()
	stop := make(chan struct{})
	go proc.Drive(s, 0, stop)

	fmt.Printf("otcat-mockplc: listening on %s (Modbus TCP, unit id any)\n", s.Addr())
	fmt.Println("otcat-mockplc: holding:0 = simulated tank level %, x100 fixed point")
	fmt.Println("otcat-mockplc: holding:100 = 0x1234 constant   holding:200:2 = float32 3.14159   coil:0 = true")
	fmt.Printf("otcat-mockplc: try: otcat --modbus %s --watch holding:0 --raw-address --interval 500ms\n", s.Addr())

	ctx, stopSig := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSig()
	<-ctx.Done()
	close(stop)
	fmt.Println("otcat-mockplc: shutting down")
}
