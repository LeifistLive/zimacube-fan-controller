# ZimaCube Fan Controller

Docker-based fan controller for the ZimaCube backplane controller on I²C bus `0`, address `0x69`.

## Features

- Temperature-based fan curve
- Automatic boost during parity check, parity sync, rebuild and clear
- Emergency temperature mode
- Hysteresis
- Persistent manual override
- `fanctl` commands
- JSON status and health API
- Docker healthcheck
- I²C retries and locking
- No privileged container
- GitHub Actions for ShellCheck and Docker build

## Unraid requirement

Add before `emhttp` in `/boot/config/go`:

```bash
modprobe i2c-dev
modprobe i2c-i801
```

## Commands

```bash
docker exec zimacube-fan-controller fanctl status
docker exec zimacube-fan-controller fanctl 75
docker exec zimacube-fan-controller fanctl auto
docker exec zimacube-fan-controller fanctl emergency
```

## API

```text
http://192.168.178.123:8086/status
http://192.168.178.123:8086/health
```

Disable with `API_ENABLED=false` and remove `ports:`.

## Fan curve

```yaml
FAN_CURVE: "0:60,36:65,40:75,43:85,46:95,48:100"
```
