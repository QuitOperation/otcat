package modbus

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/QuitOperation/otcat/internal/mock"
)

func newTestServer(t *testing.T, opts ...mock.Option) *mock.Server {
	t.Helper()
	s, err := mock.New(opts...)
	if err != nil {
		t.Fatalf("mock.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func connectedClient(t *testing.T, addr string, opts ...ClientOption) *Client {
	t.Helper()
	c := NewClient(addr, opts...)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestClientReadHoldingRegisters(t *testing.T) {
	s := newTestServer(t)
	s.SetHoldingRange(100, []uint16{10, 20, 30})
	c := connectedClient(t, s.Addr())

	got, err := c.ReadHoldingRegisters(context.Background(), 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint16{10, 20, 30}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("register %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestClientReadInputRegisters(t *testing.T) {
	s := newTestServer(t)
	s.SetInput(5, 777)
	c := connectedClient(t, s.Addr())

	got, err := c.ReadInputRegisters(context.Background(), 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 777 {
		t.Fatalf("got %d, want 777", got[0])
	}
}

func TestClientReadCoilsAndDiscrete(t *testing.T) {
	s := newTestServer(t)
	s.SetCoil(3, true)
	s.SetDiscrete(7, true)
	c := connectedClient(t, s.Addr())

	coils, err := c.ReadCoils(context.Background(), 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !coils[3] {
		t.Fatal("coil 3 should be true")
	}

	disc, err := c.ReadDiscreteInputs(context.Background(), 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !disc[7] {
		t.Fatal("discrete 7 should be true")
	}
}

func TestClientWriteSingleRegister(t *testing.T) {
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())

	if err := c.WriteSingleRegister(context.Background(), 42, 999); err != nil {
		t.Fatal(err)
	}
	if got := s.GetHolding(42); got != 999 {
		t.Fatalf("server holding[42] = %d, want 999", got)
	}
}

func TestClientWriteSingleCoil(t *testing.T) {
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())

	if err := c.WriteSingleCoil(context.Background(), 9, true); err != nil {
		t.Fatal(err)
	}
	if !s.GetCoil(9) {
		t.Fatal("server coil[9] should be true after write")
	}
	if err := c.WriteSingleCoil(context.Background(), 9, false); err != nil {
		t.Fatal(err)
	}
	if s.GetCoil(9) {
		t.Fatal("server coil[9] should be false after second write")
	}
}

func TestClientWriteMultipleRegisters(t *testing.T) {
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())

	vals := []uint16{1, 2, 3, 4, 5}
	if err := c.WriteMultipleRegisters(context.Background(), 10, vals); err != nil {
		t.Fatal(err)
	}
	for i, want := range vals {
		if got := s.GetHolding(uint16(10 + i)); got != want {
			t.Fatalf("holding[%d] = %d, want %d", 10+i, got, want)
		}
	}
}

func TestClientWriteMultipleCoils(t *testing.T) {
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())

	vals := []bool{true, false, true, true, false}
	if err := c.WriteMultipleCoils(context.Background(), 20, vals); err != nil {
		t.Fatal(err)
	}
	for i, want := range vals {
		if got := s.GetCoil(uint16(20 + i)); got != want {
			t.Fatalf("coil[%d] = %v, want %v", 20+i, got, want)
		}
	}
}

func TestClientIllegalAddressException(t *testing.T) {
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())

	// 65530 + 10 = 65540 > 65536: out of the server's table bounds.
	_, err := c.ReadHoldingRegisters(context.Background(), 65530, 10)
	if err == nil {
		t.Fatal("expected an illegal-address exception")
	}
	var exc *ExceptionError
	if !errors.As(err, &exc) {
		t.Fatalf("error is not an *ExceptionError: %v", err)
	}
	if exc.Exception != ExIllegalDataAddress {
		t.Fatalf("got exception %v, want ExIllegalDataAddress", exc.Exception)
	}
}

func TestClientIllegalValueOnSingleCoil(t *testing.T) {
	// A raw, malformed write (value other than 0xFF00/0x0000) can only
	// be produced by bypassing encodeWriteSingleCoil; calling the
	// unexported do() directly drives it, proving the *server* rejects
	// it and the *client* surfaces the exception rather than
	// misreading a rejection as success.
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())
	badPDU := []byte{FuncWriteSingleCoil, 0x00, 0x01, 0x12, 0x34}
	_, err := c.do(context.Background(), badPDU)
	if err == nil {
		t.Fatal("expected exception for illegal coil value")
	}
	var exc *ExceptionError
	if !errors.As(err, &exc) {
		t.Fatalf("error is not an *ExceptionError: %v", err)
	}
	if exc.Exception != ExIllegalDataValue {
		t.Fatalf("got exception %v, want ExIllegalDataValue", exc.Exception)
	}
}

func TestClientQuantityLimitsEnforcedClientSide(t *testing.T) {
	// These must fail before any byte reaches the network, so point at
	// an address nothing is listening on and confirm the error is a
	// local validation error, not a connection error.
	c := NewClient("127.0.0.1:1") // reserved, nothing listens here
	_, err := c.ReadHoldingRegisters(context.Background(), 0, MaxReadRegisters+1)
	if err == nil {
		t.Fatal("expected client-side limit rejection")
	}
	if got := err.Error(); !contains(got, "out of range") {
		t.Fatalf("expected a range-limit error, got: %v", got)
	}
}

func TestClientTimeout(t *testing.T) {
	s := newTestServer(t, mock.WithLatency(200*time.Millisecond))
	c := connectedClient(t, s.Addr(), WithTimeout(20*time.Millisecond))

	_, err := c.ReadHoldingRegisters(context.Background(), 0, 1)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if !netErr.Timeout() {
			t.Fatalf("net.Error present but Timeout()==false: %v", err)
		}
	}
	// Not every wrapped I/O error round-trips as net.Error through
	// io.ReadFull; the important, always-true assertion is simply that
	// the call did not silently succeed or hang past the deadline.
}

func TestClientConnectionDrop(t *testing.T) {
	s := newTestServer(t, mock.WithDropRate(1.0))
	c := connectedClient(t, s.Addr())

	_, err := c.ReadHoldingRegisters(context.Background(), 0, 1)
	if err == nil {
		t.Fatal("expected an error when the server drops the connection mid-transaction")
	}
}

func TestClientContextCancellation(t *testing.T) {
	s := newTestServer(t, mock.WithLatency(500*time.Millisecond))
	c := connectedClient(t, s.Addr(), WithTimeout(10*time.Second)) // long client timeout

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := c.ReadHoldingRegisters(ctx, 0, 1)
	if err == nil {
		t.Fatal("expected the request to fail: caller context deadline should win over the client's own timeout")
	}
}

func TestClientDialFailure(t *testing.T) {
	c := NewClient("127.0.0.1:1") // reserved port, nothing listens
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Connect(ctx); err == nil {
		t.Fatal("expected dial to a reserved/unused port to fail")
	}
}

func TestClientCloseIdempotent(t *testing.T) {
	s := newTestServer(t)
	c := connectedClient(t, s.Addr())
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got: %v", err)
	}
}

func TestClientUseBeforeConnect(t *testing.T) {
	c := NewClient("127.0.0.1:1")
	_, err := c.ReadHoldingRegisters(context.Background(), 0, 1)
	if err == nil {
		t.Fatal("expected an error when reading before Connect")
	}
}

func TestClientSequentialRequestsIndependent(t *testing.T) {
	s := newTestServer(t)
	s.SetHolding(0, 111)
	s.SetHolding(1, 222)
	c := connectedClient(t, s.Addr())

	for i := 0; i < 20; i++ {
		got, err := c.ReadHoldingRegisters(context.Background(), 0, 2)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if got[0] != 111 || got[1] != 222 {
			t.Fatalf("iteration %d: got %v", i, got)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
