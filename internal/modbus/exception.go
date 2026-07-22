package modbus

import "fmt"

// ExceptionCode is the one-byte code a server returns in place of data
// when it rejects a request. Values and names are taken verbatim from
// the Modbus Application Protocol Specification V1.1b3, §7, MB Exception
// Responses.
type ExceptionCode byte

const (
	ExIllegalFunction                    ExceptionCode = 0x01
	ExIllegalDataAddress                 ExceptionCode = 0x02
	ExIllegalDataValue                   ExceptionCode = 0x03
	ExServerDeviceFailure                ExceptionCode = 0x04
	ExAcknowledge                        ExceptionCode = 0x05
	ExServerDeviceBusy                   ExceptionCode = 0x06
	ExMemoryParityError                  ExceptionCode = 0x08
	ExGatewayPathUnavailable             ExceptionCode = 0x0A
	ExGatewayTargetDeviceFailedToRespond ExceptionCode = 0x0B
)

func (e ExceptionCode) String() string {
	switch e {
	case ExIllegalFunction:
		return "illegal function"
	case ExIllegalDataAddress:
		return "illegal data address"
	case ExIllegalDataValue:
		return "illegal data value"
	case ExServerDeviceFailure:
		return "server device failure"
	case ExAcknowledge:
		return "acknowledge"
	case ExServerDeviceBusy:
		return "server device busy"
	case ExMemoryParityError:
		return "memory parity error"
	case ExGatewayPathUnavailable:
		return "gateway path unavailable"
	case ExGatewayTargetDeviceFailedToRespond:
		return "gateway target device failed to respond"
	default:
		return fmt.Sprintf("unknown exception 0x%02X", byte(e))
	}
}

// ExceptionError wraps a server-issued exception response. It is a
// distinct type (not a plain error string) so callers — the CLI's exit
// code logic in particular — can errors.As it and map protocol-level
// rejection to a stable, scriptable exit status distinct from network
// or timeout failure.
type ExceptionError struct {
	Function      byte
	Exception     ExceptionCode
	UnitID        byte
	TransactionID uint16
}

func (e *ExceptionError) Error() string {
	return fmt.Sprintf("modbus: unit %d function 0x%02X: %s (0x%02X)",
		e.UnitID, e.Function, e.Exception, byte(e.Exception))
}
