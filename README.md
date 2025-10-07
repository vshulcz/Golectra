# Golectra

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


## Configuration

You can use ENV, CLI flags, or defaults (ENV > CLI > defaults).

#### Server
| Setting          | ENV                 | Flag | Default           | Notes                                                         |
| ---------------- | ------------------- | ---- | ----------------- | ------------------------------------------------------------- |
| Address          | `ADDRESS`           | `-a` | `:8080`           | `host:port`                                                   |
| File storage     | `FILE_STORAGE_PATH` | `-f` | `metrics-db.json` | JSON snapshot file                                            |
| Postgres DSN     | `DATABASE_DSN`      | `-d` | *empty*           | e.g. `postgres://user:pass@localhost:5432/db?sslmode=disable` |
| Secret key       | `KEY`               | `-k` | *empty*           | enables `HashSHA256`                                          |
| Store interval   | `STORE_INTERVAL`    | `-i` | `300s`            | `0` = sync writes                                             |
| Restore on start | `RESTORE`           | `-r` | `false`           | load from file at boot                                        |

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