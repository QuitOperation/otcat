# Contributing to otcat

Thank you for your interest in contributing to **otcat** — the netcat for industrial I/O!

Whether you are fixing a bug, adding support for a new protocol, refining documentation, or improving test coverage, we welcome your contributions.

---

## 📜 Principles & Core Constraints

1. **Zero External Dependencies**: `otcat` relies exclusively on Go's standard library. No third-party packages are allowed in `go.mod`. Any protocol parser or codec must be implemented within the repository.
2. **Safety First**: Writing to industrial controllers (PLCs, PACs, RTUs) can impact physical infrastructure. All write operations must respect explicit safety flags (such as `--confirm`), fail closed on unexpected network states, and undergo strict verification.
3. **Unix Philosophy**: `otcat` is designed to compose with standard Unix tools (`awk`, `jq`, `grep`, `tee`). Input should be standard, output should be predictable (`--json`, `--raw`, clean exit codes).

---

## 🛠️ Getting Started

### Requirements
- **Go**: 1.22 or higher
- **Git**: Standard git workflow
- **Make** (optional): Standard build target commands (`make build`, `make test`, `make lint`)

### Development Setup

```sh
# Clone the repository
git clone https://github.com/QuitOperation/otcat.git
cd otcat

# Run unit tests
go test -v ./...

# Build the binaries locally
go build -o otcat ./cmd/otcat
go build -o otc ./cmd/otc
```

---

## 🧪 Testing & Verification

Before submitting a Pull Request, ensure that all tests pass and code is formatted according to standard Go conventions:

```sh
# Format code
go fmt ./...

# Vet code
go vet ./...

# Run unit and race detector tests
go test -race -v ./...
```

If you are developing or modifying Modbus protocol functionality, test against local mock servers or the included simulator test suites in `internal/modbus`.

---

## 🗺️ Protocol Driver Extensions

If you plan to implement one of the registered roadmap protocols (**EtherNet/IP**, **S7comm**, or **BACnet/IP**), please review [`docs/driver_roadmap.md`](docs/driver_roadmap.md) first.

Key driver requirements:
- Place core driver logic under `internal/<protocol>/`.
- Define CLI flag registration in `internal/cli/`.
- Ensure explicit `--read`, `--write`, and `--watch` behavior matching `otcat` conventions.
- Never write to a device without validating address ranges and requiring `--confirm` for destructive operations.

---

## 🐛 Submitting Issues & Feature Requests

- Check existing issues before opening a new one to avoid duplicates.
- Use our [Issue Templates](https://github.com/QuitOperation/otcat/issues/new/choose) for bug reports and feature requests.
- Provide detailed steps to reproduce, including your OS, Go version, PLC model/simulator, and exact `otcat` flags used.

---

## 🔀 Pull Request Process

1. **Fork & Branch**: Create a feature branch off `main` (e.g., `git checkout -b feature/s7comm-header-parser`).
2. **Commit Messages**: Write clear, descriptive commit messages.
3. **Keep PRs Focused**: Avoid bundling unrelated changes into a single PR.
4. **Pass CI**: Ensure unit tests, `go vet`, and formatting checks pass.
5. **Review**: A maintainer will review your PR. Be responsive to feedback and discussions.

---

## 🤝 Code of Conduct

All contributors are expected to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md). Please report unacceptable behavior to maintainers or `security@quitoperation.org`.
