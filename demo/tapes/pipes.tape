# See quickstart.tape for why this ships as source-of-truth VHS
# alongside a pre-generated asciinema+agg GIF rather than a VHS-rendered
# one.

Output gifs/pipes_vhs.gif

Set FontSize 16
Set Width 1000
Set Height 500
Set Theme "Dracula"
Set TypingSpeed 18ms

Type "otcat-mockplc --addr 127.0.0.1:15022 &"
Enter
Sleep 1.5s

Type `otc --modbus 127.0.0.1:15022 --read holding:200:2 --raw-address --type float32 --json | jq .value`
Enter
Sleep 1.5s

Type `otc --modbus 127.0.0.1:15022 --watch holding:0 --raw-address --interval 300ms --count 6 --raw | awk '{printf "%.2f%%\n", $1/100}'`
Enter
Sleep 3s

Type `otc --modbus 127.0.0.1:15022 --watch holding:0 --raw-address --interval 300ms --count 6 --json | grep -o "\"value\":[0-9]*"`
Enter
Sleep 3s
