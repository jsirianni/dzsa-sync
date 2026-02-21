# Development Guide

This document explains how to develop, test, lint, and release **dzsa-sync**. It is intended for human contributors and for automated agents (e.g. LLMs) that need to make changes with minimal external context.

---

## 1. Prerequisites

- **Go**: Version defined in `go.mod` (e.g. 1.24). Use the same or a compatible version locally. CI uses `go-version-file: go.mod`.
- **Optional**: `make` for running lint, secure, test, build. On Windows, you can run the underlying commands directly (see below).
- **Optional**: Docker and Docker Buildx if you need to build or test the container image locally.

No other runtimes or databases are required for normal development.

---

## 2. Repository layout (quick reference)

| Path | Purpose |
|------|--------|
| `cmd/dzsasync/main.go` | Entrypoint: flags, config load, logger, metrics, HTTP client, DZSA client, ifconfig client, metrics server, port workers, shutdown. |
| `config/` | YAML config struct, `NewFromFile`, `Validate`. |
| `client/` | DZSA API client (`Query(ctx, ip, port)`), interface + default implementation. |
| `model/` | DZSA API response types (`QueryResponse`, `Result`, `Endpoint`, etc.). |
| `internal/ifconfig/` | ifconfig.net client: `Get(ctx)`, `Run(ctx, onChanged)`, `GetAddress()`, `SetAddress()`, `BaseURL` (for tests). |
| `internal/metrics/` | OTel provider, Prometheus handler, `HTTPRecorder`, `ClassifyError`, error consts. |
| `package/` | Packaging: systemd unit, scripts (pre/post install/remove), base config, Dockerfile. |
| `docs/` | User and contributor docs (configuration, installation, architecture, this guide). |
| `go.mod` | Module and dependencies; includes `tool` block for revive and gosec. |
| `Makefile` | Targets: `lint`, `secure`, `test`, `build`, `tidy`. |
| `.github/workflows/` | CI (`ci.yml`) and release (`release.yml`). |

See [architecture.md](architecture.md) for how these pieces interact.

---

## 3. Building and running locally

### Build

```bash
go build -o dzsa-sync ./cmd/dzsasync
```

Or, if you have `make`:

```bash
make build
```

The binary is named `dzsa-sync` (or `dzsa-sync.exe` on Windows). It is ignored by git (see `.gitignore`).

### Run

A config file is required. Example minimal config (e.g. `config.yaml` in the repo root or a temp dir):

```yaml
detect_ip: false
external_ip: "203.0.113.10"   # use a real IP or test value
ports: [2424]
```

Then:

```bash
./dzsa-sync -config /path/to/config.yaml
```

- The process will write logs to the default path (see `defaultLogPath` in `main.go`). Ensure that path is writable or the process will fail at startup.
- Metrics are served at `http://localhost:8888/metrics`.
- To stop: Ctrl+C (SIGINT) or send SIGTERM; the process shuts down gracefully.

For a quick local test with IP detection (hits real ifconfig.net):

```yaml
detect_ip: true
ports: [2424]
```

---

## 4. Testing

### Run all tests

```bash
go test ./...
```

With race detector (recommended where supported, e.g. Linux/macOS with CGO):

```bash
go test -race ./...
```

On Windows, `-race` may require CGO; if it fails, run without `-race`.

### Run tests for specific packages

```bash
go test ./config/...
go test ./internal/ifconfig/...
go test ./client/...
```

### Test structure and conventions

- **config**: `config_test.go` uses table-driven tests for `Validate()` (valid/invalid cases) and `NewFromFile()` (valid file, missing file, invalid YAML, validation failure). Uses `t.TempDir()` and `os.WriteFile` for temporary config files.
- **client**: `client_test.go` tests `buildEndpoint` (URL construction) with table-driven cases. No live HTTP calls; DZSA API is not mocked in the client package.
- **internal/ifconfig**: `ifconfig_test.go` uses an `httptest.Server` as a mock ifconfig server. The ifconfig `Client` has a `BaseURL` field; in tests it is set to `server.URL` so `Get()` and `Run()` hit the mock. Tests cover: success, non-200 status, invalid JSON, empty IP response, `GetAddress`/`SetAddress`, `Run` initial fetch and shutdown, and `New(..., nil, ...)` default client.

When adding or changing behavior, add or update tests in the same package. Prefer table-driven tests for multiple cases; use `httptest.Server` or injectable interfaces (e.g. `HTTPRecorder`) to avoid calling real external APIs in unit tests.

---

## 5. Linting and security

### Lint (revive)

```bash
go tool revive ./...
```

Or:

```bash
make lint
```

Revive is declared in `go.mod` under the `tool` directive; no separate install is needed if your Go version supports `go tool` (Go 1.24+). Fix any reported issues. The only exception in the repo is in `internal/metrics/recorder.go`: the package name `metrics` triggers a var-naming rule; it is explicitly disabled there with a revive directive.

### Security (gosec)

```bash
go tool gosec -exclude=G704 ./...
```

Or:

```bash
make secure
```

G704 (SSRF) is excluded because the only outbound HTTP URLs are fixed (DZSA and ifconfig.net); the config does not allow user-supplied URLs. Fix any other findings before submitting.

---

## 6. Code style and conventions

- **Formatting**: Use `gofmt` or `goimports`. CI does not enforce a formatter in the workflow; keeping code formatted is expected.
- **Packages**: Keep packages focused. Put DZSA API types in `model`, config in `config`, API client in `client`, internal helpers in `internal/*`.
- **Interfaces**: Prefer small interfaces (e.g. `client.Client`, `metrics.HTTPRecorder`) so that tests can inject mocks or no-ops.
- **Errors**: Use `fmt.Errorf("...: %w", err)` for wrapping; validate config at startup and return clear errors.
- **Logging**: Use the injected `*zap.Logger`; do not log from packages that do not receive a logger (or use a no-op logger in tests).
- **Concurrency**: Use `context.Context` for cancellation; avoid global mutable state; use channels or mutexes as documented in [architecture.md](architecture.md).

---

## 7. Adding or changing features

### Adding a new config field

1. Add the field to `config.Config` in `config/config.go` with the appropriate `yaml` tag.
2. Update `Validate()` if the new field has constraints (e.g. required when another is set).
3. Use the field in `cmd/dzsasync/main.go` (or in the appropriate package that receives config).
4. Update [docs/configuration.md](configuration.md) and, if relevant, [architecture.md](architecture.md).
5. Add or extend tests in `config/config_test.go`.

### Adding a new external dependency (HTTP, library)

1. Add the dependency in the right package; keep `client` and `internal/ifconfig` responsible for their own URLs and parsing.
2. Run `go mod tidy`.
3. If the code makes HTTP requests, consider recording them via `metrics.HTTPRecorder` (and, if needed, extend `ClassifyError` or error consts in `internal/metrics`).
4. Add unit tests (e.g. mock server or interface) so CI stays green.

### Changing the DZSA or ifconfig contract

- **DZSA**: Response shape lives in `model/`. If the API adds fields, add them to the structs; existing callers can ignore them. If the URL or method changes, update `client` and any tests or docs that reference the endpoint.
- **ifconfig**: Request/response are in `internal/ifconfig`. The client supports `BaseURL` for tests; keep that so tests do not hit the real service.

### Adding metrics

- Define new instruments in `internal/metrics` (e.g. in `provider.go` or a dedicated file). Use the existing meter and namespace (`dzsa_sync`).
- Record from the place that has the information (e.g. client code or main). Prefer the existing `HTTPRecorder` pattern for HTTP-related metrics.
- Document new metrics in the README or [docs/configuration.md](configuration.md) if they are user-visible.

---

## 8. Dependencies and tools

- **Adding a library**: `go get <module>@<version>` (or `@latest`); then `go mod tidy`. Prefer stable, maintained libraries; avoid unnecessary dependencies.
- **Tools (revive, gosec)**: They are in the `tool` block in `go.mod`. Run them with `go tool revive` and `go tool gosec`; do not commit a separate tools module or install script unless the project policy changes.
- **Upgrading Go**: Update the version in `go.mod`. Run tests and lint locally; CI will use the new version from `go.mod`.

---

## 9. CI/CD

### CI (`.github/workflows/ci.yml`)

- **Triggers**: Push to `main`, and any pull request.
- **Jobs** (run concurrently): **lint** (revive), **secure** (gosec), **test** (go test). Each job checks out the repo and uses `go-version-file: go.mod`.
- **Concurrency**: One run per branch/ref; newer runs cancel in-progress ones.

Before pushing, run locally (or equivalent):

```bash
make lint
make secure
make test
```

so CI stays green.

### Release (`.github/workflows/release.yml`)

- **Trigger**: Push of a tag matching `v*` (e.g. `v1.0.0`).
- **Steps**: Checkout (with unshallow fetch for versioning), setup Go, Docker Buildx, QEMU, login to GHCR, run `goreleaser release`.
- **Artifacts**: Defined in `.goreleaser.yml`: binaries, Linux packages (deb/rpm), Docker images (multi-arch), and manifests. Release is created on GitHub with uploads; Docker images are pushed to the configured registry (e.g. ghcr.io).

Do not trigger a release by pushing a tag unless you intend to publish a new version.

---

## 10. Releasing (human process)

1. **Version**: Decide the next version (e.g. `v1.0.1`). Ensure `main` is in a good state (CI green, changelog/docs updated if desired).
2. **Tag**: Create and push an annotated tag, e.g. `git tag -a v1.0.1 -m "Release v1.0.1"` then `git push origin v1.0.1`.
3. **Workflow**: The release workflow runs, runs GoReleaser, creates the GitHub release, uploads binaries and packages, and builds/pushes Docker images.
4. **Verify**: Check the GitHub release page and the container registry for the new assets.

To test the release process without publishing, use GoReleaser’s snapshot mode locally: `goreleaser release --snapshot --skip=publish --clean`. This builds artifacts into `dist/` without uploading.

---

## 11. Packaging (Goreleaser, Linux, Docker)

- **Goreleaser config**: `.goreleaser.yml` defines builds (linux/amd64, arm64), nfpms (deb/rpm), and Docker. Linux packages install the binary under `/usr/bin`, config under `/etc/dzsa-sync`, and a systemd unit; pre/post scripts are under `package/scripts/`.
- **Scripts**: Blitz-style: preinstall creates user/group; postinstall sets config dir ownership and daemon-reload; preremove stops the service; postremove only daemon-reload. Scripts are shell (`#!/usr/bin/env sh`, `set -eu`) for portability.
- **Dockerfile**: Multi-stage; final image is `FROM scratch` with CA certificates and binary; user is non-root. Goreleaser injects the built binary; the Dockerfile does not build the Go binary itself.
- **Changing package layout or scripts**: Edit `.goreleaser.yml` and the files under `package/`; test with `goreleaser release --snapshot --skip=publish` and, if possible, install the generated deb/rpm in a clean VM or container.

---

## 12. Debugging and common tasks

- **“No external IP” / sync skipped**: If `detect_ip` is true, ensure the host can reach ifconfig.net and that the initial 2-second window (or the 10-minute loop) has run. If `detect_ip` is false, ensure `external_ip` is set in config.
- **Sync fails (timeout, 4xx/5xx)**: Check logs for the endpoint and error. Metrics will show request count and latency by host and status code. Verify the DZSA API is up and the given IP/port are reachable from the internet.
- **Tests fail after changing config or client**: Update `config_test.go` or `client_test.go`; if you changed ifconfig, ensure tests set `Client.BaseURL` to the httptest.Server URL.
- **Revive complains about package name**: Only `internal/metrics` is exempt (see comment in `recorder.go`). Other packages should follow the rule or be fixed.
- **Gosec G704**: Excluded by design; do not add user-controlled URLs without a security review.

---

## 13. Checklist for contributors and LLMs

When making changes:

1. **Read** [architecture.md](architecture.md) to see where your change fits (config, client, ifconfig, metrics, main).
2. **Implement** in the appropriate package; keep interfaces small and testable.
3. **Add or update tests** in the same package; use mocks (e.g. httptest, injectable recorder) where appropriate.
4. **Run** `go test ./...`, `go tool revive ./...`, and `go tool gosec -exclude=G704 ./...` (or `make test`, `make lint`, `make secure`).
5. **Update docs** if you changed config, behavior, or public APIs: `docs/configuration.md`, `docs/installation.md`, README, or this guide.
6. **Commit** with a clear message; open a PR so CI runs. Ensure CI passes before merge.

When answering “how do I …?”:

- **Run locally**: Build with `go build -o dzsa-sync ./cmd/dzsasync`, run with `-config <path>`. See section 3.
- **Add a test**: See section 4 and the existing `*_test.go` files; use table-driven tests and, for HTTP, `httptest.Server` or `Client.BaseURL`.
- **Release**: Tag `v*` and push; see section 10. Do not run release workflow by hand unless you own the repo and intend to publish.

This should give future contributors and LLMs enough context and direction to develop dzsa-sync with minimal external input.
