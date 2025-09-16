set -euo pipefail

go mod tidy
go build -o ./cmd/server/server ./cmd/server
go build -o ./cmd/agent/agent ./cmd/agent

random_port() {
  python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()'
}

random_tempfile() {
  mktemp /tmp/metrics.XXXXXX
}

echo "==> Run increment #1"
./metricstest -test.v -test.run=^TestIteration1$ \
  -binary-path=cmd/server/server

echo "==> Run increment #2"
./metricstest -test.v -test.run=^TestIteration2[AB]*$ \
  -source-path=. \
  -agent-binary-path=cmd/agent/agent

echo "==> Run increment #3"
./metricstest -test.v -test.run=^TestIteration3[AB]*$ \
  -source-path=. \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server

echo "==> Run increment #4"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration4$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #5"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration5$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #6"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration6$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #7"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration7$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #8"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration8$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #9"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration9$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -file-storage-path=$TMP \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #10"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration10[AB]$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -database-dsn='postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable' \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #11"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration11$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -database-dsn='postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable' \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #12"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration12$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -database-dsn='postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable' \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #13"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration13$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -database-dsn='postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable' \
  -server-port=$PORT \
  -source-path=.

echo "==> Run increment #14"
PORT=$(random_port)
ADDRESS="localhost:${PORT}"
TMP=$(random_tempfile)
./metricstest -test.v -test.run=^TestIteration14$ \
  -agent-binary-path=cmd/agent/agent \
  -binary-path=cmd/server/server \
  -database-dsn='postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable' \
  -key="$TMP" \
  -serv
