# API

## Login

- `GET /login` – login page (public)
- `POST /login` – expects `Authorization: Basic base64(user:password)`,
  sets the `zimafan_session` session cookie on success (24h sliding)
- `POST /logout` – ends the session

If `ADMIN_PASSWORD` is set, every route except `GET /login`, `POST /login`
and `GET /api/health` requires a valid session; without a session this
returns 401 (a 302 redirect to `/login` for `GET /`). If `ADMIN_PASSWORD` is
empty, login is disabled and everything is openly reachable.

`scripts/fanctl` (for `docker exec`) automatically logs in with
`ADMIN_USER`/`ADMIN_PASSWORD` from the container environment and caches the
session in `/tmp/fanctl-session`.

`POST /login` is limited to one attempt every 2 seconds per client IP (not
globally), so a single client can never block login for everyone else. The
session cookie automatically gets `Secure` as soon as the request arrives
over TLS (directly, or via `X-Forwarded-Proto: https` behind a reverse
proxy) – it stays usable unchanged over plain HTTP on the LAN.

## Read

- `GET /api/status`
- `GET /api/health` (always reachable without login, for healthcheck/monitoring)
- `GET /api/history?limit=288`
- `GET /api/events?limit=100`
- `GET /api/config`

## Write

- `POST /api/fan/{1-100}`
- `POST /api/mode/auto`
- `POST /api/mode/emergency`
- `POST /api/profile/{name}`
- `POST /api/test/{1-100}`
- `POST /api/config`
- `POST /api/events/clear`

Independently of the session, requests with a foreign `Origin` or
`Sec-Fetch-Site: cross-site` are rejected with 403.

Write endpoints are limited per category (override: `fan`/`mode/auto`/
`mode/emergency`, `profile`, `config`, `test`, `events`) to one write per
second; a violation returns 429. `POST /api/test/{percent}` additionally
allows only one active test at a time (409 if one is already running) and
then has its own 5-second cooldown (429 during that time).

`POST /api/events/clear` permanently deletes every recorded event, then
immediately records one new event noting that the log was cleared — so the
list is never left looking broken right after a successful clear, and there
is a small audit trail of when it happened.

## Status Fields

| Field | Meaning |
| --- | --- |
| `mode` | `automatic`, `manual`, `emergency`, `array-boost`, `failsafe` |
| `target_percent` | Percentage requested by the control logic (can change even if the write fails) |
| `last_applied_percent` | Value last actually written successfully via `i2cset` |
| `fan_percent` | Deprecated alias for `target_percent`, kept for compatibility |
| `feedback_available` | Always `false`: the controller has no RPM/PWM feedback, `last_applied_percent` is only the last value written, not a hardware confirmation |
| `temperature_valid` | `false` if `disks.ini` could not be read |
| `disks_reporting` | Number of HDD sections with `temp=` in `disks.ini` (cache/flash devices do not count) |
| `disks` | One entry per HDD: `name`, `temperature`, `valid` (`false` = standby/unreadable) |
| `controller_online` | Result of the last reachability check |
| `last_write_successful` | Result of the last `i2cset` |

`GET /api/health` returns 503 while the controller is offline, the last
write failed, the temperature is unknown, the active profile is missing
(`config`), the last persistence attempt failed (`storage`), or the status
is stale. Alongside the overall `healthy`, the response returns the
individual checks `status` (`"healthy"`/`"unhealthy"`), `controller`,
`config`, `last_write_successful` and `storage` separately, so monitoring
(e.g. Uptime Kuma) can identify the cause.

## Writing Configuration

`POST /api/config` expects the complete configuration. Profiles are
validated: curve points must be between 1 and 100 percent, must not contain
duplicate temperatures, and must not fall as temperature rises. Unknown JSON
fields and data after the JSON object are rejected. Rejected configurations
change nothing.

`config_version` identifies the configuration format (currently `1`). If the
field is missing (a configuration from before this change), version 1 is
silently assumed; a version higher than this binary supports is rejected.

A fan test (`POST /api/test/{percent}`) is rejected with 409 if the value
would fall below the current emergency, failsafe, or array-boost floor; the
response includes `minimum_percent`.
