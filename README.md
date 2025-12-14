# Golectra

[![Go Report Card](https://goreportcard.com/badge/github.com/vshulcz/Golectra)](https://goreportcard.com/report/github.com/vshulcz/Golectra)
[![codecov](https://codecov.io/gh/vshulcz/Golectra/branch/main/graph/badge.svg)](https://codecov.io/gh/vshulcz/Golectra)
[![CI](https://github.com/vshulcz/Golectra/workflows/autotests/badge.svg)](https://github.com/vshulcz/Golectra/actions)
[![Lint](https://github.com/vshulcz/Golectra/workflows/lint/badge.svg)](https://github.com/vshulcz/Golectra/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/vshulcz/Golectra.svg)](https://pkg.go.dev/github.com/vshulcz/Golectra)
[![License](https://img.shields.io/github/license/vshulcz/Golectra)](LICENSE)

Golectra is a tiny, self-hosted metrics stack in pure Go:
a lightweight agent collects Go runtime + host metrics and ships them (gzipped JSON) to a server with a simple HTTP API and optional integrity checks.

## Features

* Agent samples Go runtime.MemStats + system metrics (CPU, RAM) via gopsutil.
* Gzipped JSON transport with automatic retries and rate-limited workers.
* Optional HashSHA256 header (HMAC-like) to protect request/response integrity.
* Storage: Postgres (auto-migrate) or in-memory with JSON file persistence & restore.
* HTTP API (Gin) + a quick HTML dashboard at / (tables of gauges/counters).
* Zero deps at runtime (server/agent binaries). Go 1.21+.

## Quick start

1) Run the server
```bash
go run ./cmd/server -a :8080 -f metrics-db.json -i 300
# Flags:
# -a listen address, -f file path for snapshots, -i store interval seconds (0 = sync)
```

2) Run the agent
```bash
go run ./cmd/agent -a http://localhost:8080 -r 10 -p 2 -l 2
# -r report interval (s), -p poll interval (s), -l parallel senders
```

3) Build with version info
To include build version, date, and commit in the binaries, use the following commands:

```bash
go build -o bin/server \
  -ldflags "\
    -X 'main.buildVersion=v1.2.5' \
    -X 'main.buildDate=2025-12-14T10:00:00Z' \
    -X 'main.buildCommit=abc1234' \
  " ./cmd/server

go build -o bin/agent \
  -ldflags "\
    -X 'main.buildVersion=v1.2.5' \
    -X 'main.buildDate=2025-12-14T10:00:00Z' \
    -X 'main.buildCommit=abc1234' \
  " ./cmd/agent
```

Open http://localhost:8080 to see metrics.

## API (HTTP)

Path params:

* Update one (plain text):
  `POST /update/:type/:name/:value` where `type` is `gauge` or `counter`
* Read one (plain text):
  `GET /value/:type/:name`
* Read HTML dashboard:
  `GET /`
* JSON (recommended):
```bash
# Upsert one
curl -X POST http://localhost:8080/update \
  -H "Content-Type: application/json" \
  -d '{"id":"Alloc","type":"gauge","value":12345}'

# Read one
curl -X POST http://localhost:8080/value \
  -H "Content-Type: application/json" \
  -d '{"id":"Alloc","type":"gauge"}'

# Upsert batch
curl -X POST http://localhost:8080/updates \
  -H "Content-Type: application/json" \
  -d '[{"id":"foo","type":"gauge","value":1.23},{"id":"bar","type":"counter","delta":7}]'

# Health
curl http://localhost:8080/ping
```

## Integrity header (optional but recommended)

Start both server and agent with the same secret key (-k or KEY env).
The server will validate HashSHA256 for incoming POST bodies and will always set a HashSHA256 response header.

Example with curl:
```bash
BODY='{"id":"temp","type":"gauge","value":42}'
KEY='your-secret'
HASH=$(printf '%s' "$BODY$KEY" | openssl dgst -sha256 -binary | xxd -p -c 256)

curl -X POST http://localhost:8080/update \
  -H "Content-Type: application/json" \
  -H "HashSHA256: $HASH" \
  -d "$BODY" -i
```

## Audit trail

Set `--audit-file /path/to/audit.ndjson` (or `AUDIT_FILE`) to append newline-delimited JSON events locally, `--audit-url https://audit.example.com/hook` (or `AUDIT_URL`) to POST events to a remote service, or enable both. Each successful metrics write triggers a fan-out notification to every configured sink via the Observer pattern, using this payload:

```json
{
  "ts": 1735584000,
  "metrics": ["Alloc", "PollCount"],
  "ip_address": "192.168.0.42"
}
```

Delivery failures are logged but never bubble up to the HTTP handlers, so metric ingestion stays available even if an audit sink is down.


## Configuration

You can use ENV, CLI flags, or defaults (ENV > CLI > defaults).

#### Server
| Setting          | ENV                 | Flag           | Default           | Notes                                                                 |
| ---------------- | ------------------- | -------------- | ----------------- | --------------------------------------------------------------------- |
| Address          | `ADDRESS`           | `-a`            | `:8080`           | `host:port`                                                           |
| File storage     | `FILE_STORAGE_PATH` | `-f`            | `metrics-db.json` | JSON snapshot file                                                    |
| Postgres DSN     | `DATABASE_DSN`      | `-d`            | *empty*           | e.g. `postgres://user:pass@localhost:5432/db?sslmode=disable`         |
| Secret key       | `KEY`               | `-k`            | *empty*           | enables `HashSHA256`                                                  |
| Store interval   | `STORE_INTERVAL`    | `-i`            | `300s`            | `0` = sync writes                                                     |
| Restore on start | `RESTORE`           | `-r`            | `false`           | load from file at boot                                                |
| Audit file       | `AUDIT_FILE`        | `--audit-file`  | *empty*           | newline-delimited JSON audit log fan-out target (disabled when empty) |
| Audit URL        | `AUDIT_URL`         | `--audit-url`   | *empty*           | HTTP POST endpoint for audit events (disabled when empty)             |

#### Agent
| Setting         | ENV               | Flag | Default                 | Notes                   |
| --------------- | ----------------- | ---- | ----------------------- | ----------------------- |
| Server address  | `ADDRESS`         | `-a` | `http://localhost:8080` | URL or `host:port`      |
| Secret key      | `KEY`             | `-k` | *empty*                 | adds `HashSHA256`       |
| Report interval | `REPORT_INTERVAL` | `-r` | `10s`                   | send frequency          |
| Poll interval   | `POLL_INTERVAL`   | `-p` | `2s`                    | sample frequency        |
| Rate limit      | `RATE_LIMIT`      | `-l` | `1`                     | concurrent send workers |

## Metrics you’ll see

Go runtime gauges like Alloc, HeapAlloc, NumGC, PauseTotalNs, plus host gauges TotalMemory, FreeMemory, and per-core CPUutilization{N}; counters include PollCount, etc.
