# Configuration

## Fan curve

```yaml
FAN_CURVE: "0:60,36:65,40:75,43:85,46:95,48:100"
```

Each point is `temperature:percent`.

## Safety

- `ARRAY_BOOST_PERCENT`: Minimum speed during array operations
- `EMERGENCY_TEMP`: Emergency temperature threshold
- `EMERGENCY_PERCENT`: Emergency speed
- `HYSTERESIS_C`: Temperature drop required before lowering speed

## I²C

- `I2C_BUS`
- `I2C_ADDRESS`
- `I2C_RETRIES`
- `I2C_TIMEOUT_SECONDS`

## API

- `LISTEN_ADDRESS`
- `API_TOKEN`

The API is bound by Docker Compose to the Unraid LAN address only.
