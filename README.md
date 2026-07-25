# ZimaCube Fan Controller v3.1

Docker-basierte Lüftersteuerung für die ZimaCube-Backplane unter Unraid.
Ein Go-Dienst liest die HDD-Temperaturen und den Array-Status, bestimmt daraus
eine Zieldrehzahl und schreibt sie über `i2cset` (i2c-tools) an den Controller.

## Features

- Nativer Go-Dienst, Ansteuerung über i2c-tools
- Web-Dashboard mit Live-Status, Verlaufsdiagramm und Ereignisliste
- Editierbare Profile: Silent, Balanced, Performance
- Manuelle Steuerung und Lüfter-Testtasten
- Automatischer Boost bei Parity-Check, Rebuild, Resync und Clear
- Notfallschutz über Temperaturschwelle
- Sicherheitsdrehzahl, wenn die HDD-Temperatur nicht lesbar ist
- Hysterese gegen Pendeln
- Persistente Konfiguration, Verlauf und Ereignisse mit automatischer Rotation
- REST-API mit optionalem API-Token und Schutz gegen Cross-Site-Anfragen
- Docker-Healthcheck, read-only Container, keine Capabilities
- GitHub-CI mit Tests und GHCR-Publishing

## Sicherheitsverhalten

Diese Reihenfolge ist bewusst so gewählt und wird durch Tests abgesichert:

1. Notfalltemperatur schlägt alles andere.
2. Array-Boost hebt die Drehzahl an, senkt sie nie.
3. Ist die HDD-Temperatur nicht lesbar, gilt die Sicherheitsdrehzahl
   (`array_boost_percent` des aktiven Profils) statt der untersten Kurvenstufe.
4. Manuelle Vorgaben gelten nur innerhalb dieser Grenzen.
5. Die Hysterese dämpft ausschließlich die Automatik, sie hält keine
   Boost- oder Notfalldrehzahl fest.

## Voraussetzung unter Unraid

Vor `emhttp` in `/boot/config/go` eintragen:

```bash
modprobe i2c-dev
modprobe i2c-i801
```

## Konfiguration

Alle Werte kommen aus Umgebungsvariablen, siehe `.env.example`. Ungültige Werte
werden geloggt und auf gültige Grenzen gesetzt, statt den Dienst zu starten und
später zu überraschen.

`API_TOKEN` sollte gesetzt sein, sobald das Dashboard im LAN erreichbar ist.
Ohne Token kann jeder im Netz die Lüfter steuern; Cross-Site-Anfragen aus dem
Browser werden auch ohne Token blockiert.

## Portainer

Als Git-Repository-Stack deployen. Da das Image lokal gebaut wird,
**Re-pull image** deaktivieren. Die Variablen aus `.env.example` als
Stack-Umgebungsvariablen hinterlegen.

## Dashboard

```text
http://<unraid-ip>:8086/
```

## Kommandos

```bash
docker exec zimacube-fan-controller fanctl status
docker exec zimacube-fan-controller fanctl health
docker exec zimacube-fan-controller fanctl 75
docker exec zimacube-fan-controller fanctl auto
docker exec zimacube-fan-controller fanctl emergency
docker exec zimacube-fan-controller fanctl profile performance
docker exec zimacube-fan-controller fanctl test 50
```

Wenn `API_TOKEN` gesetzt ist, wird es aus der Container-Umgebung übernommen.

## Entwicklung

```bash
go test ./...
go vet ./...
gofmt -w .
```
