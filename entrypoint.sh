#!/bin/sh
set -eu

BUS="${I2C_BUS:-0}"
ADDRESS="${I2C_ADDRESS:-0x69}"
FAN_PERCENT="${FAN_PERCENT:-90}"

if [ "$FAN_PERCENT" -lt 1 ] || [ "$FAN_PERCENT" -gt 100 ]; then
    echo "[ERROR] FAN_PERCENT must be between 1 and 100"
    exit 1
fi

FAN_HEX=$(printf "0x%02x" "$FAN_PERCENT")

echo "[INFO] Waiting for /dev/i2c-${BUS}..."

for i in $(seq 1 30); do
    if [ -e "/dev/i2c-${BUS}" ]; then
        echo "[INFO] Setting fan speed to ${FAN_PERCENT}%"

        i2cset -f -y "$BUS" "$ADDRESS" 0x04 \
            0x01 "$FAN_HEX" 0x00 0x00 0x00 0x00 0x01 0x00 i

        echo "[INFO] Fan speed successfully applied."
        break
    fi

    sleep 2
done

echo "[INFO] Container is running."
exec tail -f /dev/null