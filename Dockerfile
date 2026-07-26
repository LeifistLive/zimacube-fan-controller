# Empty by default: the repo-root VERSION file is the actual source of
# truth, so that even a plain Portainer git-stack build (no CI, no
# VERSION environment variable set) bakes in the correct version number.
# CI (ghcr.yml) can still force a git-tag value via this arg.
ARG VERSION=

# Digest-pinned so a rebuild does not silently pull a new base image.
# Both digests point at multi-arch manifest lists (amd64 + arm64); --platform
# still controls which variant gets pulled.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine@sha256:8bee1901f1e530bfb4a7850aa7a479d17ae3a18beb6e09064ed54cfd245b7191 AS build
WORKDIR /src
ENV CGO_ENABLED=0
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
COPY go.mod go.su[m] ./
COPY cmd ./cmd
COPY internal ./internal
COPY VERSION ./VERSION
# vet and tests run during the build, so a Portainer deployment without CI
# never ends up with a broken binary. They run under BUILDPLATFORM (native, no
# emulation); only the actual build below cross-compiles to TARGETOS/ARCH.
RUN go vet ./... && go test ./...
RUN APP_VERSION="${VERSION:-$(cat VERSION)}" && \
	GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
	-ldflags="-s -w -X github.com/LeifistLive/zimacube-fan-controller/internal/app.Version=${APP_VERSION}" \
	-o /out/zimafan ./cmd/zimafan

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce
# i2c-tools provides i2cget/i2cset/i2cdetect, wget is used by the healthcheck
# and by fanctl (BusyBox wget does not support --method).
RUN apk add --no-cache i2c-tools wget ca-certificates tzdata
COPY --from=build /out/zimafan /usr/local/bin/zimafan
COPY scripts/fanctl /usr/local/bin/fanctl
RUN chmod 0755 /usr/local/bin/zimafan /usr/local/bin/fanctl

# The process runs as root because /dev/i2c-* on Unraid is root:root 0600.
# A dedicated user would additionally need matching permissions on the device.
# docker-compose.yml defines the same healthcheck again (Compose fully
# overrides this one as soon as it is used) - this one only applies to a
# plain "docker run" without Compose (e.g. straight from GHCR).
# Keep both places in sync when changing this.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
	CMD wget -q --spider http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/zimafan"]
