# Architecture

This document describes the architecture of **dzsa-sync**: how it is structured, how components interact, and how data and control flow through the system. It is intended for contributors and automated agents (e.g. LLMs) that need to reason about or modify the codebase.

---

## 1. Purpose and high-level behavior

**dzsa-sync** registers DayZ Standalone server endpoints with [dayzsalauncher.com](https://dayzsalauncher.com) so they appear in the DZSA launcher. It does **not** run a game server or export Prometheus metrics about game state; it only:

1. **Resolves the host’s external IP** (either from config or by querying [ifconfig.net](https://ifconfig.net/json)).
2. **Periodically registers each configured port** with the DZSA API by performing a GET request per `external_ip:port`.
3. **Exposes an HTTP API** (configurable host/port, default `:8888`) with Prometheus metrics and JSON endpoints for synced server data, and **writes JSON logs** (including sync results and ifconfig outcomes).

When the external IP changes (in “detect IP” mode), the program immediately re-syncs all ports and resets the per-port sync interval so that the launcher sees the new IP without waiting for the next scheduled tick.

---

## 2. External systems

| System | Role | How dzsa-sync uses it |
|--------|------|------------------------|
| **dayzsalauncher.com** | DZSA launcher backend | GET `https://dayzsalauncher.com/api/v1/query/{ip}:{port}` to register/query a server. Response is JSON with server details (name, players, mods, etc.). |
| **ifconfig.net** | Public IP detection | GET `https://ifconfig.net/json` when `detect_ip` is true. Response includes `ip` (string). Used every 10 minutes; result is cached and compared for changes. |

There are no databases or message queues; state is in-memory (current IP, ticker state) and config is file-based.

---

## 3. Component overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              main (cmd/dzsasync)                         │
│  - Load config, setup logger (JSON + lumberjack), signal handling       │
│  - Create shared HTTP client, metrics provider, DZSA client, ifconfig   │
│  - Create server store (internal/servers), start API HTTP server         │
│  - Start ifconfig loop (if detect_ip) and per-port workers              │
│  - On shutdown: cancel context, wait for workers                        │
└─────────────────────────────────────────────────────────────────────────┘
         │                    │                    │                    │
         ▼                    ▼                    ▼                    ▼
┌──────────────┐    ┌─────────────────┐   ┌──────────────┐   ┌─────────────────┐
│ config       │    │ client (DZSA)    │   │ ifconfig     │   │ internal/metrics│
│ - YAML load  │    │ - Query(ip,port)│   │ - Get() IP   │   │ - Provider      │
│ - Validate   │    │ - Metrics via   │   │ - Run() loop │   │ - HTTPRecorder  │
│ - API host/  │    │   HTTPRecorder  │   │ - GetAddress │   │ - RequestCount  │
│   port       │    └─────────────────┘   └──────────────┘   │   RequestLatency│
└──────────────┘            │                    │             └─────────────────┘
         │                   │                    │                    │
         │                   ▼                    ▼                    │
         │            ┌──────────────┐     (shared recorder)           │
         │            │ model        │                                 │
         │            │ QueryResponse│                                 │
         │            │ Result, etc. │                                 │
         │            └──────────────┘                                 │
         │                   │                                          │
         │                   ▼                                          │
         │            ┌──────────────┐                                  │
         │            │ internal/    │   API server (configurable        │
         └───────────►│ servers      │   host/port, default :8888)       │
                      │ - Store by   │   /metrics, /api/v1/servers,      │
                      │   port       │   /api/v1/servers/<port>          │
                      └──────────────┘                                  │
                                         ▼                              │
                              Prometheus /metrics + JSON /api/v1/servers
```

- **config**: Reads and validates the YAML config (detect_ip, external_ip, ports).
- **client**: Single responsibility—call the DZSA API for one `ip:port`; uses shared `*http.Client` and optional `metrics.HTTPRecorder`.
- **internal/ifconfig**: Fetches public IP from ifconfig.net; caches it and runs a 10-minute loop when `detect_ip` is true; supports `BaseURL` override for tests.
- **internal/metrics**: OpenTelemetry meter provider, Prometheus exporter, and `HTTPRecorder` implementation (request count + latency by host, status code, error type). Serves `/metrics` via the handler returned by `Provider.Handler()`.
- **internal/servers**: Thread-safe store of the latest DZSA sync result per config port. Updated by port workers on successful sync; read by API handlers for `GET /api/v1/servers` and `GET /api/v1/servers/<port>` (JSON).
- **model**: DTOs for the DZSA API response (`QueryResponse`, `Result`, `Endpoint`, etc.).

---

## 4. Repository and package layout

```
dzsa-sync/
├── cmd/dzsasync/          # Entrypoint: main.go (flags, wiring, orchestration)
├── config/                 # YAML config load and validation
├── client/                 # DZSA API client (GET .../query/{ip}:{port})
├── model/                  # DZSA API response types
├── internal/
│   ├── ifconfig/           # ifconfig.net client and 10m IP loop
│   ├── metrics/            # OTel provider, Prometheus handler, HTTPRecorder, error classification
│   └── servers/            # Store of latest DZSA result per port; used by API handlers
├── package/                # Packaging assets (systemd, scripts, Dockerfile, base config)
├── docs/                   # User and contributor documentation
├── go.mod, Makefile, .goreleaser.yml, .github/workflows/
└── README.md
```

- **cmd/dzsasync**: The only `main` package. Parses `-config`, builds logger, metrics, HTTP client, DZSA client, ifconfig client, server store; starts the API server (metrics + /api/v1/servers) and goroutines; handles shutdown.
- **config**: No internal state beyond the config struct; used only at startup.
- **client**: Stateless except for the injected `*http.Client` and optional `HTTPRecorder`; used by port workers.
- **internal/ifconfig**: Holds cached `address` (mutex-protected); `Run()` runs in a dedicated goroutine and updates the cache; port workers read via `GetAddress()`.
- **internal/metrics**: Global meter provider is set in `NewProvider()`; `HTTPRecorder` is implemented here and used by both DZSA and ifconfig clients.

---

## 5. Concurrency and control flow

### 5.1 Goroutines

| Goroutine | Started in | Responsibility |
|-----------|------------|----------------|
| **API server** | main | Serves HTTP on configurable host/port (default `:8888`) with `/metrics` and `/api/v1/servers` (JSON); runs until shutdown. |
| **ifconfig loop** | main (if `detect_ip`) | Every 10 minutes calls ifconfig; on IP change updates cache and sends a trigger to each port worker. Blocks until context cancel. |
| **Port worker** (one per port) | main | Runs a 1-hour ticker and listens on a trigger channel; on tick or trigger, resolves IP (ifconfig or config), calls DZSA `Query(ip, port)`, logs result; on trigger also resets ticker. Exits when context is cancelled. |

Main goroutine: after starting the above, it blocks on `<-signalCtx.Done()`, then cancels the root context and waits for all port workers via `sync.WaitGroup`.

### 5.2 Trigger channels (IP change)

When ifconfig detects an IP change, it calls `onIPChanged(oldIP, newIP)`. That function sends a single non-blocking signal on each port’s trigger channel (`chan struct{}`, buffer 1). Each port worker’s select receives either:

- `ticker.C`: perform one sync (hourly).
- `trigger`: perform one sync **and** reset the 1-hour ticker (so the next sync is again in 1 hour from now).
- `ctx.Done()`: exit.

So an IP change causes one immediate sync per port and resets the interval without waiting for the next hourly tick.

### 5.3 Shared state

- **Current external IP**: Stored in `internal/ifconfig.Client.address` (mutex). Written by ifconfig `Run` (and by `SetAddress` when `detect_ip` is false). Read by port workers via `GetAddress()` and by main when passing static `ExternalIP` into ifconfig.
- **Config**: Read-only after load; no concurrent writes.
- **Metrics**: Recorded via OpenTelemetry; concurrency-safe.
- **Synced server data**: Stored in `internal/servers.Store` (RWMutex). Written by port workers on successful DZSA sync; read by API handlers for `GET /api/v1/servers` and `GET /api/v1/servers/<port>`.

---

## 6. Data flow

1. **Startup**  
   Config path → `config.NewFromFile` → validated `*config.Config`. Logger is created (JSON to file with lumberjack). Metrics provider and HTTP recorder are created. One shared `*http.Client` is used for both DZSA and ifconfig. DZSA client and ifconfig client are constructed with that client and the same recorder.

2. **IP resolution**  
   - If `!detect_ip`: `ifconfig.SetAddress(cfg.ExternalIP)`; no ifconfig loop.  
   - If `detect_ip`: ifconfig `Run()` goroutine starts; it does an initial GET, then every 10 minutes another GET; each successful response updates the cached IP and, if the IP changed, calls `onIPChanged`, which notifies all port workers.

3. **Per-port sync**  
   Each port worker, on tick or trigger: reads `ifconfig.GetAddress()` (or falls back to `cfg.ExternalIP`), then calls `dzsaClient.Query(ctx, ip, port)`. The client builds `GET https://dayzsalauncher.com/api/v1/query/{ip}:{port}`, performs the request, decodes JSON into `model.QueryResponse`, and records metrics. On success, the worker calls `store.Set(port, &resp.Result)` so the API can serve the latest data. Main logs the sync result (endpoint, name, players, etc.). Errors are logged and metrics still record the attempt.

4. **Shutdown**  
   SIGINT/SIGTERM → `signalCtx` is done → main cancels root context → API server is shut down via `Shutdown()`, ifconfig loop exits, each port worker sees `ctx.Done()` and returns → `WaitGroup` completes → process exits.

---

## 7. Key types and interfaces

- **config.Config**: `DetectIP`, `ExternalIP`, `Ports []int`, `API *APIConfig` (optional host/port for HTTP server). Validated by `Validate()` (e.g. external_ip required when !DetectIP, ports non-empty, no duplicate ports, api.port 1–65535 when set).
- **client.Client**: Interface with `Query(ctx, ip, port) (*model.QueryResponse, error)`. Implemented by `defaultClient` (uses base URL, `*http.Client`, optional `HTTPRecorder`).
- **internal/metrics.HTTPRecorder**: Interface with `RecordRequest(ctx, host, statusCode, errType string, duration time.Duration)`. Implemented by the OTel-based recorder; used by DZSA and ifconfig after each HTTP call. `host` is `"dzsa"` or `"ifconfig"`; `errType` comes from `metrics.ClassifyError(err, statusCode)` (e.g. `none`, `timeout`, `status_4xx`).
- **model.QueryResponse**: DZSA API response; contains `Result` (Name, Endpoint, Players, MaxPlayers, Version, Map, etc.).

---

## 8. Logging

- **Library**: `go.uber.org/zap` with JSON encoding.
- **Output**: Single file with rotation via `gopkg.in/natefinch/lumberjack.v2` (path, max size, max backups, max age, compress). Default path is a constant in main (e.g. `/var/log/dzsa-sync/dzsa-sync.log`).
- **What is logged**: Startup (metrics server, etc.), each DZSA sync result (endpoint, name, players, max_players, version, map), each ifconfig sync with detected IP, IP change and trigger, errors (sync failure, ifconfig failure), shutdown.

Logs are structured (zap fields); no separate “access log” for the metrics endpoint.

---

## 9. Metrics

- **Stack**: OpenTelemetry SDK with Prometheus exporter; metrics are served in Prometheus exposition format at `GET /metrics` on the configurable API server (default `:8888`).
- **Instruments** (namespace `dzsa_sync`):  
  - **RequestCount** (counter): One per HTTP request; attributes `host` (dzsa | ifconfig), `status_code`, `error` (e.g. none, timeout, status_4xx, status_5xx, decode_error, unknown).  
  - **RequestLatency** (histogram): Duration in seconds per request; attributes `host`, `status_code`.
- **Recording**: Done inside the DZSA client and ifconfig client after each request, using the shared `HTTPRecorder`. Error classification is in `internal/metrics` (`ClassifyError`).

---

## 10. Configuration

- **Source**: Single YAML file; path given by required `-config` flag.
- **Fields**: `detect_ip` (bool), `external_ip` (string), `ports` ([]int), `api` (optional: `host`, `port`). See [docs/configuration.md](configuration.md).
- **Validation**: On load, `Validate()` is called; invalid config causes process to exit with an error before any goroutines or servers start.

---

## 11. Shutdown and signals

- **Signals**: `SIGINT`, `SIGTERM` are captured via `signal.NotifyContext`. The resulting context (`signalCtx`) is passed to the ifconfig loop and to each port worker.
- **Order**: When the signal is received, main stops listening and cancels the root context. The metrics HTTP server is shut down with a short timeout. The ifconfig loop and port workers observe `ctx.Done()` and return. Main waits on the port workers’ `WaitGroup`, then exits. No second signal handler is required; a forceful kill (SIGKILL) will terminate the process without graceful shutdown.

This architecture keeps the process single-purpose (DZSA registration + IP detection + self-observability), with clear boundaries between config, clients, metrics, and orchestration, and with concurrency limited to a fixed set of goroutines and channels.
