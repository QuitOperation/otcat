#!/bin/bash
cd "$(dirname "$0")"
source ./_typing.sh

run 'otcat-mockplc --addr 127.0.0.1:15021 &' \
    'nohup timeout 20 otcat-mockplc --addr 127.0.0.1:15021 > /dev/null 2>&1 < /dev/null &'
sleep 0.6

comment "no --confirm: otcat refuses -- always, by default"
run 'otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999' \
    'echo -n | otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999'

comment "--dry-run shows the exact wire payload -- no socket ever opened"
run 'otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999 --dry-run'

comment "explicit --confirm actually sends it"
run 'otc --modbus 127.0.0.1:15021 --write holding:100 --raw-address --value 999 --confirm'

comment "verify it landed"
run 'otc --modbus 127.0.0.1:15021 --read holding:100 --raw-address --raw'
