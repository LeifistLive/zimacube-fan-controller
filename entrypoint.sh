#!/bin/sh
set -e

BUS=0
ADDRESS=0x69

# Standard = 90 %
HEX=${FAN_PWM_HEX:-0x5a}

echo "Waiting for /dev/i2c-${BUS}..."

for i in $(seq 1 30); do

    if [ -e /dev/i2c-${BUS} ]; then

        echo "Found controller."

        i2cset -f -y "$BUS" "$ADDRESS" 0x04 \
            0x01 "$HEX" 0x00 0x00 0x00 0x00 0x01 0x00 i

        echo "Fan speed applied."

        exec sleep infinity
    fi

    sleep 2

done

echo "Fan controller not found."

exit 1