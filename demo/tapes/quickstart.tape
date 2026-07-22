# Real VHS (https://github.com/charmbracelet/vhs) tape. Requires VHS +
# ffmpeg + a Chromium/Chrome binary (VHS renders via a headless
# browser) -- none of which were reachable from this project's build
# sandbox, so demo/gifs/quickstart.gif was generated with an equivalent
# asciinema+agg pipeline instead (see demo/README.md). This file is the
# real, authoritative VHS source: `vhs quickstart.tape` regenerates the
# identical demo anywhere VHS itself runs (e.g. `docker run --rm -v
# $PWD:/vhs ghcr.io/charmbracelet/vhs quickstart.tape`).

Output gifs/quickstart_vhs.gif

Set FontSize 16
Set Width 1000
Set Height 500
Set Theme "Dracula"
Set TypingSpeed 18ms

Type "otcat-mockplc --addr 127.0.0.1:15020 &"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15020 --read holding:100 --raw-address --json"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15020 --read holding:200:2 --raw-address --type float32 --json"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15020 --watch holding:0 --raw-address --interval 500ms --count 5 --json"
Enter
Sleep 4s
