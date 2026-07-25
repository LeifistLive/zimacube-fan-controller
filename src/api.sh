#!/bin/sh
set -eu

PORT="${1:-8080}"
STATE_JSON="/run/fan-controller/state.json"

while true; do
    {
        read -r request || true
        path="$(printf '%s' "$request" | awk '{print $2}')"
        while IFS= read -r header; do
            [ "$header" = "$(printf '\r')" ] && break
            [ -z "$header" ] && break
        done

        case "$path" in
            /|/status)
                if [ -r "$STATE_JSON" ]; then
                    body="$(cat "$STATE_JSON")"
                    code="200 OK"
                else
                    body='{"error":"state unavailable"}'
                    code="503 Service Unavailable"
                fi
                ;;
            /health)
                if /usr/local/bin/healthcheck.sh >/dev/null 2>&1; then
                    body='{"status":"healthy"}'
                    code="200 OK"
                else
                    body='{"status":"unhealthy"}'
                    code="503 Service Unavailable"
                fi
                ;;
            *)
                body='{"error":"not found"}'
                code="404 Not Found"
                ;;
        esac

        length="$(printf '%s' "$body" | wc -c)"
        printf 'HTTP/1.1 %s\r\n' "$code"
        printf 'Content-Type: application/json\r\n'
        printf 'Content-Length: %s\r\n' "$length"
        printf 'Connection: close\r\n\r\n'
        printf '%s' "$body"
    } | nc -l -p "$PORT" -w 2
done
