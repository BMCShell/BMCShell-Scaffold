#!/usr/bin/env bash

if [[ -z ${BMCSHELL_TARGET_IP:-} ]]; then
    printf '%s\n' '请先用 bmc use 选定 BMC。' >&2
    exit 1
fi

printf '当前 BMC：%s\n' "$BMCSHELL_TARGET_IP"
if [[ -n ${BMCSHELL_TARGET_MODEL:-} ]]; then
    printf '型号：%s\n' "$BMCSHELL_TARGET_MODEL"
fi
