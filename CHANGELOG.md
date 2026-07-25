# Changelog

## 3.1.1

### Behoben

- Ein per Portainer-Git-Stack lokal gebautes Image zeigte im Dashboard und in
  `/api/health` immer `vdev` statt der echten Versionsnummer, weil
  `docker-compose.yml` den `VERSION`-Build-Arg ohne gesetzte Umgebungsvariable
  fest auf `dev` setzte. Eine `VERSION`-Datei im Repo-Root ist jetzt die
  Quelle der Wahrheit; das Dockerfile liest sie automatisch, sofern kein
  `VERSION`-Arg übergeben wird (CI setzt ihn weiterhin aus dem Git-Tag)

## 3.1.0

### Behoben

- `internal/app` importierte `os` ohne Verwendung, das Projekt ließ sich nicht
  übersetzen
- `I2C_RETRIES=0` meldete Schreiberfolg, ohne jemals `i2cset` aufzurufen
- `CHECK_INTERVAL_SECONDS=0` führte zu einem Panic in `time.NewTicker`
- Eine fehlende oder unlesbare `disks.ini` wurde als 0 °C gewertet und senkte
  die Lüfter auf die unterste Kurvenstufe; jetzt greift eine Sicherheitsdrehzahl
- `config.json` wurde in die Standardprofile hineingelesen, gelöschte Profile
  kamen nach einem Neustart zurück
- Kurven aus `config.json` und der REST-API wurden weder validiert noch
  sortiert, obwohl die Auswertung sortierte Punkte voraussetzt
- `fanctl` zerlegte den Token-Header am Leerzeichen, der Token kam nie an
- Ereignisliste im Dashboard wurde per `innerHTML` erzeugt (Stored XSS über
  Werte aus `var.ini` und Profilnamen)
- Ein unlesbarer Array-Status löst keinen Dauerboost mehr aus
- `recon P` von Unraid wird als Rebuild erkannt
- Die Hysterese hält keine Boost- oder Notfalldrehzahl mehr fest
- Nach einem Lüftertest stellt der Regelkreis den berechneten Wert wieder her
- `GET /` war ein Catch-all und lieferte für jeden Pfad HTML

### Sicherheit

- Token-Vergleich läuft in konstanter Zeit
- Schreibendpunkte prüfen Origin und `Sec-Fetch-Site` gegen Cross-Site-Anfragen
- Content-Security-Policy ohne `unsafe-inline`, CSS und JavaScript liegen auf
  eigenen Routen

### Neu und verbessert

- `history.jsonl` und `events.jsonl` rotieren (`MAX_LOG_LINES`), Lesezugriffe
  laufen über einen Ring-Buffer statt die Datei komplett zu laden
- `SAFE_SHUTDOWN_PERCENT` setzt beim Beenden eine definierte Drehzahl
- Der HTTP-Server startet vor der Controller-Suche, das Dashboard zeigt jetzt,
  warum kein Controller gefunden wird
- `i2cdetect` läuft nur noch beim Start, nach Schreibfehlern und alle
  `DETECT_INTERVAL_SECONDS`, nicht mehr in jedem Zyklus
- Retries verwenden einen Context-fähigen Backoff, Shutdown verzögert sich nicht
- Adresserkennung wertet das `i2cdetect`-Raster spaltenweise aus
- Atomare Konfigurationsschreibvorgänge mit `fsync`
- Unit-Tests für Kurve, Entscheidungslogik, INI-Parsing, Store und HTTP-Schicht
- `docker-compose.yml` bindet `/var/local/emhttp` als Verzeichnis ein und nimmt
  Port, Bind-Adresse und Busnummer aus Variablen

## 3.0.0

- Web-Dashboard, Verlauf, Ereignisse, Profile, REST-API, Docker-Härtung
