ARG VERSION=dev

# Digest-gepinnt, damit ein Rebuild nicht ungeprüft ein neues Basisimage zieht.
# Beide Digests zeigen auf multi-arch Manifest-Listen (amd64 + arm64), --platform
# steuert weiterhin, welche Variante gezogen wird.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine@sha256:8bee1901f1e530bfb4a7850aa7a479d17ae3a18beb6e09064ed54cfd245b7191 AS build
WORKDIR /src
ENV CGO_ENABLED=0
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
COPY go.mod go.su[m] ./
COPY cmd ./cmd
COPY internal ./internal
# vet und Tests laufen im Build, damit ein Portainer-Deployment ohne CI nicht
# mit einem defekten Binary endet. Sie laufen unter BUILDPLATFORM (nativ, ohne
# Emulation); nur der eigentliche Build unten kreuzkompiliert auf TARGETOS/ARCH.
RUN go vet ./... && go test ./...
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
	-ldflags="-s -w -X github.com/LeifistLive/zimacube-fan-controller/internal/app.Version=${VERSION}" \
	-o /out/zimafan ./cmd/zimafan

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce
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
