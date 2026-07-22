#!/bin/bash
# Shared helper for the demo/*.sh recording scripts. Every command shown
# in the resulting GIFs is real and actually executed against a real
# (mock) Modbus TCP server -- only the character-by-character "typing"
# before each command is simulated, for deterministic, reproducible
# output. This is the same convention VHS's own `Type` command produces
# when scripted rather than hand-typed.
GREEN='\033[1;32m'
RESET='\033[0m'

type_line() {
    printf "${GREEN}\$ ${RESET}"
    local text="$1"
    local i
    for ((i = 0; i < ${#text}; i++)); do
        printf "%s" "${text:$i:1}"
        sleep 0.018
    done
    printf "\n"
    sleep 0.25
}

# run CMD_TO_DISPLAY [ACTUAL_CMD_IF_DIFFERENT]
run() {
    type_line "$1"
    eval "${2:-$1}"
    sleep 0.9
}

comment() {
    printf "\033[2m# %s\033[0m\n" "$1"
    sleep 0.6
}
