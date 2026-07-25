#!/bin/sh
set -eu

BUS="${I2C_BUS:-0}"
ADDRESS="${I2C_ADDRESS:-0x69}"

NORMAL_PERCENT="${NORMAL_PERCENT:-90}"
HIGH_PERCENT="${HIGH_PERCENT:-100}"
HIGH_TEMP="${HIGH_TEMP:-47}"
CHECK_INTERVAL="${CHECK_INTERVAL:-30}"

VAR_INI="/var/local/emhttp/var.ini"
DISKS_INI="/var/local/emhttp/disks.ini"

OVERRIDE_FILE="/tmp/fan-override"
STATE_FILE="/tmp/fan-state"

LAST_PERCENT=""

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

validate_percent() {
    value="$1"

    case "$value" in
        ''|*[!0-9]*)
            return 1
            ;;
    esac

    [ "$value" -ge 1 ] && [ "$value" -le 100 ]
}

set_fan_speed() {
    percent="$1"
    reason="$2"

    if ! validate_percent "$percent"; then
        log "[ERROR] Invalid fan percentage: $percent"
        return 1
    fi

    # Nicht immer wieder denselben Befehl senden.
    if [ "$percent" = "$LAST_PERCENT" ]; then
        return 0
    fi

    hex="$(printf '0x%02x' "$percent")"

    log "[INFO] Setting fan speed to ${percent}% (${reason})"

    if i2cset -f -y "$BUS" "$ADDRESS" 0x04 \
        0x01 "$hex" 0x00 0x00 0x00 0x00 0x01 0x00 i; then

        LAST_PERCENT="$percent"

        {
            echo "Applied speed: ${percent}%"
            echo "Reason: ${reason}"
            echo "Updated: $(date '+%Y-%m-%d %H:%M:%S')"
        } > "$STATE_FILE"

        log "[OK] Fan speed applied successfully."
        return 0
    fi

    log "[ERROR] Failed to communicate with I2C controller."
    return 1
}

get_md_resync() {
    if [ ! -r "$VAR_INI" ]; then
        echo "0"
        return
    fi

    awk -F= '
        $1 == "mdResync" {
            gsub(/"/, "", $2)
            print $2
            found=1
            exit
        }
        END {
            if (!found) print "0"
        }
    ' "$VAR_INI"
}

get_max_disk_temp() {
    if [ ! -r "$DISKS_INI" ]; then
        echo "0"
        return
    fi

    # Es werden nur numerische temp="-Werte ausgewertet.
    # "*" bei heruntergefahrenen oder unbekannten Laufwerken wird ignoriert.
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

log "[INFO] ZimaCube fan controller starting."
log "[INFO] Bus ${BUS}, address ${ADDRESS}"
log "[INFO] Normal ${NORMAL_PERCENT}%, high ${HIGH_PERCENT}%"
log "[INFO] High-temperature threshold: ${HIGH_TEMP} °C"

attempt=1
while [ ! -e "/dev/i2c-${BUS}" ]; do
    if [ "$attempt" -ge 60 ]; then
        log "[ERROR] /dev/i2c-${BUS} was not found."
        exit 1
    fi

    log "[INFO] Waiting for /dev/i2c-${BUS} (${attempt}/60)..."
    attempt=$((attempt + 1))
    sleep 2
done

log "[OK] /dev/i2c-${BUS} is available."

while true; do
    TARGET_PERCENT="$NORMAL_PERCENT"
    REASON="normal operation"

    # Manuelle Einstellung hat höchste Priorität.
    if [ -f "$OVERRIDE_FILE" ]; then
        OVERRIDE="$(cat "$OVERRIDE_FILE" 2>/dev/null || true)"

        if validate_percent "$OVERRIDE"; then
            TARGET_PERCENT="$OVERRIDE"
            REASON="manual override"
        else
            log "[WARN] Invalid manual override removed."
            rm -f "$OVERRIDE_FILE"
        fi
    else
        MD_RESYNC="$(get_md_resync)"
        MAX_TEMP="$(get_max_disk_temp)"

        case "$MD_RESYNC" in
            ''|*[!0-9]*)
                MD_RESYNC=0
                ;;
        esac

        case "$MAX_TEMP" in
            ''|*[!0-9]*)
                MAX_TEMP=0
                ;;
        esac

        if [ "$MD_RESYNC" -gt 0 ]; then
            TARGET_PERCENT="$HIGH_PERCENT"
            REASON="parity check, sync, rebuild or clear running"
        elif [ "$MAX_TEMP" -ge "$HIGH_TEMP" ]; then
            TARGET_PERCENT="$HIGH_PERCENT"
            REASON="highest disk temperature ${MAX_TEMP} °C"
        else
            REASON="normal operation; highest disk temperature ${MAX_TEMP} °C"
        fi
    fi

    set_fan_speed "$TARGET_PERCENT" "$REASON" || true
    sleep "$CHECK_INTERVAL"
done