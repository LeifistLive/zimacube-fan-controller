# Changelog

## 4.3.0

### Kritisch behoben

- Ein `ADMIN_PASSWORD` über 72 Byte ließ bcrypt fehlschlagen; der Code
  behauptete "fail closed", setzte `enabled` aber faktisch nie und ließ das
  Dashboard damit ohne jeden Login offen. `newAuth` gibt jetzt einen Fehler
  zurück, `New()`/der Dienst starten in diesem Fall gar nicht erst

### Behoben

- `a.state.reapplyAt` wurde in `evaluate()` außerhalb des Locks gelesen;
  jetzt Teil desselben unter `RLock()` erstellten Snapshots wie die übrigen
  Loop-Statusfelder
- `storageOK` war ein einzelnes globales Flag: ein erfolgreicher
  History-Schreibvorgang konnte einen weiterhin ungelösten Config-Fehler
  verdecken. Jetzt pro Kategorie (config/override/history/events) getrennt
  verfolgt, `storage` in `/api/health` ist nur `true`, wenn alle vier zuletzt
  erfolgreich waren
- `Store.Remove()` synct jetzt das Verzeichnis nach dem Löschen (dieselbe
  Garantie, die `SaveJSON` beim Schreiben schon hatte)
- Fehler bei der Logrotation (Zeilenzählung, Prune) werden jetzt geloggt
  statt still verschluckt
- Sessions und Login-Rate-Limits pro IP wurden bisher nur beim erneuten
  Zugriff auf denselben (dann abgelaufenen) Eintrag entfernt; ein
  stündlicher Sweep räumt jetzt abgelaufene Sessions und veraltete
  Rate-Limits aktiv auf
- Das Session-Cookie bekommt `Secure`, sobald die Anfrage über TLS ankam
  (direkt oder `X-Forwarded-Proto: https`); bei reinem HTTP unverändert
- Der Login-Rate-Limiter war global (ein Client konnte damit jeden anderen,
  inklusive des echten Admins, aussperren); jetzt pro Client-IP
- Profilnamen und Anzeigenamen haben jetzt eine Längenbegrenzung (64 Zeichen)

## 4.2.1

### Geprüft

- Audit: der Dienst greift nie direkt auf die Festplatten zu. `disks.ini`/
  `var.ini` werden nur von Unraids eigener RAM-Datei gelesen (unabhängig vom
  Poll-Intervall kein Plattenzugriff), I²C spricht ausschließlich den
  Fan-Controller-Chip an, nirgends wird `smartctl`/`hdparm` aufgerufen.
  Einzige Voraussetzung: `/data` muss auf nicht-Array-Speicher bleiben (wie
  im mitgelieferten `fan-data`-Docker-Volume), da die Verlaufs-/Event-Datei
  alle paar Minuten geschrieben wird. Das ist jetzt in
  [docker-compose.yml](docker-compose.yml), [README.md](README.md) und einem
  Code-Kommentar an `Config.DataDir` dokumentiert, um versehentliches
  Umbiegen auf eine Array-Platte zu verhindern.

## 4.2.0

### Neu und verbessert

- Browser drosseln `setInterval` in Hintergrund-Tabs stark (teils auf einmal
  pro Minute oder seltener), wodurch das Dashboard in einem inaktiven Tab
  lange auf veralteten Daten stehen blieb und erst beim Zurückwechseln
  "aufwachte". Ein `visibilitychange`/`focus`-Listener löst jetzt sofort
  einen vollständigen Refresh aus, sobald der Tab wieder sichtbar/aktiv wird
- `fetch`-Aufrufe haben jetzt ein 10-Sekunden-Timeout (`AbortController`);
  eine nach Standby oder Netzwerkwechsel hängende Anfrage blockiert die
  Oberfläche nicht mehr unbegrenzt
- Der Verlauf-Chart wird beim Wechsel auf die Verlauf-Seite immer neu
  gezeichnet: `canvas.clientWidth` ist 0, solange die Seite nicht sichtbar
  ist, wodurch ein Redraw, der zufällig im Hintergrund lief, das Diagramm in
  falscher Breite gezeichnet haben konnte

## 4.1.2

### Behoben

- Die eigentliche Ursache dafür, dass Prozent-Eingabe und Testknöpfe bei
  Automatik/Notfall sichtbar blieben: `.controls-row` setzt `display:flex`
  als Autor-Regel, die das `hidden`-Attribut (nur Browser-Standardregel mit
  gleicher Spezifität) immer überstimmt – unabhängig vom Attribut selbst.
  Das JS setzte `hidden` also korrekt, es hatte nur nie eine sichtbare
  Wirkung. `.controls-row[hidden]{display:none}` erzwingt das jetzt explizit
  (dasselbe Muster wie zuvor schon bei `.chart-tooltip` und `.login-error`).
  Der vorherige 4.1.1-Fix (Button/Zeilen aus demselben Zustand ableiten)
  bleibt zusätzlich bestehen.

## 4.1.1

### Behoben

- Nach Klick auf "Manuell" ohne anschließendes "Setzen" hob der nächste
  Status-Poll wieder "Automatik" hervor, während Prozent-Eingabe und
  Testknöpfe trotzdem sichtbar blieben (Button und Zeilen liefen über
  getrennten Zustand auseinander). Beide werden jetzt aus demselben Wert
  abgeleitet, können also nicht mehr auseinanderlaufen; ein echter,
  anderswo gesetzter Override (z. B. Notfall) überschreibt eine offene,
  aber nie bestätigte Manuell-Ansicht weiterhin korrekt.

## 4.1.0

### Neu und verbessert

- Jeder Sidebar-Punkt (Status, Steuerung, Verlauf, Ereignisse, Konfiguration)
  ist jetzt eine eigene Seite: nur der ausgewählte Bereich ist sichtbar,
  statt alle fünf untereinander durchzuscrollen. Navigation läuft über den
  URL-Hash, Browser-Vor/Zurück funktioniert
- Die Ereignisliste zeigt nur noch 10 Einträge gleichzeitig, mit
  Vorherige/Nächste-Pfeilen und Seitenanzeige darunter (ähnlich der
  Log-Ansicht in Portainer); ein Filter setzt die Seite zurück auf 1

## 4.0.2

### Behoben

- Der Chart-Tooltip erschien im gesamten Diagramm, auch tief im
  Verlaufs-Füllbereich unterhalb der Linie; er zeigt sich jetzt nur noch,
  wenn der Mauszeiger nah an der Linie selbst ist
- "Modus & Test" zeigte Prozent-Eingabe und Testknöpfe immer an; sie
  erscheinen jetzt nur noch, wenn "Manuell" ausgewählt ist (bei Automatik
  und Notfall bleiben sie ausgeblendet)

## 4.0.1

### Behoben

- Der Flash-Boot-Stick wurde auf einem realen System trotz `IsHDD()`-Filter
  weiter als HDD gezählt, weil seine `disks.ini`-Sektion kein `type="FLASH"`
  hatte. Der Ausschluss prüft jetzt zusätzlich den Sektionsnamen (`flash`,
  `cache*`), unabhängig vom `type=`-Feld
- Die Festplatten-Kachel zeigte die Laufwerke in `disks.ini`-Schreibreihen-
  folge statt sortiert; `disk1..diskN` und `parity`/`parity2` erscheinen jetzt
  in natürlicher Reihenfolge

## 4.0.0

### Breaking

- `API_TOKEN` entfällt vollständig, ersetzt durch einen Login
  (`ADMIN_USER`/`ADMIN_PASSWORD`). **Vor dem Update:** `ADMIN_PASSWORD` in
  der Stack-Umgebung setzen, sonst ist das Dashboard nach dem Update offen
  erreichbar (Login ist ohne gesetztes Passwort bewusst deaktiviert, damit
  ein frischer Deploy nicht aussperrt). `scripts/fanctl` loggt sich jetzt
  automatisch mit denselben Variablen ein, ein alter `API_TOKEN`-Eintrag in
  der Stack-Konfiguration wird ignoriert

### Neu und verbessert

- Login-Maske schützt das gesamte Dashboard (Session-Cookie, 24h gleitend,
  bcrypt-Hash des Passworts, striktes Rate-Limit gegen Brute-Force);
  `GET /api/health` bleibt für Docker-Healthcheck und externes Monitoring
  ohne Login erreichbar
- Neue Kachel je HDD-Temperatur; Cache-/Flash-Geräte in `disks.ini` zählen
  nicht mehr zu `disks_reporting`/`maximum_disk_temperature` (konnten bisher
  eine SSD/NVMe-Cache-Temperatur in die Lüfterkurve einrechnen)
- Hell/Dunkel-Umschalter oben im Header (Standard: dunkel)
- Automatik/Manuell/Notfall sind jetzt eine einzige, den aktuellen Modus
  hervorhebende Leiste statt über Header-Buttons und ein separates
  Eingabefeld verteilt
- Verlaufs-Charts zeigen beim Hover den exakten Wert und Zeitpunkt
- Bestätigungsmeldungen nach einer Aktion sind lesbar formuliert und blenden
  sich nach 4 Sekunden automatisch aus, statt dauerhaft als Roh-JSON stehen
  zu bleiben
- "Grund" ist jetzt Teil der Controller-Statuskarte statt einer isoliert
  wirkenden eigenen Kachel
- README vollständig auf Englisch, inklusive aller Funktionen seit 3.1.0

## 3.2.0

### Behoben

- `setOverride` schrieb den internen Zustand, bevor `override.json`
  gespeichert war; schlug das Speichern fehl, blieb der Override trotzdem
  aktiv. Persistenz läuft jetzt zuerst, `setOverride` gibt einen Fehler
  zurück, die betroffenen Endpunkte antworten mit HTTP 500
- Derselbe Fehler betraf `handleConfigUpdate` und `handleProfile`
  (Speicher-Update vor Persistenz mit Revert-Versuch bei Fehlschlag);
  beide persistieren jetzt zuerst
- Ein manueller Lüftertest konnte die Drehzahl unter das aktuelle
  Notfall-, Failsafe- oder Array-Boost-Minimum schreiben; unsichere
  Testwerte werden jetzt mit HTTP 409 abgelehnt
- Persistenzfehler von `AppendHistory`/`AppendEvent` wurden verschluckt;
  sie werden jetzt geloggt und fließen in `/api/health` ein

### Sicherheit

- Konfiguration und Override aus `config.json`/`override.json` werden mit
  `DisallowUnknownFields` und einer EOF-Prüfung geladen, unbekannte Felder
  oder Daten nach dem JSON-Objekt werden abgelehnt (gilt auch für
  `POST /api/config`)
- Schreibende Endpunkte sind pro Kategorie (Override, Profil, Konfiguration,
  Test) auf einen Schreibvorgang pro Sekunde begrenzt; ein Lüftertest lässt
  zusätzlich nur einen aktiven Test gleichzeitig zu und hat einen eigenen
  5-Sekunden-Cooldown
- Profil-, Konfigurations- und Override-Änderungen sind über einen
  eigenen Mutex serialisiert, sodass sich nebenläufige Anfragen nicht mehr
  gegenseitig überschreiben können

### Neu und verbessert

- `/api/status` liefert `target_percent`, `last_applied_percent` und
  `feedback_available` getrennt von `fan_percent` (bleibt als Alias
  erhalten); das Dashboard zeigt jetzt Soll- und Ist-Wert getrennt und weist
  darauf hin, dass der Controller keine RPM-Rückmeldung liefert
  (`last_applied_percent` ändert sich nur bei einem tatsächlich
  erfolgreichen I²C-Schreibvorgang)
  - `/api/health` liefert zusätzlich `status`, `controller`, `config` und
  `storage` als einzelne Prüfungen statt nur eines Gesamt-`healthy`
- `REAPPLY_INTERVAL_SECONDS` (Default 300) schreibt die aktive PWM
  regelmäßig erneut, auch ohne Wertänderung, und zusätzlich sofort nach
  einer erfolgreichen Controller-Neuerkennung; nach einem fehlgeschlagenen
  Schreibversuch wird bereits nach 10 statt nach 300 Sekunden erneut
  versucht
- `config.json` trägt jetzt `config_version`; fehlt das Feld (ältere
  Installationen), wird stillschweigend Version 1 angenommen, eine höhere
  Version als von diesem Binary unterstützt wird abgelehnt
- CI: `golangci-lint` ist auf `v1.64.2` gepinnt statt `latest`,
  `go mod tidy` samt `git diff --exit-code` hält `go.mod`/`go.sum`
  konsistent, `docker compose config` prüft die Compose-Datei
- `docker-compose.yml` setzt `stop_grace_period: 20s`, damit die
  Abschaltdrehzahl vor einem `SIGKILL` sicher geschrieben werden kann

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
