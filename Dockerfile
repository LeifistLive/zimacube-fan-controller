FROM golang:1.24-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.su[m] ./
COPY cmd ./cmd
COPY internal ./internal
# vet und Tests laufen im Build, damit ein Portainer-Deployment ohne CI nicht
# mit einem defekten Binary endet.
RUN go vet ./... && go test ./... && go build -trimpath -ldflags="-s -w" -o /out/zimafan ./cmd/zimafan

FROM alpine:3.22
# i2c-tools stellt i2cget/i2cset/i2cdetect bereit, wget wird vom Healthcheck
# und von fanctl benutzt (BusyBox-wget kennt --method nicht).
RUN apk add --no-cache i2c-tools wget ca-certificates tzdata
COPY --from=build /out/zimafan /usr/local/bin/zimafan
COPY scripts/fanctl /usr/local/bin/fanctl
RUN chmod 0755 /usr/local/bin/zimafan /usr/local/bin/fanctl

# Der Prozess laeuft als root, weil /dev/i2c-* auf Unraid root:root 0600 ist.
# Ein eigener Benutzer braeuchte zusaetzlich passende Rechte auf dem Device.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
	CMD wget -q --spider http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/zimafan"]
