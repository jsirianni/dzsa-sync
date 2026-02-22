# dzsa-sync

[![CI](https://github.com/jsirianni/dzsa-sync/actions/workflows/ci.yml/badge.svg)](https://github.com/jsirianni/dzsa-sync/actions/workflows/ci.yml)

Register DayZ Standalone servers with [dayzsalauncher.com](https://dayzsalauncher.com) so they appear in the DZSA launcher. Uses the same GET API as the launcher’s server check.

- **Configuration:** [docs/configuration.md](docs/configuration.md)
- **Installation:** [docs/installation.md](docs/installation.md)
- **Architecture:** [docs/architecture.md](docs/architecture.md)
- **Development:** [docs/develop.md](docs/develop.md)

## Features

- YAML config with optional external IP detection via [ifconfig.net](https://ifconfig.net/json)
- One goroutine per server port; each registers on a 1-hour ticker
- When the external IP changes (every 10 minutes check), all servers are re-synced and tickers reset
- JSON file logging with rotation (lumberjack)
- OpenTelemetry metrics (request count, latency, server player count) exposed in Prometheus format; configurable API server (default `:8888`) with `/metrics` and JSON `/api/v1/servers` endpoints

## Quick start

1. Create a config file (see [docs/configuration.md](docs/configuration.md)):

   ```yaml
   detect_ip: true
   servers:
     - name: main
       port: 2424
     - name: modded
       port: 2324
   ```

2. Run:

   ```bash
   dzsa-sync -config /path/to/config.yaml
   ```

## API server

The HTTP server is configurable via the `api` section in config (default: all interfaces, port 8888). It serves:

- **Prometheus metrics**: `GET /metrics` — RequestCount and RequestLatency (attributes: `host` [dzsa | ifconfig], `status_code`, `error` [none | timeout | status_4xx | …]); `server_player_count` (gauge: players from DZSA response, attribute: `server` from config).
- **Synced servers (JSON)**: `GET /api/v1/servers` — list all synced servers; `GET /api/v1/servers/<port>` — single server by config port (404 if unknown or not yet synced).

## Build and test

- `make lint` – revive
- `make secure` – gosec
- `make test` – tests
- `make build` – build binary

## License

MIT
