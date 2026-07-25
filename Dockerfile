FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/zimafan ./cmd/zimafan

FROM alpine:3.22
RUN apk add --no-cache i2c-tools ca-certificates tzdata wget \
    && adduser -D -H -s /sbin/nologin zimafan

COPY --from=build /out/zimafan /usr/local/bin/zimafan
COPY scripts/fanctl /usr/local/bin/fanctl
RUN chmod 755 /usr/local/bin/zimafan /usr/local/bin/fanctl

ENTRYPOINT ["/usr/local/bin/zimafan"]
