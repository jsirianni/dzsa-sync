# Configuration

dzsa-sync is configured via a YAML file. Pass the path with the `-config` flag:

```bash
dzsa-sync -config /etc/dzsa-sync/config.yaml
```

## Config file format

| Field         | Type    | Description |
|---------------|---------|-------------|
| `detect_ip`   | bool    | When `true`, use https://ifconfig.net/json to detect the host's external IP. When `false`, you must set `external_ip`. |
| `external_ip` | string  | Required when `detect_ip` is `false`. The external IP address used when registering servers with DZSA launcher. |
| `ports`       | []int   | List of server query ports. Each port is registered as `external_ip:port` with dayzsalauncher.com. |

## Example

**Dynamic IP (detect from ifconfig.net):**

```yaml
detect_ip: true
ports:
  - 2424
  - 2324
  - 27016
```

**Static IP:**

```yaml
detect_ip: false
external_ip: "203.0.113.10"
ports:
  - 2424
  - 2324
```

## Logging

Logs are written as JSON to a file with rotation (see [lumberjack](https://pkg.go.dev/gopkg.in/natefinch/lumberjack.v2)). The default path is `/var/log/dzsa-sync/dzsa-sync.log`. Rotation settings (max size, backups, max age, compression) are built-in defaults.

## Metrics

Prometheus metrics are exposed at `http://:8888/metrics`. See the repo README for metric names and labels.
