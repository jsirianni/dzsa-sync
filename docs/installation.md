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
- Log directory: `/var/log/dzsa-sync/` (mode 0750, owner dzsa-sync)
- Systemd unit: `dzsa-sync.service`

### Scripts (blitz-style)

- **preinstall**: Creates the `dzsa-sync` user and group (system, nologin), then runs `systemctl daemon-reload`.
- **postinstall**: Ensures `/etc/dzsa-sync` exists and is owned by `dzsa-sync:dzsa-sync`, then runs `systemctl daemon-reload`. The service is not started or enabled automatically.
- **preremove**: Stops `dzsa-sync.service`.
- **postremove**: Runs `systemctl daemon-reload` only.

### After install

1. Edit `/etc/dzsa-sync/config.yaml` (set `detect_ip` and `servers` (name + port for each), and `external_ip` if not using IP detection).
2. Enable and start the service:

   ```bash
   sudo systemctl enable dzsa-sync
   sudo systemctl start dzsa-sync
   ```

3. View the configured log file (default `/var/log/dzsa-sync/dzsa-sync.log`).

## Manual run

Build from source (see repo root):

```bash
go build -o dzsa-sync ./cmd/dzsasync
./dzsa-sync -config /path/to/config.yaml
```

The API server (metrics and `/api/v1/servers`) listens on a configurable host/port (default port 8888). Ensure the log path is writable.
