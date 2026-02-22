# Configuration

dzsa-sync is configured via a YAML file. Pass the path with the `-config` flag:

```bash
dzsa-sync -config /etc/dzsa-sync/config.yaml
```

## Config file format

| Field         | Type    | Description |
|---------------|---------|-------------|
| `log_path`    | string  | **Required.** Path to the log file (JSON, rotated via lumberjack). |
| `detect_ip`   | bool    | When `true`, use https://ifconfig.net/json to detect the host's external IP. When `false`, you must set `external_ip`. |
| `external_ip` | string  | Required when `detect_ip` is `false`. The external IP address used when registering servers with DZSA launcher. |
| `servers`     | []object| List of servers to register. Each entry must have `name` (string) and `port` (1–65535). Names are used in metrics and logs. |
| `servers[].name` | string | **Required.** Label for the server (e.g. for metrics attribute `server`). |
| `servers[].port` | int    | **Required.** Server query port (1–65535). Registered as `external_ip:port` with dayzsalauncher.com. |
| `api`         | object  | Optional. HTTP API server (metrics and synced-servers endpoints). When omitted, defaults to host `""` (all interfaces) and port `8888`. |
| `api.host`    | string  | Listen address for the API server. Empty means all interfaces (e.g. `:port`). |
| `api.port`    | int     | Listen port (1–65535). Default `8888` when `api` is omitted. |

## Example

**Dynamic IP (detect from ifconfig.net):**

```yaml
detect_ip: true
servers:
  - name: main
    port: 2424
  - name: modded
    port: 2324
  - name: experimental
    port: 27016
```

**Static IP:**

```yaml
detect_ip: false
external_ip: "203.0.113.10"
servers:
  - name: main
    port: 2424
  - name: modded
    port: 2324
```

**With API server on localhost:**

```yaml
api:
  host: localhost
  port: 8888
detect_ip: true
servers:
  - name: main
    port: 2424
```

## Logging

Logs are written as JSON to a file with rotation (see [lumberjack](https://pkg.go.dev/gopkg.in/natefinch/lumberjack.v2)). You must set `log_path` in the config (e.g. `/var/log/dzsa-sync/dzsa-sync.log`). Rotation settings (max size, backups, max age, compression) are built-in defaults.

## API server and metrics

The same HTTP server serves Prometheus metrics and the synced-servers JSON API. When `api` is omitted, it listens on all interfaces at port 8888.

- **Prometheus metrics**: `GET /metrics` — see the repo README for metric names and labels (including `server_player_count` with attribute `server`).
- **Synced servers**: `GET /api/v1/servers` returns a JSON list of all synced servers (by config port). `GET /api/v1/servers/<port>` returns a single server by the port number defined in config; responds with 404 if the port is not configured or not yet synced.
