# Installation

1. Add the I²C modules to `/boot/config/go` (see README).
2. Check the bus: `i2cdetect -l`, then set `I2C_BUS` accordingly.
3. In Portainer, create a stack from this repository (Git repository stack,
   or paste [docker-compose.yml](../docker-compose.yml) directly).
4. Set the variables from `.env.example` as stack environment variables,
   at minimum `ADMIN_PASSWORD` (protects the whole dashboard via login).
5. Enable **Re-pull image**, so redeploying re-fetches the pinned version
   tag if it changed. `docker-compose.yml` deliberately pins a specific
   version instead of `:latest`, so an upgrade means bumping the tag in
   that file, not just re-pulling.
6. Deploy the stack and open `http://<unraid-ip>:8086/`.

## Building Locally Instead

If you've changed the code and want Portainer to build the image itself
instead of pulling from GHCR, replace the `image:` line in
`docker-compose.yml` with:

```yaml
build:
  context: .
  args:
    # Leave empty: the Dockerfile then falls back to the VERSION file in
    # the repo. Only set this to force a different version number.
    VERSION: "${VERSION:-}"
```

and disable **Re-pull image** — there is nothing to pull anymore, and
Portainer will build from the Dockerfile on every deploy instead.

## Checking After Deployment

```bash
docker exec zimacube-fan-controller fanctl health
docker logs zimacube-fan-controller | tail -20
```

If the dashboard shows `HDD temperature unreadable`, the mount of
`/var/local/emhttp` is wrong. If it shows `I2C controller unreachable`, the
bus number or address is wrong, or the kernel modules are missing.
