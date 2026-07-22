#!/bin/bash
cd "$(dirname "$0")"
source ./_typing.sh

run 'otcat-mockplc --addr 127.0.0.1:15022 &' \
    'nohup timeout 20 otcat-mockplc --addr 127.0.0.1:15022 > /dev/null 2>&1 < /dev/null &'
sleep 0.6

comment "ndjson pipes straight into jq"
run 'otc --modbus 127.0.0.1:15022 --read holding:200:2 --raw-address --type float32 --json | jq .value'

comment "--raw pipes straight into awk -- no parsing glue code, ever"
run 'otc --modbus 127.0.0.1:15022 --watch holding:0 --raw-address --interval 300ms --count 6 --raw | awk "{printf \"%.2f%%\n\", \$1/100}"'

comment "or grep -o, to pull one field out of every line"
run 'otc --modbus 127.0.0.1:15022 --watch holding:0 --raw-address --interval 300ms --count 6 --json | grep -o "\"value\":[0-9]*"'
