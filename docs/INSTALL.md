# Installation

1. I²C-Module in `/boot/config/go` eintragen (siehe README).
2. Bus prüfen: `i2cdetect -l`, danach `I2C_BUS` passend setzen.
3. Repository auf GitHub ablegen.
4. In Portainer einen Git-Stack anlegen oder aktualisieren.
5. Variablen aus `.env.example` als Stack-Umgebungsvariablen setzen,
   mindestens `ADMIN_PASSWORD` (schützt das gesamte Dashboard per Login).
6. **Re-pull image** deaktivieren, der Build läuft lokal.
7. Stack deployen und `http://<unraid-ip>:8086/` öffnen.

## Nach dem Deployment prüfen

```bash
docker exec zimacube-fan-controller fanctl health
docker logs zimacube-fan-controller | tail -20
```

Zeigt das Dashboard `HDD-Temperatur nicht lesbar`, stimmt der Mount von
`/var/local/emhttp` nicht. Zeigt es `I2C-Controller nicht erreichbar`, passen
Busnummer oder Adresse nicht, oder die Kernelmodule fehlen.
