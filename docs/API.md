# API

## Login

- `GET /login` – Login-Seite (öffentlich)
- `POST /login` – erwartet `Authorization: Basic base64(user:passwort)`,
  setzt bei Erfolg die Session-Cookie `zimafan_session` (24h gleitend)
- `POST /logout` – beendet die Session

Ist `ADMIN_PASSWORD` gesetzt, brauchen alle Routen außer `GET /login`,
`POST /login` und `GET /api/health` eine gültige Session; ohne Session gibt
es 401 (bei `GET /` einen 302-Redirect nach `/login`). Ist `ADMIN_PASSWORD`
leer, ist der Login deaktiviert und alles offen erreichbar.

`scripts/fanctl` (für `docker exec`) loggt sich automatisch mit
`ADMIN_USER`/`ADMIN_PASSWORD` aus der Container-Umgebung ein und cached die
Session in `/tmp/fanctl-session`.

## Lesend

- `GET /api/status`
- `GET /api/health` (immer ohne Login erreichbar, für Healthcheck/Monitoring)
- `GET /api/history?limit=288`
- `GET /api/events?limit=100`
- `GET /api/config`

## Schreibend

- `POST /api/fan/{1-100}`
- `POST /api/mode/auto`
- `POST /api/mode/emergency`
- `POST /api/profile/{name}`
- `POST /api/test/{1-100}`
- `POST /api/config`

Unabhängig von der Session werden Anfragen mit fremdem `Origin` oder
`Sec-Fetch-Site: cross-site` mit 403 abgelehnt.

Schreibende Endpunkte sind pro Kategorie (Override: `fan`/`mode/auto`/
`mode/emergency`, `profile`, `config`, `test`) auf einen Schreibvorgang pro
Sekunde begrenzt; ein Verstoß liefert 429. `POST /api/test/{percent}` lässt
zusätzlich nur einen aktiven Test gleichzeitig zu (409, wenn schon einer
läuft) und hat danach einen eigenen 5-Sekunden-Cooldown (429 währenddessen).

## Statusfelder

| Feld | Bedeutung |
| --- | --- |
| `mode` | `automatic`, `manual`, `emergency`, `array-boost`, `failsafe` |
| `target_percent` | Von der Regellogik angeforderter Prozentwert (kann sich ändern, auch wenn das Schreiben fehlschlägt) |
| `last_applied_percent` | Zuletzt tatsächlich erfolgreich per `i2cset` geschriebener Wert |
| `fan_percent` | Veralteter Alias für `target_percent`, bleibt aus Kompatibilitätsgründen erhalten |
| `feedback_available` | Immer `false`: der Controller hat keine RPM/PWM-Rückmeldung, `last_applied_percent` ist nur der zuletzt geschriebene Wert, keine Hardware-Bestätigung |
| `temperature_valid` | `false`, wenn `disks.ini` nicht lesbar war |
| `disks_reporting` | Anzahl der HDD-Abschnitte mit `temp=` in `disks.ini` (Cache/Flash-Geräte zählen nicht mit) |
| `disks` | Ein Eintrag je HDD: `name`, `temperature`, `valid` (`false` = Standby/unlesbar) |
| `controller_online` | Ergebnis der letzten Erreichbarkeitsprüfung |
| `last_write_successful` | Ergebnis des letzten `i2cset` |

`GET /api/health` liefert 503, solange der Controller offline ist, der letzte
Schreibvorgang fehlschlug, die Temperatur unbekannt ist, das aktive Profil
fehlt (`config`), die letzte Persistenz fehlschlug (`storage`) oder der
Status veraltet ist. Neben dem Gesamt-`healthy` liefert die Antwort die
Einzelprüfungen `status` (`"healthy"`/`"unhealthy"`), `controller`, `config`,
`last_write_successful` und `storage` getrennt, damit Monitoring (z. B.
Uptime Kuma) die Ursache erkennen kann.

## Konfiguration schreiben

`POST /api/config` erwartet die vollständige Konfiguration. Profile werden
validiert: Kurvenpunkte müssen zwischen 1 und 100 Prozent liegen, dürfen keine
doppelten Temperaturen enthalten und nicht mit steigender Temperatur fallen.
Unbekannte JSON-Felder und Daten nach dem JSON-Objekt werden abgelehnt.
Abgelehnte Konfigurationen ändern nichts.

`config_version` kennzeichnet das Konfigurationsformat (aktuell `1`). Fehlt
das Feld (Konfiguration von vor dieser Änderung), wird stillschweigend
Version 1 angenommen; eine höhere Version als von diesem Binary unterstützt
wird abgelehnt.

Ein Lüftertest (`POST /api/test/{percent}`) wird mit 409 abgelehnt, wenn der
Wert die aktuelle Notfall-, Failsafe- oder Array-Boost-Untergrenze
unterschreiten würde; die Antwort enthält `minimum_percent`.
