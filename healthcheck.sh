#!/bin/sh
set -eu

HEALTH_FILE="/run/fan-controller/healthy"
STATE_FILE="/run/fan-controller/state"

[ -f "$HEALTH_FILE" ]
[ -s "$STATE_FILE" ]

# Letzter erfolgreicher I²C-Zugriff darf höchstens 5 Minuten alt sein.
now="$(date +%s)"
modified="$(stat -c %Y "$HEALTH_FILE")"
age=$((now - modified))

[ "$age" -le 300 ]