# Security Policy

## 🛡️ Operational Safety Notice (Industrial Systems)

`otcat` interacts directly with industrial control systems (ICS), Programmable Logic Controllers (PLCs), Remote Terminal Units (RTUs), and SCADA networks using protocols like Modbus TCP, EtherNet/IP, S7comm, and BACnet/IP.

> [!CAUTION]
> **Writing to physical PLCs or industrial process equipment carries inherent operational risks.**
> Incorrect register writes can lead to machine malfunction, process interruption, or safety hazards. Always verify target IP addresses, register offsets, unit IDs, and payload values before executing write operations (`--write`). Use `--confirm` in interactive environments.

---

## 🔒 Supported Versions

We provide security updates and patches for the following versions of `otcat`:

| Version | Supported |
| ------- | --------- |
| `v1.0.x` (Latest Release) | ✅ Yes |
| `< v1.0.0` | ❌ No |
| `main` branch | ✅ Yes |

---

## 🚨 Reporting a Vulnerability

If you discover a potential security vulnerability in `otcat` (such as a buffer overflow, out-of-bounds register parsing, infinite loop in decoder, unhandled packet panic, or unexpected write path exposure), please report it responsibly.

**Please DO NOT open a public GitHub Issue for security vulnerabilities.**

Instead, report the issue privately:

1. **Email**: Send details to `security@quitoperation.org` or contact repository maintainers directly.
2. **Details to Include**:
   - Description of the vulnerability and potential impact.
   - Proof-of-concept (PoC) or exact `otcat` CLI commands / pcap file to trigger the issue.
   - Affected version(s) and operating system / architecture.
   - Any suggested remediations or patches.

### Response Timeline
* **Acknowledgement**: Within 48 hours.
* **Assessment & Fix Plan**: Within 7 business days.
* **Public Disclosure**: Coordinated release after patch verification.

---

## 📋 Security Design Guidelines

`otcat` enforces security and safety principles by design:

- **Zero Third-Party Dependencies**: Minimal attack surface with zero supply-chain package dependencies.
- **Fail Closed**: Network timeouts, malformed protocol responses, and disconnects immediately return structured error codes rather than hanging or retrying blindly.
- **Explicit Write Confirmation**: Destructive CLI commands require explicit user confirmation or explicit standard input piping flags (`--confirm`, `--from-stdin`).
