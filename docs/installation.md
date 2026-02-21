# Installation

## Linux (systemd)

Download the latest release from the [releases page](https://github.com/jsirianni/dzsa-sync/releases).

**RPM (Fedora/RHEL):**

```bash
sudo dnf install ./dzsa-sync-amd64.rpm
```

**Debian/Ubuntu:**

```bash
sudo apt install -f ./dzsa-sync-amd64.deb
```

### Package contents

- Binary: `/usr/bin/dzsa-sync`
- Config directory: `/etc/dzsa-sync/` (base config file included)
- Systemd unit: `dzsa-sync.service`

### Scripts (blitz-style)

- **preinstall**: Creates the `dzsa-sync` user and group (system, nologin), then runs `systemctl daemon-reload`.
- **postinstall**: Ensures `/etc/dzsa-sync` exists and is owned by `dzsa-sync:dzsa-sync`, then runs `systemctl daemon-reload`. The service is not started or enabled automatically.
- **preremove**: Stops `dzsa-sync.service`.
- **postremove**: Runs `systemctl daemon-reload` only.

### After install

1. Edit `/etc/dzsa-sync/config.yaml` (set `detect_ip` and `ports`, and `external_ip` if not using IP detection).
2. Enable and start the service:

   ```bash
   sudo systemctl enable dzsa-sync
   sudo systemctl start dzsa-sync
   ```

3. View logs (if using journald for stdout/stderr) or the configured log file (e.g. `/var/log/dzsa-sync/dzsa-sync.log` when running as service with that path).

## Docker

Images are published to `ghcr.io/jsirianni/dzsa-sync`. The image is built from scratch and includes only the binary and CA certificates.

```bash
docker run -d --name dzsa-sync \
  -v /path/to/config.yaml:/etc/dzsa-sync/config.yaml:ro \
  -p 8888:8888 \
  ghcr.io/jsirianni/dzsa-sync:latest \
  -config /etc/dzsa-sync/config.yaml
```

Ensure the config is valid and the process can write logs to its configured path if you mount a volume for logs.

## Manual run

Build from source (see repo root):

```bash
go build -o dzsa-sync ./cmd/dzsasync
./dzsa-sync -config /path/to/config.yaml
```

Metrics are served on port 8888; ensure the log path is writable.
