# ZimaCube Fan Controller

A small Go service for controlling the ZimaCube backplane fan controller over I²C.

## Features

- Temperature-based fan curve
- Automatic boost during parity check, parity sync, rebuild and clear
- Emergency temperature protection
- Hysteresis
- Persistent manual override
- REST API
- Built-in web interface
- Docker healthcheck
- I²C retries and timeouts
- Read-only container filesystem
- No privileged mode
- Optional API token for write operations
- GitHub Actions for tests, Docker build and GHCR publishing
- No Prometheus integration

## Unraid requirement

Add these lines to `/boot/config/go` before `emhttp`:

```bash
modprobe i2c-dev
modprobe i2c-i801
```

## Portainer

Use the repository as a Git stack and build locally from `docker-compose.yml`.

Do not enable image pulling for the local image `zimacube-fan-controller:local`.

## Web interface

```text
http://192.168.178.123:8086/
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
GET  /api/status
GET  /api/health
POST /api/fan/{1-100}
POST /api/mode/auto
POST /api/mode/emergency
```

When `API_TOKEN` is set, POST requests require:

```text
X-API-Token: your-token
```

## Safety priority

1. Emergency temperature
2. Array-operation minimum boost
3. Manual mode
4. Temperature curve

A manual low speed therefore cannot suppress emergency protection or the configured array boost.
