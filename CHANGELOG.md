# Changelog

## 4.5.0

### Changed

- Translated the entire project to English: the web dashboard (all labels,
  buttons, status text, and messages), every Go source and test file
  (comments and log/error messages), the Dockerfile, docker-compose.yml, CI
  workflow, `scripts/fanctl`, `.env.example`, and the docs
  (`docs/API.md`, `docs/INSTALL.md`, this changelog). No functional changes;
  `go build`/`go vet`/`go test`/`gofmt` all pass unchanged.

## 4.4.0

### Fixed

- Mobile view (≤900px): `.sidebar-footer` was hidden entirely, but it held
  the only logout button – logging out was therefore unreachable on a phone.
  Logout now lives in the page header (next to refresh/theme toggle), so it
  stays visible at every screen size without horizontal scrolling. Checked
  at several breakpoints (375px, 750px, 1280px): no more horizontal overflow

### Added

- Event list: choose entries per page (10 or 25) next to the page
  navigation, the choice is remembered in the browser (localStorage)

## 4.3.1

### Fixed

- Removed a stale CI comment: `go.sum` has not been "dependency-free" since
  `golang.org/x/crypto` (bcrypt) was added; `actions/setup-go` now caches
  modules again (`cache: true`), since there is now a real basis (`go.sum`)
  for that

### Documented (no behavior change)

- The healthcheck is deliberately defined twice (`Dockerfile` for
  `docker run`/GHCR images without Compose, `docker-compose.yml` overrides
  it for Compose deployments) – both places now carry a comment pointing at
  the other, so they stay in sync
- Duplicate `go vet`/`go test` runs (once in GitHub CI, once in the
  `Dockerfile` build) and `tailLines()`, which reads the whole file instead
  of from the end: both reviewed and deliberately left unchanged, see the
  review discussion – not a real problem at the project's current size

## 4.3.0

### Critical Fix

- An `ADMIN_PASSWORD` over 72 bytes made bcrypt fail; the code claimed
  "fail closed" but in practice never set `enabled`, leaving the dashboard
  open without any login. `newAuth` now returns an error, and `New()`/the
  service refuse to start in that case

### Fixed

- `a.state.reapplyAt` was read outside the lock in `evaluate()`; it is now
  part of the same snapshot taken under `RLock()` as the other loop state
  fields
- `storageOK` was a single global flag: a successful history write could
  mask a still-unresolved config error. Now tracked separately per category
  (config/override/history/events); `storage` in `/api/health` is only
  `true` when all four last succeeded
- `Store.Remove()` now syncs the directory after deleting (the same
  guarantee `SaveJSON` already gave on writes)
- Log rotation errors (line counting, pruning) are now logged instead of
  being silently swallowed
- Sessions and per-IP login rate limits were previously only removed when
  the same (by then expired) entry was accessed again; an hourly sweep now
  actively cleans up expired sessions and stale rate limits
- The session cookie gets `Secure` as soon as the request arrived over TLS
  (directly or via `X-Forwarded-Proto: https`); unchanged over plain HTTP
- The login rate limiter was global (one client could lock out every other
  client, including the real admin); now per client IP
- Profile names and display names now have a length limit (64 characters)

## 4.2.1

### Audited

- Audit: the service never accesses the hard drives directly. `disks.ini`/
  `var.ini` are only read from Unraid's own RAM-backed file (no disk access
  regardless of the poll interval), I²C only talks to the fan controller
  chip, and `smartctl`/`hdparm` are called nowhere. The only requirement:
  `/data` must stay on non-array storage (as in the bundled `fan-data`
  Docker volume), since the history/event file is written every few
  minutes. This is now documented in
  [docker-compose.yml](docker-compose.yml), [README.md](README.md), and a
  code comment on `Config.DataDir`, to prevent accidentally redirecting it
  to an array disk.

## 4.2.0

### Added and Improved

- Browsers heavily throttle `setInterval` in background tabs (sometimes to
  once a minute or less), which left the dashboard sitting on stale data for
  a long time in an inactive tab and only "waking up" when switched back to.
  A `visibilitychange`/`focus` listener now triggers an immediate full
  refresh as soon as the tab becomes visible/active again
- `fetch` calls now have a 10-second timeout (`AbortController`); a request
  stuck after standby or a network change no longer blocks the UI
  indefinitely
- The history chart is always redrawn when switching to the History page:
  `canvas.clientWidth` is 0 while the page is not visible, so a redraw that
  happened to run in the background could have drawn the chart at the wrong
  width

## 4.1.2

### Fixed

- The actual reason the percent input and test buttons stayed visible in
  Automatic/Emergency: `.controls-row` sets `display:flex` as an author
  rule, which always overrides the `hidden` attribute (only a browser
  default rule at equal specificity) – regardless of the attribute itself.
  The JS was setting `hidden` correctly, it just never had a visible effect.
  `.controls-row[hidden]{display:none}` now enforces this explicitly (the
  same pattern already used for `.chart-tooltip` and `.login-error`). The
  previous 4.1.1 fix (deriving the button/rows from the same state) remains
  in place as well.

## 4.1.1

### Fixed

- After clicking "Manual" without then clicking "Set", the next status poll
  would highlight "Automatic" again, while the percent input and test
  buttons stayed visible regardless (the button and the rows were driven by
  separate state that could drift apart). Both are now derived from the
  same value, so they can no longer disagree; a genuine override committed
  elsewhere (e.g. Emergency) still correctly overrides an open-but-never-
  confirmed Manual view.

## 4.1.0

### Added and Improved

- Every sidebar entry (Status, Control, History, Events, Configuration) is
  now its own page: only the selected section is visible, instead of
  scrolling through all five stacked on top of each other. Navigation runs
  through the URL hash, browser back/forward works
- The event list now shows only 10 entries at a time, with previous/next
  arrows and a page indicator below (similar to the log view in Portainer);
  a filter resets the page back to 1

## 4.0.2

### Fixed

- The chart tooltip appeared anywhere in the chart, even deep in the filled
  area below the line; it now only shows when the cursor is close to the
  line itself
- "Mode & Test" always showed the percent input and test buttons; they now
  only appear when "Manual" is selected (they stay hidden for Automatic and
  Emergency)

## 4.0.1

### Fixed

- The flash boot stick was still counted as an HDD on a real system despite
  the `IsHDD()` filter, because its `disks.ini` section had no
  `type="FLASH"`. The exclusion now also checks the section name (`flash`,
  `cache*`), independent of the `type=` field
- The hard drive tile showed drives in `disks.ini` write order instead of
  sorted; `disk1..diskN` and `parity`/`parity2` now appear in natural order

## 4.0.0

### Breaking

- `API_TOKEN` is removed entirely, replaced by a login
  (`ADMIN_USER`/`ADMIN_PASSWORD`). **Before updating:** set `ADMIN_PASSWORD`
  in the stack environment, otherwise the dashboard is openly reachable
  after the update (login is deliberately disabled without a password set,
  so a fresh deploy does not lock you out). `scripts/fanctl` now logs in
  automatically with the same variables; an old `API_TOKEN` entry in the
  stack configuration is ignored

### Added and Improved

- A login screen protects the whole dashboard (session cookie, 24h sliding,
  bcrypt-hashed password, strict rate limit against brute force);
  `GET /api/health` stays reachable without login for the Docker
  healthcheck and external monitoring
- New tile for each HDD's temperature; cache/flash devices in `disks.ini`
  no longer count toward `disks_reporting`/`maximum_disk_temperature`
  (previously they could factor an SSD/NVMe cache temperature into the fan
  curve)
- Light/dark toggle at the top of the header (default: dark)
- Automatic/Manual/Emergency are now a single bar highlighting the current
  mode, instead of being split across header buttons and a separate input
  field
- History charts show the exact value and time on hover
- Confirmation messages after an action are worded readably and
  automatically fade out after 4 seconds, instead of sitting on screen as
  raw JSON forever
- "Reason" is now part of the controller status card instead of its own,
  visually isolated tile
- README fully rewritten in English, including every feature added since
  3.1.0

## 3.2.0

### Fixed

- `setOverride` wrote the in-memory state before `override.json` was saved;
  if saving failed, the override stayed active anyway. Persistence now runs
  first, `setOverride` returns an error, and the affected endpoints respond
  with HTTP 500
- The same bug affected `handleConfigUpdate` and `handleProfile` (memory
  update before persistence with a revert attempt on failure); both now
  persist first
- A manual fan test could write a speed below the current emergency,
  failsafe, or array-boost minimum; unsafe test values are now rejected
  with HTTP 409
- Persistence errors from `AppendHistory`/`AppendEvent` were swallowed; they
  are now logged and reflected in `/api/health`

### Security

- Configuration and override loaded from `config.json`/`override.json` use
  `DisallowUnknownFields` and an EOF check; unknown fields or data after the
  JSON object are rejected (also applies to `POST /api/config`)
- Write endpoints are limited per category (override, profile, config,
  test) to one write per second; a fan test additionally allows only one
  active test at a time and has its own 5-second cooldown
- Profile, configuration, and override changes are serialized through a
  dedicated mutex, so concurrent requests can no longer overwrite each
  other

### Added and Improved

- `/api/status` returns `target_percent`, `last_applied_percent`, and
  `feedback_available` separately from `fan_percent` (kept as an alias);
  the dashboard now shows target and actual values separately and notes
  that the controller provides no RPM feedback (`last_applied_percent`
  only changes on an actually successful I²C write)
  - `/api/health` additionally returns `status`, `controller`, `config`,
  and `storage` as individual checks instead of just an overall `healthy`
- `REAPPLY_INTERVAL_SECONDS` (default 300) periodically rewrites the active
  PWM even without a value change, and additionally immediately after a
  successful controller rediscovery; after a failed write it retries after
  10 seconds instead of waiting the full 300
- `config.json` now carries `config_version`; if the field is missing
  (older installations), version 1 is silently assumed, and a version
  higher than this binary supports is rejected
- CI: `golangci-lint` is pinned to `v1.64.2` instead of `latest`,
  `go mod tidy` plus `git diff --exit-code` keeps `go.mod`/`go.sum`
  consistent, `docker compose config` validates the compose file
- `docker-compose.yml` sets `stop_grace_period: 20s`, so the shutdown speed
  can be safely written before a `SIGKILL`

## 3.1.1

### Fixed

- An image built locally via a Portainer git stack always showed `vdev` in
  the dashboard and in `/api/health` instead of the real version number,
  because `docker-compose.yml` hard-set the `VERSION` build arg to `dev`
  without an environment variable set. A `VERSION` file in the repo root is
  now the source of truth; the Dockerfile reads it automatically unless a
  `VERSION` arg is passed (CI still sets it from the git tag)

## 3.1.0

### Fixed

- `internal/app` imported `os` without using it, the project failed to
  compile
- `I2C_RETRIES=0` reported a successful write without ever calling `i2cset`
- `CHECK_INTERVAL_SECONDS=0` caused a panic in `time.NewTicker`
- A missing or unreadable `disks.ini` was treated as 0 °C and dropped the
  fans to the lowest curve step; a safety speed now applies instead
- `config.json` was decoded into the default profiles, so deleted profiles
  reappeared after a restart
- Curves from `config.json` and the REST API were neither validated nor
  sorted, even though evaluation assumes sorted points
- `fanctl` split the token header on whitespace, so the token never arrived
- The dashboard's event list was built via `innerHTML` (stored XSS via
  values from `var.ini` and profile names)
- An unreadable array status no longer triggers a permanent boost
- Unraid's `recon P` is now recognized as a rebuild
- Hysteresis no longer holds a boost or emergency speed in place
- After a fan test, the control loop restores the computed value again
- `GET /` was a catch-all and served HTML for every path

### Security

- Token comparison runs in constant time
- Write endpoints check Origin and `Sec-Fetch-Site` against cross-site
  requests
- Content-Security-Policy without `unsafe-inline`, CSS and JavaScript are
  served from their own routes

### Added and Improved

- `history.jsonl` and `events.jsonl` rotate (`MAX_LOG_LINES`), reads go
  through a ring buffer instead of loading the whole file
- `SAFE_SHUTDOWN_PERCENT` sets a defined speed on shutdown
- The HTTP server starts before the controller search, the dashboard now
  shows why no controller was found
- `i2cdetect` now only runs on startup, after write failures, and every
  `DETECT_INTERVAL_SECONDS`, no longer every cycle
- Retries use a context-aware backoff, shutdown is not delayed
- Address detection parses the `i2cdetect` grid column by column
- Atomic configuration writes with `fsync`
- Unit tests for the curve, decision logic, INI parsing, store, and HTTP
  layer
- `docker-compose.yml` mounts `/var/local/emhttp` as a directory and takes
  the port, bind address, and bus number from variables

## 3.0.0

- Web dashboard, history, events, profiles, REST API, Docker hardening
