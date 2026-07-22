# Terminal demos

Three recordings, each a real otcat/otc binary running against a real
(mock) Modbus TCP server — nothing staged, no output hand-edited.

| GIF | Shows |
|---|---|
| `gifs/quickstart.gif` | Spin up `otcat-mockplc`, read a register, read a float32, watch a live value |
| `gifs/write_safety.gif` | The write-confirmation gate refusing, `--dry-run`, then an actual confirmed write |
| `gifs/pipes.gif` | ndjson into `jq`, `--raw` into `awk`, `--json` into `grep -o` |

## How these were actually made

Two independent pipelines exist for the same source scripts, because
the intended tool, [VHS](https://github.com/charmbracelet/vhs), needs
a real Chromium/Chrome binary to render frames (it drives a headless
browser via `go-rod`), and that binary was not obtainable from this
project's build sandbox's network allowlist — every download path VHS
tries (`storage.googleapis.com`, a couple of mirrors) was unreachable.
This is a sandbox limitation, not a otcat limitation: VHS works fine
wherever normal internet access to those hosts exists.

**What actually generated `gifs/*.gif`:** the shell scripts in this
directory (`quickstart.sh`, `write_safety.sh`, `pipes.sh`), each run
for real under [asciinema](https://asciinema.org/) (which needs no
browser — it just wraps a pty) and converted to GIF with
[`agg`](https://github.com/asciinema/agg) (pure Rust, no browser,
static binary):

```sh
asciinema rec --cols 100 --rows 20 --idle-time-limit 2 \
  -c "bash quickstart.sh" quickstart.cast
agg quickstart.cast gifs/quickstart.gif --font-size 16
```

**What's in `tapes/*.tape`:** the same three demos, written in VHS's
own format, ready to run wherever a real VHS install exists:

```sh
# with VHS + ffmpeg + a real browser installed locally, or:
docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs tapes/quickstart.tape
```

The two pipelines are deliberately kept in sync by hand — if you
change one, change the other. `_typing.sh`'s per-character typing
delay (18ms) matches the `.tape` files' `Set TypingSpeed 18ms` for the
same reason.

## One honesty note on the asciinema scripts

Every *command's output* in `quickstart.sh` / `write_safety.sh` /
`pipes.sh` is completely real — the actual compiled binary, actually
executed, actually talking to a real socket. The one simulated part is
the **keystroke-by-keystroke typing animation** before each command
(`_typing.sh`'s `type_line`), which exists for deterministic,
reproducible recordings — the exact same effect VHS's own `Type`
command produces when driven from a script rather than a human's live
typing. Nothing about *what the tool did* is simulated, only *how fast
someone appeared to type it*.

One deliberate implementation detail: the refusal demo in
`write_safety.sh` displays the command a user would actually type
(`otc --write ... --value 999`, no stdin redirection) but executes it
with stdin piped from an empty source (`echo -n | otc ...`). This
guarantees the deterministic "non-interactive, refuse" code path fires
regardless of whether the recording tool's own pty makes otcat's
inherited stdin look like a terminal — without it, the same command
could nondeterministically hit the interactive y/N prompt path instead
and hang waiting for input the script never provides. Every other
command runs exactly as displayed.

## Regenerating

```sh
go build -o /usr/local/bin/otc ./cmd/otc          # or anywhere on $PATH
go build -o /usr/local/bin/otcat ./cmd/otcat
go build -o /usr/local/bin/otcat-mockplc ./cmd/otcat-mockplc
cd demo
bash quickstart.sh      # sanity-check it runs cleanly on its own first
asciinema rec --cols 100 --rows 20 --idle-time-limit 2 -c "bash quickstart.sh" q.cast
agg q.cast gifs/quickstart.gif --font-size 16
# repeat for write_safety.sh and pipes.sh
```
