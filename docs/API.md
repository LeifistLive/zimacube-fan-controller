# API

## Lesend

- `GET /api/status`
- `GET /api/health`
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

Ist `API_TOKEN` gesetzt, brauchen Schreibzugriffe:

```text
X-API-Token: dein-token
```

Unabhängig vom Token werden Anfragen mit fremdem `Origin` oder
`Sec-Fetch-Site: cross-site` mit 403 abgelehnt.

## Statusfelder

| Feld | Bedeutung |
| --- | --- |
| `mode` | `automatic`, `manual`, `emergency`, `array-boost`, `failsafe` |
| `temperature_valid` | `false`, wenn `disks.ini` nicht lesbar war |
| `disks_reporting` | Anzahl der `temp=`-Einträge in `disks.ini` |
| `controller_online` | Ergebnis der letzten Erreichbarkeitsprüfung |
| `last_write_successful` | Ergebnis des letzten `i2cset` |

`GET /api/health` liefert 503, solange der Controller offline ist, der letzte
Schreibvorgang fehlschlug, die Temperatur unbekannt ist oder der Status veraltet.

## Konfiguration schreiben

`POST /api/config` erwartet die vollständige Konfiguration. Profile werden
validiert: Kurvenpunkte müssen zwischen 1 und 100 Prozent liegen, dürfen keine
doppelten Temperaturen enthalten und nicht mit steigender Temperatur fallen.
Abgelehnte Konfigurationen ändern nichts.
