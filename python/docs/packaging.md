# Python packaging: how the Go binary gets into the wheel

## What's real vs. what's the plan

This source tree ships one bundled binary,
`python/otcat/_bin/otcat-linux-amd64`, built directly from this
project's own Go source in this environment. It is what makes the
Python test suite an actual integration test rather than a mock of a
mock: `Client()` with no arguments resolves to that real binary and
talks to a real `otcat-mockplc` over a real socket (see
`tests/conftest.py`).

Shipping *one* platform's binary in a source checkout is fine for
development. Shipping it on PyPI is not — a `pip install otcat` on
macOS or Windows would silently get a Linux binary that can't execute.
The correct end state is **platform-specific wheels**, each containing
only the one binary that platform needs, which is what the CI
pipeline below is designed to produce. It has not been run for real
end-to-end (that requires PyPI publish credentials this project's
build environment doesn't have) — the Linux leg has been verified
directly (build binary → build wheel → install wheel → run tests
against it, all in this repo's own CI-equivalent commands); the
macOS/Windows legs follow the identical, mechanical pattern.

## The pipeline

```
GoReleaser (.goreleaser.yaml)
  --> dist/otcat_<os>_<arch>/otcat[.exe]     (already verified, see docs/releasing.md)
        |
        v
copy each into python/otcat/_bin/otcat-<os>-<arch>[.exe]
        |
        v
build ONE wheel per (os, arch) pair, each containing only its own binary,
tagged with the right platform tag (manylinux_x86_64, macosx_11_0_arm64,
win_amd64, ...) so pip's own platform matching does the rest
        |
        v
twine upload dist/*.whl  (+ one sdist, which ships NO binary and falls
                           back to $PATH / OTCAT_BINARY at import time --
                           see otcat/_binary.py)
```

## Why platform-specific wheels instead of one universal wheel

A wheel is not allowed to contain multiple incompatible native
binaries and pick one at install time — `pip` picks the *wheel* based
on platform tags before anything inside it runs. The standard,
widely-used pattern (the same one `ruff`, ripgrep's Python wrapper, and
many other Go/Rust-backed Python tools use) is: build the native
binary once per target with the language's own cross-compiler (Go's
`GOOS`/`GOARCH`, already free — see `.goreleaser.yaml`), then build a
separate wheel per target that embeds just that one binary and
declares the matching platform tag.

## GitHub Actions sketch (`python-release.yml`, not yet added to
## `.github/workflows/` -- add it once PyPI publishing is actually
## wanted, so a stray push can't trigger a broken/premature publish)

```yaml
name: python-release
on:
  push:
    tags: ["v*"]
jobs:
  build-wheels:
    strategy:
      matrix:
        include:
          - { os: ubuntu-latest,  goos: linux,   goarch: amd64, tag: manylinux_2_17_x86_64 }
          - { os: ubuntu-latest,  goos: linux,   goarch: arm64, tag: manylinux_2_17_aarch64 }
          - { os: macos-latest,   goos: darwin,  goarch: amd64, tag: macosx_11_0_x86_64 }
          - { os: macos-latest,   goos: darwin,  goarch: arm64, tag: macosx_11_0_arm64 }
          - { os: windows-latest, goos: windows, goarch: amd64, tag: win_amd64 }
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.22" }
      - run: |
          ext=""; [ "${{ matrix.goos }}" = "windows" ] && ext=".exe"
          GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
            go build -ldflags "-s -w" \
            -o python/otcat/_bin/otcat-${{ matrix.goos }}-${{ matrix.goarch }}$ext \
            ./cmd/otcat
      - uses: actions/setup-python@v5
        with: { python-version: "3.12" }
      - run: pip install build
      - working-directory: python
        run: python -m build --wheel -C--build-option=--plat-name -C--build-option=${{ matrix.tag }}
      - uses: actions/upload-artifact@v4
        with: { name: wheel-${{ matrix.tag }}, path: python/dist/*.whl }

  publish:
    needs: build-wheels
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { path: dist, merge-multiple: true }
      - run: pip install twine
      - run: twine upload dist/*.whl
        env:
          TWINE_USERNAME: __token__
          TWINE_PASSWORD: ${{ secrets.PYPI_API_TOKEN }}
```

## Before actually publishing to PyPI

- Confirm the name `otcat` is available on PyPI (not checked from this
  environment — no network path to pypi.org's search from here that
  wouldn't also require deciding on a fallback name in the same
  breath; do this manually before the first publish).
- Decide the sdist's behavior deliberately: it ships no binary at all
  (by design, per the pipeline above), so `pip install otcat` from
  source on a platform with no wheel falls straight to `_binary.py`'s
  `$PATH`/`OTCAT_BINARY` search. Document that clearly in the PyPI
  project description, not just in this file.
