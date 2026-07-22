# See quickstart.tape for why this ships as source-of-truth VHS
# alongside a pre-generated asciinema+agg GIF rather than a VHS-rendered
# one.

Output gifs/write_safety_vhs.gif

Set FontSize 16
Set Width 1000
Set Height 500
Set Theme "Dracula"
Set TypingSpeed 18ms

Type "otcat-mockplc --addr 127.0.0.1:15021 &"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999 --dry-run"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999 --confirm"
Enter
Sleep 1.5s

Type "otc --modbus 127.0.0.1:15021 --read holding:100 --raw-address --raw"
Enter
Sleep 2s
