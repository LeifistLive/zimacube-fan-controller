#!/bin/sh
set -eu

HEALTH_FILE="/run/fan-controller/healthy"
STATE_FILE="/run/fan-controller/state"

[ -f "$HEALTH_FILE" ]
[ -s "$STATE_FILE" ]

now="$(date +%s)"
modified="$(stat -c %Y "$HEALTH_FILE")"
age=$((now - modified))

[ "$age" -le 120 ]
