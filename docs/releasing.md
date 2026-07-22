# Releasing otcat

otcat is released with [GoReleaser](https://goreleaser.com) (OSS,
free tier) and distributed three ways: raw archives on GitHub Releases,
`go install`, and native OS packages (`.deb`/`.rpm`/`.apk`) hosted on
[Cloudsmith](https://cloudsmith.io) so `apt`/`dnf`/`apk` users get a
real, signed-repository install experience instead of "download a
random binary off GitHub."

## What's actually verified vs. what's configured

Everything in `.goreleaser.yaml` was run for real in this project's
build environment via `goreleaser release --snapshot --clean`: all 24
binaries (4 tools × {linux,darwin,windows} × {amd64,arm64}) built, all
6 archives produced, all 6 Linux packages (`deb`×2, `rpm`×2, `apk`×2)
built by nfpm, and the resulting `.deb` was actually installed with
`dpkg -i` and run. What is *not* verified from this environment: the
Cloudsmith push step, because that needs a real Cloudsmith API token
this project's build sandbox does not have. That step is written
against Cloudsmith's documented CLI behavior but you should treat it
as configured, not proven, until you run it once with real
credentials.

## One-time setup

1. **GitHub**: nothing to do — `release.yml` uses the automatic
   `GITHUB_TOKEN`.
2. **Cloudsmith**: create an account and a repository (this project
   assumes namespace `quitoperation`, repo `otcat` — update
   `.github/workflows/release.yml` if yours differs), then create an
   API token and add it as a repository secret named
   `CLOUDSMITH_API_KEY`. Without that secret set, the release workflow
   still runs — the Cloudsmith step is skipped (`if:
   env.CLOUDSMITH_API_KEY != ''`), not failed.
3. Tag and push: `git tag v1.0.0 && git push --tags`. That's the entire
   trigger.

## Local testing before you ever push a tag

```sh
# validate the config
goreleaser check

# build everything as if releasing, without publishing anything
goreleaser release --snapshot --clean

# sanity-check one artifact for real
sudo dpkg -i dist/otcat_*_amd64.deb
otcat --version
sudo dpkg -r otcat
```

## Why Cloudsmith via CLI instead of GoReleaser's native integration

GoReleaser has a built-in `cloudsmiths:` publisher block — but as of
this writing it is a **GoReleaser Pro** feature. This project uses the
free OSS distribution, so `release.yml` instead runs
[`cloudsmith-cli`](https://github.com/cloudsmith-io/cloudsmith-cli)
(`pip install cloudsmith-cli`) directly against the `.deb`/`.rpm`/`.apk`
files nfpm already produced:

```sh
cloudsmith push deb quitoperation/otcat/ubuntu/noble dist/otcat_1.0.0_amd64.deb
cloudsmith push rpm quitoperation/otcat/el/9 dist/otcat-1.0.0-1.x86_64.rpm
cloudsmith push alpine quitoperation/otcat/alpine/v3.20 dist/otcat_1.0.0_x86_64.apk
```

If you later upgrade to GoReleaser Pro, the equivalent native config is:

```yaml
cloudsmiths:
  - organization: quitoperation
    repository: otcat
    distributions:
      deb: ["ubuntu/noble", "ubuntu/jammy"]
      rpm: "el/9"
      alpine: "alpine/v3.20"
```

## Installing otcat (end-user instructions)

**Debian / Ubuntu**, once the Cloudsmith apt repo is live:

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/quitoperation/otcat/setup.deb.sh' | sudo -E bash
sudo apt install otcat
```

**Fedora / RHEL / CentOS**:

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/quitoperation/otcat/setup.rpm.sh' | sudo -E bash
sudo dnf install otcat
```

**Alpine**:

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/quitoperation/otcat/setup.alpine.sh' | sh
apk add otcat
```

(The exact `setup.*.sh` URLs above are Cloudsmith's standard
per-repository bootstrap script pattern; confirm the real ones from
your repository's "Set Me Up" page in the Cloudsmith dashboard once it
exists — don't trust these blindly before that.)

**Any OS with Go 1.22+, no package manager needed:**

```sh
go install github.com/QuitOperation/otcat/cmd/otcat@latest
go install github.com/QuitOperation/otcat/cmd/otc@latest
```

**Everyone else**: download the archive for your OS/arch from the
[Releases page](https://github.com/QuitOperation/otcat/releases).

## Package naming and arch mapping

| GoReleaser arch | Debian arch | RPM arch  | Alpine arch |
|---|---|---|---|
| amd64 | amd64 | x86_64 | x86_64 |
| arm64 | arm64 | aarch64 | aarch64 |

`otcat-mockplc` and `otcat-latencyprobe` are archived (available in the
`.tar.gz`/`.zip` downloads and via `go install`) but deliberately left
out of the `.deb`/`.rpm`/`.apk` packages — they're demo/dev tools, not
something that belongs on a system-wide `PATH` via a package manager.
