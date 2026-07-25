FROM alpine:3.22

RUN apk add --no-cache \
      i2c-tools \
      util-linux \
    && addgroup -S fancontroller \
    && mkdir -p /run/fan-controller

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY fanctl /usr/local/bin/fanctl
COPY healthcheck.sh /usr/local/bin/healthcheck.sh

RUN chmod 755 \
      /usr/local/bin/entrypoint.sh \
      /usr/local/bin/fanctl \
      /usr/local/bin/healthcheck.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]