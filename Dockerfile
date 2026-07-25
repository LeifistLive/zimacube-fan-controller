FROM alpine:3.22

RUN apk add --no-cache i2c-tools

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY fanctl /usr/local/bin/fanctl

RUN chmod 755 \
    /usr/local/bin/entrypoint.sh \
    /usr/local/bin/fanctl

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]