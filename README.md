# ZimaCube Fan Controller

Docker-based fan control for the ZimaCube backplane under Unraid. A Go
service reads HDD temperatures and array status, derives a target fan speed,
and writes it to the backplane controller over `i2cset` (i2c-tools).

## Features

- Native Go service, driven over i2c-tools
- Web dashboard with live status, history charts (hover for exact values),
  per-HDD temperature tiles, and an event log
- Login screen protecting the whole dashboard (session cookie, bcrypt-hashed
  admin password); `GET /api/health` stays open for the Docker healthcheck
  and external monitoring
- Light/dark theme toggle (defaults to dark)
- Editable profiles: Silent, Balanced, Performance
- A single mode switch (Automatic / Manual / Emergency) plus manual test
  buttons
- Automatic boost during parity check, rebuild, resync and clear
- Emergency protection via a temperature threshold
- Safety speed whenever the HDD temperature cannot be read
- Hysteresis against oscillation
- Separate target vs. last-applied fan percent: the API and dashboard show
  what was requested and what was last actually written over I²C, since a
  failed write must not look like a successful one; there is no RPM/PWM
  feedback from the controller
- Periodic reapply of the active PWM value (`REAPPLY_INTERVAL_SECONDS`,
  default 300s) so a backplane controller reset does not go unnoticed;
  retried after 10s instead of the full interval right after a failed write
- Only real HDDs (array data/parity members) count toward the reported
  temperature and disk count; cache/flash devices are excluded
- Persistent configuration, history and events with automatic rotation and
  a `config_version` field for future migrations
- REST API with per-category rate limiting, same-origin (CSRF) checks, and
  strict JSON parsing (unknown fields and trailing data are rejected)
- Manual fan tests are rejected if they would undercut the current
  emergency/failsafe/array-boost minimum, and only one test can run at a time
- Docker healthcheck, read-only container, no capabilities
- GitHub CI with tests, golangci-lint (pinned version) and GHCR publishing
  for linux/amd64 and linux/arm64

## Safety behavior

This order is deliberate and covered by tests:

1. The emergency temperature overrides everything else.
2. Array boost raises the speed, never lowers it.
3. If the HDD temperature cannot be read, the safety speed
   (`array_boost_percent` of the active profile) applies instead of the
   lowest curve step.
4. Manual overrides only apply within these limits.
5. Hysteresis only damps the automatic curve; it never holds a boost or
   emergency speed.

## Requirement on Unraid

Add to `/boot/config/go`, before `emhttp` starts:

```bash
modprobe i2c-dev
modprobe i2c-i801
```

## Configuration

Every value comes from environment variables, see `.env.example`. Invalid
values are logged and clamped to a valid range instead of starting the
service and surprising you later.

Set `ADMIN_PASSWORD` before the dashboard is reachable on the LAN. Without
it, login is disabled and the whole dashboard is open; with it, every route
except `GET /login`, `POST /login` and `GET /api/health` requires a session.
Cross-site requests are rejected regardless of login state.

## Portainer

Deploy as a Git repository stack. Since the image is built locally, disable
**Re-pull image**. Set the variables from `.env.example` as stack
environment variables, at minimum `ADMIN_PASSWORD`.

## Dashboard

```text
http://<unraid-ip>:8086/
```

## Commands

```bash
docker exec zimacube-fan-controller fanctl status
docker exec zimacube-fan-controller fanctl health
docker exec zimacube-fan-controller fanctl 75
docker exec zimacube-fan-controller fanctl auto
docker exec zimacube-fan-controller fanctl emergency
docker exec zimacube-fan-controller fanctl profile performance
docker exec zimacube-fan-controller fanctl test 50
```

`fanctl` logs in with `ADMIN_USER`/`ADMIN_PASSWORD` from the container
environment automatically and caches the session cookie in
`/tmp/fanctl-session`.

## Development

```bash
go test ./...
go vet ./...
gofmt -w .
golangci-lint run
```

See [docs/API.md](docs/API.md) for the full REST API and
[docs/INSTALL.md](docs/INSTALL.md) for step-by-step setup.
