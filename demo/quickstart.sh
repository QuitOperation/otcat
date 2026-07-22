#!/bin/bash
cd "$(dirname "$0")"
source ./_typing.sh

comment "no PLC on your desk? otcat ships a real one, in software"
run 'otcat-mockplc --addr 127.0.0.1:15020 &' \
    'nohup timeout 20 otcat-mockplc --addr 127.0.0.1:15020 > /dev/null 2>&1 < /dev/null &'
sleep 0.6

comment "one register, decoded and typed"
run 'otc --modbus 127.0.0.1:15020 --read holding:100 --raw-address --json'

comment "a 32-bit float spanning two registers"
run 'otc --modbus 127.0.0.1:15020 --read holding:200:2 --raw-address --type float32 --json'

comment "watch it change in real time -- a live PI-controlled tank level"
run 'otc --modbus 127.0.0.1:15020 --watch holding:0 --raw-address --interval 500ms --count 5 --json'
