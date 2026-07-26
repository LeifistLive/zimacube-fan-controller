# Installation

1. Add the I²C modules to `/boot/config/go` (see README).
2. Check the bus: `i2cdetect -l`, then set `I2C_BUS` accordingly.
3. Push the repository to GitHub.
4. Create or update a git stack in Portainer.
5. Set the variables from `.env.example` as stack environment variables,
   at minimum `ADMIN_PASSWORD` (protects the whole dashboard via login).
6. Disable **Re-pull image**, the build runs locally.
7. Deploy the stack and open `http://<unraid-ip>:8086/`.

## Checking After Deployment

```bash
docker exec zimacube-fan-controller fanctl health
docker logs zimacube-fan-controller | tail -20
```

If the dashboard shows `HDD temperature unreadable`, the mount of
`/var/local/emhttp` is wrong. If it shows `I2C controller unreachable`, the
bus number or address is wrong, or the kernel modules are missing.
