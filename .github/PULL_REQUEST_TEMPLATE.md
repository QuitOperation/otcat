## Description

Provide a clear description of the changes introduced in this Pull Request.

Fixes #(issue)

## Type of Change

- [ ] 🐛 Bug fix (non-breaking change fixing an issue)
- [ ] ✨ New feature (non-breaking change adding functionality or protocol support)
- [ ] ⚠️ Breaking change (fix or feature that alters CLI flags or output structure)
- [ ] 📚 Documentation update
- [ ] ⚙️ CI / Build / Refactoring

## Protocol Impact

- [ ] Modbus TCP
- [ ] EtherNet/IP (CIP)
- [ ] S7comm
- [ ] BACnet/IP
- [ ] Core CLI / Shared Network Stack

## Checklist

- [ ] My code follows the project's zero-dependency policy (standard library only).
- [ ] I have executed `go fmt ./...` and `go vet ./...`.
- [ ] I have added unit tests for my changes and ran `go test -race ./...`.
- [ ] All new and existing tests pass locally.
- [ ] Documentation has been updated accordingly (if applicable).
- [ ] For write operations (`--write`), safety confirmations (`--confirm`) and validation are strictly enforced.

## Testing Verification

Describe the tests executed to verify your changes (e.g. mock server tests, physical PLC hardware test setup, packet capture analysis).
