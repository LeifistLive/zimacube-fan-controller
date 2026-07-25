FROM alpine:3.22

RUN apk add --no-cache i2c-tools util-linux busybox-extras

COPY src/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY src/fanctl /usr/local/bin/fanctl
COPY src/healthcheck.sh /usr/local/bin/healthcheck.sh
COPY src/api.sh /usr/local/bin/api.sh

RUN chmod 755 /usr/local/bin/entrypoint.sh /usr/local/bin/fanctl /usr/local/bin/healthcheck.sh /usr/local/bin/api.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
