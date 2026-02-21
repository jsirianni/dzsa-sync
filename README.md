# dzsa-sync

[![CI](https://github.com/jsirianni/dzsa-sync/actions/workflows/ci.yml/badge.svg)](https://github.com/jsirianni/dzsa-sync/actions/workflows/ci.yml)

Register DayZ Standalone servers with [dayzsalauncher.com](https://dayzsalauncher.com) so they appear in the DZSA launcher. Uses the same GET API as the launcher’s server check.

- **Configuration:** [docs/configuration.md](docs/configuration.md)
- **Installation:** [docs/installation.md](docs/installation.md)

## Features

- YAML config with optional external IP detection via [ifconfig.net](https://ifconfig.net/json)
- One goroutine per server port; each registers on a 1-hour ticker
- When the external IP changes (every 10 minutes check), all servers are re-synced and tickers reset
- JSON file logging with rotation (lumberjack)
- OpenTelemetry metrics (request count, latency) exposed in Prometheus format at `http://:8888/metrics`

## Quick start

1. Create a config file (see [docs/configuration.md](docs/configuration.md)):

   ```yaml
   detect_ip: true
   ports: [2424, 2324]
   ```

2. Run:

   ```bash
   dzsa-sync -config /path/to/config.yaml
   ```

## Metrics

Prometheus metrics are served at `:8888/metrics`:

- **RequestCount** – HTTP requests (attributes: `host` [dzsa \| ifconfig], `status_code`, `error` [none \| timeout \| status_4xx \| …])
- **RequestLatency** – Request duration in seconds (attributes: `host`, `status_code`)

## Build and test

- `make lint` – revive
- `make secure` – gosec
- `make test` – tests
- `make build` – build binary

## License

MIT
