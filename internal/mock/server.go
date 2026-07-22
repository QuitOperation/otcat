// Package mock implements just enough of a Modbus TCP server to test
// and benchmark otcat's client against real bytes on a real socket,
// without requiring physical (or even virtual) PLC hardware. It is not
// a general-purpose simulator: it speaks exactly the eight function
// codes internal/modbus speaks, and no more.
package mock

import (
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	fcReadCoils            = 0x01
	fcReadDiscreteInputs   = 0x02
	fcReadHoldingRegisters = 0x03
	fcReadInputRegisters   = 0x04
	fcWriteSingleCoil      = 0x05
	fcWriteSingleRegister  = 0x06
	fcWriteMultipleCoils   = 0x0F
	fcWriteMultipleRegs    = 0x10
	exceptionBit           = 0x80

	exIllegalFunction  = 0x01
	exIllegalAddress   = 0x02
	exIllegalValue     = 0x03
	exServerDeviceFail = 0x04
)

// Server is a single-listener, in-memory Modbus TCP server: four flat
// register/bit tables sized to the full 16-bit address space, guarded
// by one mutex. That is a deliberately naive concurrency model — real
// PLCs do not pipeline internally either — chosen so the server's own
// behavior never becomes the bottleneck being measured in a benchmark.
type Server struct {
	mu       sync.Mutex
	coils    [65536]bool
	discrete [65536]bool
	holding  [65536]uint16
	input    [65536]uint16

	ln net.Listener

	// Fault injection, used by resilience tests (client_resilience_test.go).
	latency  time.Duration
	dropRate float64
	rng      *rand.Rand
	rngMu    sync.Mutex

	wg sync.WaitGroup
}

type Option func(*Server)

// WithLatency adds a fixed delay before every response is written,
// simulating a slow serial-to-TCP gateway.
func WithLatency(d time.Duration) Option { return func(s *Server) { s.latency = d } }

// WithDropRate closes the connection mid-transaction with probability p
// instead of responding, simulating a flaky network link.
func WithDropRate(p float64) Option { return func(s *Server) { s.dropRate = p } }

// New starts a server listening on 127.0.0.1 with an OS-assigned port
// and returns immediately; call Addr for the dialable address and
// Close to shut down.
func New(opts ...Option) (*Server, error) {
	return NewAt("127.0.0.1:0", opts...)
}

// NewAt is New with an explicit listen address, for callers (notably
// cmd/otcat-mockplc) that want a fixed, memorable port instead of an
// OS-assigned one.
func NewAt(addr string, opts ...Option) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln, rng: rand.New(rand.NewSource(1))}
	for _, o := range opts {
		o(s)
	}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

func (s *Server) Addr() string { return s.ln.Addr().String() }

func (s *Server) Close() error {
	err := s.ln.Close()
	s.wg.Wait()
	return err
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		s.wg.Add(1)
		go s.handle(conn)
	}
}

func (s *Server) shouldDrop() bool {
	if s.dropRate <= 0 {
		return false
	}
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Float64() < s.dropRate
}

func (s *Server) handle(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	for {
		header := make([]byte, 7)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		txID := binary.BigEndian.Uint16(header[0:2])
		length := binary.BigEndian.Uint16(header[4:6])
		unitID := header[6]

		if length < 2 {
			return
		}
		pdu := make([]byte, length-1)
		if _, err := io.ReadFull(conn, pdu); err != nil {
			return
		}

		if s.shouldDrop() {
			return
		}
		if s.latency > 0 {
			time.Sleep(s.latency)
		}

		respPDU := s.dispatch(pdu)

		respHeader := make([]byte, 7)
		binary.BigEndian.PutUint16(respHeader[0:2], txID)
		binary.BigEndian.PutUint16(respHeader[2:4], 0)
		binary.BigEndian.PutUint16(respHeader[4:6], uint16(len(respPDU)+1))
		respHeader[6] = unitID

		if _, err := conn.Write(append(respHeader, respPDU...)); err != nil {
			return
		}
	}
}

func exception(fc, code byte) []byte { return []byte{fc | exceptionBit, code} }

func (s *Server) dispatch(pdu []byte) []byte {
	if len(pdu) == 0 {
		return exception(0, exIllegalFunction)
	}
	fc := pdu[0]
	s.mu.Lock()
	defer s.mu.Unlock()

	switch fc {
	case fcReadCoils:
		return s.readBits(pdu, fc, s.coils[:])
	case fcReadDiscreteInputs:
		return s.readBits(pdu, fc, s.discrete[:])
	case fcReadHoldingRegisters:
		return s.readRegs(pdu, fc, s.holding[:])
	case fcReadInputRegisters:
		return s.readRegs(pdu, fc, s.input[:])
	case fcWriteSingleCoil:
		return s.writeSingleCoil(pdu)
	case fcWriteSingleRegister:
		return s.writeSingleRegister(pdu)
	case fcWriteMultipleCoils:
		return s.writeMultipleCoils(pdu)
	case fcWriteMultipleRegs:
		return s.writeMultipleRegisters(pdu)
	default:
		return exception(fc, exIllegalFunction)
	}
}

func (s *Server) readBits(pdu []byte, fc byte, table []bool) []byte {
	if len(pdu) != 5 {
		return exception(fc, exIllegalValue)
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	qty := binary.BigEndian.Uint16(pdu[3:5])
	if qty < 1 || qty > 2000 {
		return exception(fc, exIllegalValue)
	}
	if int(addr)+int(qty) > len(table) {
		return exception(fc, exIllegalAddress)
	}
	byteCount := (int(qty) + 7) / 8
	out := make([]byte, 2+byteCount)
	out[0] = fc
	out[1] = byte(byteCount)
	for i := 0; i < int(qty); i++ {
		if table[int(addr)+i] {
			out[2+i/8] |= 1 << uint(i%8)
		}
	}
	return out
}

func (s *Server) readRegs(pdu []byte, fc byte, table []uint16) []byte {
	if len(pdu) != 5 {
		return exception(fc, exIllegalValue)
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	qty := binary.BigEndian.Uint16(pdu[3:5])
	if qty < 1 || qty > 125 {
		return exception(fc, exIllegalValue)
	}
	if int(addr)+int(qty) > len(table) {
		return exception(fc, exIllegalAddress)
	}
	out := make([]byte, 2+int(qty)*2)
	out[0] = fc
	out[1] = byte(qty * 2)
	for i := 0; i < int(qty); i++ {
		binary.BigEndian.PutUint16(out[2+2*i:4+2*i], table[int(addr)+i])
	}
	return out
}

func (s *Server) writeSingleCoil(pdu []byte) []byte {
	if len(pdu) != 5 {
		return exception(fcWriteSingleCoil, exIllegalValue)
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	val := binary.BigEndian.Uint16(pdu[3:5])
	if val != 0xFF00 && val != 0x0000 {
		return exception(fcWriteSingleCoil, exIllegalValue)
	}
	s.coils[addr] = val == 0xFF00
	return append([]byte{fcWriteSingleCoil}, pdu[1:]...)
}

func (s *Server) writeSingleRegister(pdu []byte) []byte {
	if len(pdu) != 5 {
		return exception(fcWriteSingleRegister, exIllegalValue)
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	val := binary.BigEndian.Uint16(pdu[3:5])
	s.holding[addr] = val
	return append([]byte{fcWriteSingleRegister}, pdu[1:]...)
}

func (s *Server) writeMultipleCoils(pdu []byte) []byte {
	if len(pdu) < 6 {
		return exception(fcWriteMultipleCoils, exIllegalValue)
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	qty := binary.BigEndian.Uint16(pdu[3:5])
	byteCount := int(pdu[5])
	if qty < 1 || qty > 1968 || byteCount != (int(qty)+7)/8 || len(pdu) != 6+byteCount {
		return exception(fcWriteMultipleCoils, exIllegalValue)
	}
	if int(addr)+int(qty) > len(s.coils) {
		return exception(fcWriteMultipleCoils, exIllegalAddress)
	}
	for i := 0; i < int(qty); i++ {
		s.coils[int(addr)+i] = pdu[6+i/8]&(1<<uint(i%8)) != 0
	}
	resp := make([]byte, 5)
	resp[0] = fcWriteMultipleCoils
	copy(resp[1:5], pdu[1:5])
	return resp
}

func (s *Server) writeMultipleRegisters(pdu []byte) []byte {
	if len(pdu) < 6 {
		return exception(fcWriteMultipleRegs, exIllegalValue)
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	qty := binary.BigEndian.Uint16(pdu[3:5])
	byteCount := int(pdu[5])
	if qty < 1 || qty > 123 || byteCount != int(qty)*2 || len(pdu) != 6+byteCount {
		return exception(fcWriteMultipleRegs, exIllegalValue)
	}
	if int(addr)+int(qty) > len(s.holding) {
		return exception(fcWriteMultipleRegs, exIllegalAddress)
	}
	for i := 0; i < int(qty); i++ {
		s.holding[int(addr)+i] = binary.BigEndian.Uint16(pdu[6+2*i : 8+2*i])
	}
	resp := make([]byte, 5)
	resp[0] = fcWriteMultipleRegs
	copy(resp[1:5], pdu[1:5])
	return resp
}

// --- Test/demo setup helpers ---------------------------------------------

func (s *Server) SetHolding(addr uint16, v uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holding[addr] = v
}

func (s *Server) SetHoldingRange(addr uint16, vs []uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range vs {
		s.holding[int(addr)+i] = v
	}
}

func (s *Server) GetHolding(addr uint16) uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.holding[addr]
}

func (s *Server) SetInput(addr uint16, v uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input[addr] = v
}

func (s *Server) SetCoil(addr uint16, v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.coils[addr] = v
}

func (s *Server) GetCoil(addr uint16) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.coils[addr]
}

func (s *Server) SetDiscrete(addr uint16, v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discrete[addr] = v
}
