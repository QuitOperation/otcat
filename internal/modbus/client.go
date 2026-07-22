package modbus

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Client is a single-connection, single-endpoint Modbus TCP client.
//
// It is intentionally synchronous: one request is written and its
// response fully read before the next request is sent, guarded by mu.
// The spec permits pipelining multiple in-flight transactions over one
// TCP connection (Modbus Messaging on TCP/IP Implementation Guide
// V1.0b, §4.2), but a large fraction of deployed field devices — the
// serial-to-TCP gateways in particular — are single-threaded internally
// and silently misbehave under pipelining. Serializing costs one
// round-trip of latency per request; it never costs correctness. For a
// CLI tool making tens of requests, not tens of thousands, that trade
// is not close.
type Client struct {
	addr    string
	unitID  byte
	timeout time.Duration

	mu   sync.Mutex
	conn net.Conn
	txID uint32
}

type ClientOption func(*Client)

// WithUnitID sets the Modbus unit (slave) identifier, defaulting to 1.
// Devices addressed directly over TCP (no serial gateway behind them)
// commonly ignore this field, but some product families require the
// classic 0xFF or a specific rack/slot-derived value.
func WithUnitID(id byte) ClientOption { return func(c *Client) { c.unitID = id } }

// WithTimeout bounds every individual request/response round trip.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.timeout = d }
}

func NewClient(addr string, opts ...ClientOption) *Client {
	c := &Client{addr: addr, unitID: 1, timeout: 5 * time.Second}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("modbus: dial %s: %w", c.addr, err)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		// OT request/response pairs are tiny (often under 20 bytes);
		// Nagle's algorithm exists to batch small writes and would
		// only add latency here, never useful throughput.
		_ = tc.SetNoDelay(true)
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(30 * time.Second)
	}
	c.conn = conn
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) nextTxID() uint16 {
	// Wraps at 65536 by construction (uint16 cast); a wrapped ID
	// colliding with a still-outstanding transaction is a non-issue
	// because the client never has more than one request in flight.
	return uint16(atomic.AddUint32(&c.txID, 1))
}

// do sends one PDU and returns the matching, exception-checked response
// PDU. Framing errors, transaction mismatches, and exception responses
// are all resolved here so every typed method above stays a one-liner.
func (c *Client) do(ctx context.Context, pdu []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("modbus: not connected: call Connect first")
	}

	deadline := time.Now().Add(c.timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("modbus: set deadline: %w", err)
	}

	txID := c.nextTxID()
	header := mbapHeader{
		transactionID: txID,
		protocolID:    protocolID,
		length:        uint16(len(pdu) + 1), // +1 for unitID
		unitID:        c.unitID,
	}
	frame := append(encodeMBAP(header), pdu...)

	if _, err := c.conn.Write(frame); err != nil {
		return nil, fmt.Errorf("modbus: write: %w", err)
	}

	respHeaderBuf := make([]byte, mbapLen)
	if _, err := io.ReadFull(c.conn, respHeaderBuf); err != nil {
		return nil, fmt.Errorf("modbus: read header: %w", err)
	}
	rh, err := decodeMBAP(respHeaderBuf)
	if err != nil {
		return nil, err
	}
	if rh.transactionID != txID {
		return nil, fmt.Errorf("modbus: transaction ID mismatch: sent %d, received %d (stale response, out-of-order gateway, or two clients sharing one connection)", txID, rh.transactionID)
	}

	pduLen := int(rh.length) - 1
	respPDU := make([]byte, pduLen)
	if _, err := io.ReadFull(c.conn, respPDU); err != nil {
		return nil, fmt.Errorf("modbus: read PDU (%d bytes): %w", pduLen, err)
	}

	if len(respPDU) == 0 {
		return nil, fmt.Errorf("modbus: empty PDU in response")
	}
	if ec, isExc := asException(respPDU); isExc {
		return nil, &ExceptionError{
			Function:      respPDU[0] &^ exceptionBit,
			Exception:     ec,
			UnitID:        c.unitID,
			TransactionID: txID,
		}
	}
	if wantFC := pdu[0]; respPDU[0] != wantFC {
		return nil, fmt.Errorf("modbus: function code mismatch: sent 0x%02X, received 0x%02X", wantFC, respPDU[0])
	}
	return respPDU, nil
}

// --- Typed operations, one per function code family ----------------------

func (c *Client) ReadCoils(ctx context.Context, address uint16, quantity int) ([]bool, error) {
	if quantity < 1 || quantity > MaxReadBits {
		return nil, fmt.Errorf("modbus: read coils quantity %d out of range [1,%d]", quantity, MaxReadBits)
	}
	resp, err := c.do(ctx, encodeReadRequest(FuncReadCoils, address, uint16(quantity)))
	if err != nil {
		return nil, err
	}
	return decodeReadBitsResponse(resp, quantity)
}

func (c *Client) ReadDiscreteInputs(ctx context.Context, address uint16, quantity int) ([]bool, error) {
	if quantity < 1 || quantity > MaxReadBits {
		return nil, fmt.Errorf("modbus: read discrete inputs quantity %d out of range [1,%d]", quantity, MaxReadBits)
	}
	resp, err := c.do(ctx, encodeReadRequest(FuncReadDiscreteInputs, address, uint16(quantity)))
	if err != nil {
		return nil, err
	}
	return decodeReadBitsResponse(resp, quantity)
}

func (c *Client) ReadHoldingRegisters(ctx context.Context, address uint16, quantity int) ([]uint16, error) {
	if quantity < 1 || quantity > MaxReadRegisters {
		return nil, fmt.Errorf("modbus: read holding registers quantity %d out of range [1,%d]", quantity, MaxReadRegisters)
	}
	resp, err := c.do(ctx, encodeReadRequest(FuncReadHoldingRegisters, address, uint16(quantity)))
	if err != nil {
		return nil, err
	}
	return decodeReadRegistersResponse(resp, quantity)
}

func (c *Client) ReadInputRegisters(ctx context.Context, address uint16, quantity int) ([]uint16, error) {
	if quantity < 1 || quantity > MaxReadRegisters {
		return nil, fmt.Errorf("modbus: read input registers quantity %d out of range [1,%d]", quantity, MaxReadRegisters)
	}
	resp, err := c.do(ctx, encodeReadRequest(FuncReadInputRegisters, address, uint16(quantity)))
	if err != nil {
		return nil, err
	}
	return decodeReadRegistersResponse(resp, quantity)
}

func (c *Client) WriteSingleCoil(ctx context.Context, address uint16, on bool) error {
	resp, err := c.do(ctx, encodeWriteSingleCoil(address, on))
	if err != nil {
		return err
	}
	want := coilOff
	if on {
		want = coilOn
	}
	return decodeWriteEcho(resp, address, want)
}

func (c *Client) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	resp, err := c.do(ctx, encodeWriteSingleRegister(address, value))
	if err != nil {
		return err
	}
	return decodeWriteEcho(resp, address, value)
}

func (c *Client) WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error {
	req, err := encodeWriteMultipleRegisters(address, values)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	return decodeWriteEcho(resp, address, uint16(len(values)))
}

func (c *Client) WriteMultipleCoils(ctx context.Context, address uint16, values []bool) error {
	req, err := encodeWriteMultipleCoils(address, values)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	return decodeWriteEcho(resp, address, uint16(len(values)))
}
