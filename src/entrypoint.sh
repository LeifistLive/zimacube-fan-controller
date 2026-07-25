#!/bin/sh
set -eu

BUS="${I2C_BUS:-0}"
ADDRESS="${I2C_ADDRESS:-0x69}"
FAN_CURVE="${FAN_CURVE:-0:60,36:65,40:75,43:85,46:95,48:100}"
ARRAY_BOOST_PERCENT="${ARRAY_BOOST_PERCENT:-100}"
EMERGENCY_TEMP="${EMERGENCY_TEMP:-52}"
EMERGENCY_PERCENT="${EMERGENCY_PERCENT:-100}"
HYSTERESIS_C="${HYSTERESIS_C:-2}"
CHECK_INTERVAL="${CHECK_INTERVAL:-15}"
I2C_RETRIES="${I2C_RETRIES:-3}"
API_ENABLED="${API_ENABLED:-true}"
API_PORT="${API_PORT:-8080}"

VAR_INI="/var/local/emhttp/var.ini"
DISKS_INI="/var/local/emhttp/disks.ini"
RUNTIME_DIR="/run/fan-controller"
PERSIST_DIR="/var/lib/zimacube-fan-controller"
STATE_FILE="${RUNTIME_DIR}/state"
STATE_JSON="${RUNTIME_DIR}/state.json"
HEALTH_FILE="${RUNTIME_DIR}/healthy"
LOCK_FILE="${RUNTIME_DIR}/i2c.lock"
COMMAND_FILE="${RUNTIME_DIR}/command"
OVERRIDE_FILE="${PERSIST_DIR}/manual_override"

LAST_PERCENT=""
LAST_TEMP=0
RUNNING=1

mkdir -p "$RUNTIME_DIR" "$PERSIST_DIR"

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }

is_integer() {
    case "${1:-}" in ''|*[!0-9]*) return 1 ;; *) return 0 ;; esac
}

validate_percent() {
    is_integer "$1" && [ "$1" -ge 1 ] && [ "$1" -le 100 ]
}

validate_config() {
    validate_percent "$ARRAY_BOOST_PERCENT" || exit 1
    validate_percent "$EMERGENCY_PERCENT" || exit 1
    for value in "$EMERGENCY_TEMP" "$HYSTERESIS_C" "$CHECK_INTERVAL" "$I2C_RETRIES" "$API_PORT"; do
        is_integer "$value" || exit 1
    done
    old_ifs="$IFS"; IFS=','
    for point in $FAN_CURVE; do
        temp="${point%%:*}"
        percent="${point##*:}"
        is_integer "$temp" || exit 1
        validate_percent "$percent" || exit 1
    done
    IFS="$old_ifs"
}

controller_present() {
    [ -e "/dev/i2c-${BUS}" ] || return 1
    output="$(i2cdetect -y "$BUS" "$ADDRESS" "$ADDRESS" 2>/dev/null || true)"
    printf '%s\n' "$output" | grep -qiE "(^|[[:space:]])(${ADDRESS#0x}|UU)([[:space:]]|$)"
}

get_max_disk_temp() {
    [ -r "$DISKS_INI" ] || { echo 0; return; }
    awk -F= '/^temp=/ {
        value=$2; gsub(/"/, "", value)
        if (value ~ /^[0-9]+$/ && value > maximum) maximum=value
    } END { print maximum+0 }' "$DISKS_INI"
}

get_array_operation() {
    [ -r "$VAR_INI" ] || { echo "none"; return; }

    action="$(awk -F= '$1 == "mdResyncAction" {
        value=$2; gsub(/"/, "", value); print value; exit
    }' "$VAR_INI")"

    resync="$(awk -F= '$1 == "mdResync" {
        value=$2; gsub(/"/, "", value); print value; exit
    }' "$VAR_INI")"

    case "$resync" in ''|*[!0-9]*) resync=0 ;; esac

    if [ "$resync" -gt 0 ]; then
        case "$action" in
            check*) echo "parity-check" ;;
            reconstruct*) echo "rebuild" ;;
            resync*) echo "parity-sync" ;;
            clear*) echo "clear" ;;
            *) echo "${action:-array-operation}" ;;
        esac
    else
        echo "none"
    fi
}

curve_percent_for_temp() {
    temp="$1"; selected=1
    old_ifs="$IFS"; IFS=','
    for point in $FAN_CURVE; do
        threshold="${point%%:*}"
        percent="${point##*:}"
        [ "$temp" -ge "$threshold" ] && selected="$percent"
    done
    IFS="$old_ifs"
    echo "$selected"
}

apply_hysteresis() {
    requested="$1"; temp="$2"

    [ -z "$LAST_PERCENT" ] && { echo "$requested"; return; }
    [ "$requested" -ge "$LAST_PERCENT" ] && { echo "$requested"; return; }

    if [ "$temp" -le $((LAST_TEMP - HYSTERESIS_C)) ]; then
        echo "$requested"
    else
        echo "$LAST_PERCENT"
    fi
}

write_state() {
    mode="$1"; percent="$2"; temp="$3"; operation="$4"; reason="$5"
    updated="$(date '+%Y-%m-%d %H:%M:%S')"

    {
        echo "mode=${mode}"
        echo "applied_percent=${percent}"
        echo "maximum_disk_temperature=${temp}"
        echo "array_operation=${operation}"
        echo "reason=${reason}"
        echo "updated=${updated}"
    } > "${STATE_FILE}.tmp"
    mv "${STATE_FILE}.tmp" "$STATE_FILE"

    safe_reason="$(printf '%s' "$reason" | sed 's/\\/\\\\/g; s/"/\\"/g')"
    printf '{"mode":"%s","fan_percent":%s,"maximum_disk_temperature":%s,"array_operation":"%s","reason":"%s","updated":"%s","controller_online":true}\n' \
        "$mode" "$percent" "$temp" "$operation" "$safe_reason" "$updated" > "${STATE_JSON}.tmp"
    mv "${STATE_JSON}.tmp" "$STATE_JSON"
}

set_fan_speed() {
    percent="$1"; mode="$2"; temp="$3"; operation="$4"; reason="$5"

    if [ "$percent" = "$LAST_PERCENT" ]; then
        LAST_TEMP="$temp"
        write_state "$mode" "$percent" "$temp" "$operation" "$reason"
        touch "$HEALTH_FILE"
        return 0
    fi

    hex="$(printf '0x%02x' "$percent")"
    attempt=1
    while [ "$attempt" -le "$I2C_RETRIES" ]; do
        log "[INFO] Setze Lüfter auf ${percent}% – ${reason} (Versuch ${attempt}/${I2C_RETRIES})"
        if flock "$LOCK_FILE" i2cset -f -y "$BUS" "$ADDRESS" 0x04 \
            0x01 "$hex" 0x00 0x00 0x00 0x00 0x01 0x00 i; then
            LAST_PERCENT="$percent"
            LAST_TEMP="$temp"
            touch "$HEALTH_FILE"
            write_state "$mode" "$percent" "$temp" "$operation" "$reason"
            log "[OK] Lüftergeschwindigkeit erfolgreich gesetzt."
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    rm -f "$HEALTH_FILE"
    log "[ERROR] I²C-Befehl fehlgeschlagen."
    return 1
}

process_command() {
    [ -s "$COMMAND_FILE" ] || return 1
    command="$(cat "$COMMAND_FILE" 2>/dev/null || true)"
    rm -f "$COMMAND_FILE"

    case "$command" in
        auto)
            rm -f "$OVERRIDE_FILE"; LAST_PERCENT=""
            log "[INFO] Automatik aktiviert."
            ;;
        emergency)
            printf '%s\n' "$EMERGENCY_PERCENT" > "$OVERRIDE_FILE"; LAST_PERCENT=""
            log "[WARN] Notfallmodus aktiviert."
            ;;
        *)
            if validate_percent "$command"; then
                printf '%s\n' "$command" > "$OVERRIDE_FILE"; LAST_PERCENT=""
                log "[INFO] Manueller Wert: ${command}%."
            fi
            ;;
    esac
    return 0
}

determine_target() {
    MAX_TEMP="$(get_max_disk_temp)"
    ARRAY_OPERATION="$(get_array_operation)"
    MODE="automatic"

    if [ -f "$OVERRIDE_FILE" ]; then
        manual="$(cat "$OVERRIDE_FILE" 2>/dev/null || true)"
        if validate_percent "$manual"; then
            TARGET_PERCENT="$manual"; MODE="manual"; REASON="manuelle Vorgabe"; return
        fi
        rm -f "$OVERRIDE_FILE"
    fi

    if [ "$MAX_TEMP" -ge "$EMERGENCY_TEMP" ]; then
        TARGET_PERCENT="$EMERGENCY_PERCENT"; MODE="emergency"
        REASON="Notfalltemperatur erreicht: ${MAX_TEMP} °C"; return
    fi

    curve_target="$(curve_percent_for_temp "$MAX_TEMP")"
    TARGET_PERCENT="$(apply_hysteresis "$curve_target" "$MAX_TEMP")"
    REASON="Temperaturkurve: höchste HDD-Temperatur ${MAX_TEMP} °C"

    if [ "$ARRAY_OPERATION" != "none" ] && [ "$TARGET_PERCENT" -lt "$ARRAY_BOOST_PERCENT" ]; then
        TARGET_PERCENT="$ARRAY_BOOST_PERCENT"
        REASON="Array-Operation läuft: ${ARRAY_OPERATION}"
    fi
}

shutdown() { RUNNING=0; log "[INFO] Container wird beendet."; }
trap shutdown TERM INT

validate_config

log "[INFO] ZimaCube Fan Controller startet."
log "[INFO] Bus ${BUS}, Adresse ${ADDRESS}"
log "[INFO] Lüfterkurve: ${FAN_CURVE}"
log "[INFO] Array-Boost: ${ARRAY_BOOST_PERCENT}%"
log "[INFO] Notfall: ${EMERGENCY_PERCENT}% ab ${EMERGENCY_TEMP} °C"

attempt=1
while ! controller_present; do
    [ "$attempt" -ge 60 ] && { log "[ERROR] Controller nicht gefunden."; exit 1; }
    log "[INFO] Warte auf I²C-Controller (${attempt}/60) ..."
    attempt=$((attempt + 1))
    sleep 2
done

log "[OK] I²C-Controller gefunden."

if [ "$API_ENABLED" = "true" ]; then
    /usr/local/bin/api.sh "$API_PORT" &
    API_PID=$!
    log "[INFO] Status-API läuft auf Port ${API_PORT}."
else
    API_PID=""
fi

while [ "$RUNNING" -eq 1 ]; do
    process_command || true
    determine_target
    set_fan_speed "$TARGET_PERCENT" "$MODE" "$MAX_TEMP" "$ARRAY_OPERATION" "$REASON" || true

    elapsed=0
    while [ "$elapsed" -lt "$CHECK_INTERVAL" ] && [ "$RUNNING" -eq 1 ]; do
        [ -s "$COMMAND_FILE" ] && break
        sleep 1
        elapsed=$((elapsed + 1))
    done
done

[ -n "${API_PID:-}" ] && kill "$API_PID" 2>/dev/null || true
exit 0
