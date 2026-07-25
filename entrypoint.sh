#!/bin/sh
set -eu

BUS="${I2C_BUS:-0}"
ADDRESS="${I2C_ADDRESS:-0x69}"

NORMAL_PERCENT="${NORMAL_PERCENT:-90}"
HIGH_PERCENT="${HIGH_PERCENT:-100}"

HIGH_TEMP="${HIGH_TEMP:-47}"
LOW_TEMP="${LOW_TEMP:-44}"

CHECK_INTERVAL="${CHECK_INTERVAL:-15}"
I2C_RETRIES="${I2C_RETRIES:-3}"

VAR_INI="/var/local/emhttp/var.ini"
DISKS_INI="/var/local/emhttp/disks.ini"

RUNTIME_DIR="/run/fan-controller"
OVERRIDE_FILE="${RUNTIME_DIR}/override"
COMMAND_FILE="${RUNTIME_DIR}/command"
STATE_FILE="${RUNTIME_DIR}/state"
HEALTH_FILE="${RUNTIME_DIR}/healthy"
LOCK_FILE="${RUNTIME_DIR}/i2c.lock"

LAST_PERCENT=""
TEMPERATURE_HIGH=0
RUNNING=1

mkdir -p "$RUNTIME_DIR"

log() {
    printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

is_integer() {
    case "${1:-}" in
        ''|*[!0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

validate_percent() {
    is_integer "$1" &&
        [ "$1" -ge 1 ] &&
        [ "$1" -le 100 ]
}

validate_config() {
    for value in "$NORMAL_PERCENT" "$HIGH_PERCENT"; do
        validate_percent "$value" || {
            log "[ERROR] Ungültiger Prozentwert: $value"
            exit 1
        }
    done

    for value in "$HIGH_TEMP" "$LOW_TEMP" "$CHECK_INTERVAL" "$I2C_RETRIES"; do
        is_integer "$value" || {
            log "[ERROR] Ungültiger Zahlenwert: $value"
            exit 1
        }
    done

    if [ "$LOW_TEMP" -ge "$HIGH_TEMP" ]; then
        log "[ERROR] LOW_TEMP muss kleiner als HIGH_TEMP sein."
        exit 1
    fi
}

controller_present() {
    [ -e "/dev/i2c-${BUS}" ] || return 1

    output="$(
        i2cdetect -y "$BUS" "$ADDRESS" "$ADDRESS" 2>/dev/null ||
            true
    )"

    printf '%s\n' "$output" |
        grep -qiE "(^|[[:space:]])(${ADDRESS#0x}|UU)([[:space:]]|$)"
}

set_fan_speed() {
    percent="$1"
    reason="$2"

    validate_percent "$percent" || {
        log "[ERROR] Ungültige Lüftergeschwindigkeit: $percent"
        return 1
    }

    if [ "$percent" = "$LAST_PERCENT" ]; then
        write_state "$percent" "$reason"
        return 0
    fi

    hex="$(printf '0x%02x' "$percent")"
    attempt=1

    while [ "$attempt" -le "$I2C_RETRIES" ]; do
        log "[INFO] Setze Lüfter auf ${percent}% – ${reason} (Versuch ${attempt}/${I2C_RETRIES})"

        if flock "$LOCK_FILE" \
            i2cset -f -y "$BUS" "$ADDRESS" 0x04 \
            0x01 "$hex" 0x00 0x00 0x00 0x00 0x01 0x00 i; then

            LAST_PERCENT="$percent"
            touch "$HEALTH_FILE"
            write_state "$percent" "$reason"
            log "[OK] Lüftergeschwindigkeit erfolgreich gesetzt."
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 1
    done

    rm -f "$HEALTH_FILE"
    log "[ERROR] I²C-Befehl nach ${I2C_RETRIES} Versuchen fehlgeschlagen."
    return 1
}

get_max_disk_temp() {
    [ -r "$DISKS_INI" ] || {
        echo 0
        return
    }

    awk -F= '
        /^temp=/ {
            value=$2
            gsub(/"/, "", value)

            if (value ~ /^[0-9]+$/ && value > maximum) {
                maximum=value
            }
        }
        END {
            print maximum+0
        }
    ' "$DISKS_INI"
}

get_array_operation() {
    [ -r "$VAR_INI" ] || {
        echo "none"
        return
    }

    action="$(
        awk -F= '
            $1 == "mdResyncAction" {
                value=$2
                gsub(/"/, "", value)
                print value
                exit
            }
        ' "$VAR_INI"
    )"

    resync="$(
        awk -F= '
            $1 == "mdResync" {
                value=$2
                gsub(/"/, "", value)
                print value
                exit
            }
        ' "$VAR_INI"
    )"

    case "$resync" in
        ''|*[!0-9]*) resync=0 ;;
    esac

    if [ "$resync" -gt 0 ]; then
        printf '%s\n' "${action:-array operation}"
    else
        echo "none"
    fi
}

write_state() {
    percent="$1"
    reason="$2"
    max_temp="${3:-$(get_max_disk_temp)}"
    operation="${4:-$(get_array_operation)}"

    mode="automatic"
    [ -f "$OVERRIDE_FILE" ] && mode="manual"

    temporary="${STATE_FILE}.tmp"

    {
        echo "mode=${mode}"
        echo "applied_percent=${percent}"
        echo "maximum_disk_temperature=${max_temp}"
        echo "array_operation=${operation}"
        echo "reason=${reason}"
        echo "updated=$(date '+%Y-%m-%d %H:%M:%S')"
    } > "$temporary"

    mv "$temporary" "$STATE_FILE"
}

process_command() {
    [ -s "$COMMAND_FILE" ] || return 1

    command="$(cat "$COMMAND_FILE" 2>/dev/null || true)"
    rm -f "$COMMAND_FILE"

    case "$command" in
        auto)
            rm -f "$OVERRIDE_FILE"
            log "[INFO] Automatische Regelung aktiviert."
            LAST_PERCENT=""
            ;;

        status)
            ;;

        *)
            if validate_percent "$command"; then
                printf '%s\n' "$command" > "$OVERRIDE_FILE"
                log "[INFO] Manueller Wert angefordert: ${command}%"
                LAST_PERCENT=""
            else
                log "[WARN] Ungültiger Befehl ignoriert: $command"
            fi
            ;;
    esac

    return 0
}

determine_target() {
    MAX_TEMP="$(get_max_disk_temp)"
    ARRAY_OPERATION="$(get_array_operation)"

    if [ -f "$OVERRIDE_FILE" ]; then
        MANUAL_PERCENT="$(cat "$OVERRIDE_FILE" 2>/dev/null || true)"

        if validate_percent "$MANUAL_PERCENT"; then
            TARGET_PERCENT="$MANUAL_PERCENT"
            REASON="manuelle Vorgabe"
            return
        fi

        rm -f "$OVERRIDE_FILE"
    fi

    if [ "$ARRAY_OPERATION" != "none" ]; then
        TARGET_PERCENT="$HIGH_PERCENT"
        REASON="Array-Operation läuft: ${ARRAY_OPERATION}"
        return
    fi

    # Hysterese:
    # Ab HIGH_TEMP auf hohe Drehzahl.
    # Erst unter/gleich LOW_TEMP wieder zurück.
    if [ "$MAX_TEMP" -ge "$HIGH_TEMP" ]; then
        TEMPERATURE_HIGH=1
    elif [ "$MAX_TEMP" -le "$LOW_TEMP" ]; then
        TEMPERATURE_HIGH=0
    fi

    if [ "$TEMPERATURE_HIGH" -eq 1 ]; then
        TARGET_PERCENT="$HIGH_PERCENT"
        REASON="HDD-Temperatur ${MAX_TEMP} °C, Hochtemperaturmodus"
    else
        TARGET_PERCENT="$NORMAL_PERCENT"
        REASON="Normalbetrieb, höchste HDD-Temperatur ${MAX_TEMP} °C"
    fi
}

shutdown() {
    RUNNING=0
    log "[INFO] Container wird beendet."
}

trap shutdown TERM INT

validate_config

log "[INFO] ZimaCube Fan Controller startet."
log "[INFO] Bus ${BUS}, Adresse ${ADDRESS}"
log "[INFO] Normal ${NORMAL_PERCENT}%, Hochlast ${HIGH_PERCENT}%"
log "[INFO] Temperatur: hoch ab ${HIGH_TEMP} °C, zurück ab ${LOW_TEMP} °C"
log "[INFO] Prüfintervall: ${CHECK_INTERVAL} Sekunden"

attempt=1
while ! controller_present; do
    if [ "$attempt" -ge 60 ]; then
        log "[ERROR] Controller ${ADDRESS} auf Bus ${BUS} nicht gefunden."
        exit 1
    fi

    log "[INFO] Warte auf I²C-Controller (${attempt}/60) ..."
    attempt=$((attempt + 1))
    sleep 2
done

log "[OK] I²C-Controller gefunden."

while [ "$RUNNING" -eq 1 ]; do
    process_command || true
    determine_target

    set_fan_speed \
        "$TARGET_PERCENT" \
        "$REASON" || true

    elapsed=0
    while [ "$elapsed" -lt "$CHECK_INTERVAL" ] &&
          [ "$RUNNING" -eq 1 ]; do

        # Befehle reagieren dadurch innerhalb maximal einer Sekunde.
        if [ -s "$COMMAND_FILE" ]; then
            break
        fi

        sleep 1
        elapsed=$((elapsed + 1))
    done
done

exit 0