# Driver roadmap

otcat ships one complete driver — Modbus TCP — and three registered but
unimplemented ones: EtherNet/IP (CIP), S7comm, and BACnet/IP. This is a
scope decision, not an oversight, and this document is the honest
accounting of it: what each protocol actually requires, and in what
order they would be built.

## Why Modbus first, and alone, in this release

Modbus is the smallest correctly-implementable target in this space: a
public specification, an 8-function-code surface for data access, a
5-byte fixed request header, and no session/connection state beyond the
TCP socket itself. It is also still the most widely deployed fieldbus
in brownfield industrial installations, which makes it the highest-value
single protocol to get exactly right before spreading effort thinner.

## What each remaining driver actually needs

**EtherNet/IP (CIP).** A Register Session / UnRegister Session
encapsulation handshake; Unconnected Send (service 0x52) for one-off
explicit messages and Forward Open/Forward Close for a connected
session; CIP `Get_Attribute_Single`/`Get_Attribute_List`/`Set_Attribute_*`
services; symbolic segment encoding for tag names; and per-vendor
structured-tag (Rockwell UDT) layout handling, since CIP's own type
system does not fully standardize how a UDT serializes on the wire.
Reference: ODVA, *The CIP Networks Library, Volume 1: Common Industrial
Protocol (CIP)* and *Volume 2: EtherNet/IP Adaptation of CIP*.

**S7comm.** COTP (ISO-on-TCP, RFC 1006) connection setup carrying an S7
PDU negotiation, then S7 "job" requests encoding an item address as
(area code, DB number, byte offset, bit offset, transport size).
Siemens has not published this protocol; every existing open
implementation (snap7, python-snap7, Sharp7) is derived from packet
capture, which is exactly the kind of "looks right, might not be"
surface this project chooses not to ship silently.

**BACnet/IP.** UDP-native; typically requires Who-Is/I-Am discovery
before an address is resolvable at all; APDUs use a tag-length-value
encoding with both application and context tags and explicit
opening/closing tags for constructed data — a materially larger parser
than Modbus's fixed-position PDU. Reference: ASHRAE/ANSI Standard 135,
*BACnet — A Data Communication Protocol for Building Automation and
Control Networks.*

## What "implemented" will mean before a driver ships

Each future driver is held to the same bar Modbus was built to in this
release: a from-specification (or, where no specification exists,
cross-validated against at least two independent implementations)
encoder/decoder; a mock server exercising every function/service it
claims to support; fuzz coverage on every parser touching bytes from
the network; and — specifically for any write path — a dry-run mode
that reports the exact payload before it is sent. A driver that cannot
clear that bar stays a stub with a clear error rather than shipping
partial, silently-incomplete protocol support.
