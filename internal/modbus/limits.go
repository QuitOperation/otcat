package modbus

// These ceilings are not arbitrary; each falls out of the 253-byte PDU
// budget fixed by the Modbus Application Protocol Specification V1.1b3
// §4.1 (kept identical over TCP for serial-gateway interoperability,
// per §3.1.2), and are enforced client-side so a malformed request is
// rejected in under a microsecond instead of round-tripping to a PLC
// to find out the same thing.
//
// PDU budget: 253 bytes total = 1 (function code) + up to 252 data bytes.
const maxPDU = 253

const (
	// FC 0x03/0x04 read-register response: 1 byte-count field leaves
	// 252 - 1 = 251 data bytes -> floor(251/2) = 125 registers. §6.3, §6.4.
	MaxReadRegisters = 125

	// FC 0x01/0x02 read-bit response: same 251-byte data budget, 8 bits
	// per byte -> 2008 in theory, but the spec fixes the ceiling at
	// 2000 bits directly. §6.1, §6.2.
	MaxReadBits = 2000

	// FC 0x10 write-multiple-registers request: data section is
	// address(2) + quantity(2) + byteCount(1) + N*2 registers, must fit
	// in 252 bytes -> N <= (252-5)/2 = 123. §6.12.
	MaxWriteRegisters = 123

	// FC 0x0F write-multiple-coils request: address(2) + quantity(2)
	// + byteCount(1) + ceil(N/8) bytes must fit in 252 -> N <= 1968.
	// §6.11.
	MaxWriteBits = 1968
)
