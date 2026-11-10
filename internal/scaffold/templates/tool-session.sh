#!/usr/bin/env bash

if [[ -z ${BMCSHELL_TARGET_IP:-} ]]; then
    printf '%s\n' 'Select a BMC with bmc use first.' >&2
    exit 1
fi

printf 'Current BMC: %s\n' "$BMCSHELL_TARGET_IP"
if [[ -n ${BMCSHELL_TARGET_MODEL:-} ]]; then
    printf 'Model: %s\n' "$BMCSHELL_TARGET_MODEL"
fi
