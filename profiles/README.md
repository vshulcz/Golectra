## Benchmarks & memory profiling

This folder documents how to re-run every performance benchmark and interpret the resulting `.pprof` files.

### Metrics service – `Service.UpsertBatch`

1. Build the standalone benchmark once (re-usable between runs):
   ```bash
   go test -c ./internal/services/metrics -o metrics_bench.test
   ```
2. Capture a baseline profile (before changes):
   ```bash
   mkdir -p profiles
   ./metrics_bench.test \
     -test.run=^$ \
     -test.bench=BenchmarkServiceUpsertBatch \
     -test.benchmem \
     -test.memprofile=profiles/base.pprof
   ```
3. Re-run the same command with `-test.memprofile=profiles/result.pprof` after making changes to the service.
4. Compare both:
   ```bash
   go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
   ```

Latest diff (after trimming IDs once + deduping names in-place):

```
Showing nodes accounting for 389.16MB, 13.35% of 2915.98MB total
    flat  flat%   sum%        cum   cum%
 -741.44MB 25.43% 13.35%  -741.44MB 25.43%  github.com/vshulcz/Golectra/internal/services/metrics.metricNames
  389.16MB 13.35% 13.35%   389.16MB 13.35%  github.com/vshulcz/Golectra/internal/services/metrics.(*Service).UpsertBatch
```

Resulting benchmark: `BenchmarkServiceUpsertBatch-8 257097 4506 ns/op 13184 B/op 2 allocs/op`.

### HTTP API – `/updates` JSON handler

```
go test ./internal/adapters/http/ginserver \
  -run ^$ \
  -bench BenchmarkHandlerUpdateMetricsBatchJSON \
  -benchmem \
  -memprofile=profiles/handler.pprof
go tool pprof -top ginserver.test profiles/handler.pprof
```

After bypassing Gin’s reflection-heavy binder and decoding into a pooled slice:

```
BenchmarkHandlerUpdateMetricsBatchJSON-8  10000  101473 ns/op  54666 B/op  448 allocs/op
Top allocators: encoding/json.Decoder.refill (53%), Service.UpsertBatch (25%).
```

### HTTP publisher – gzip JSON client

```
go test ./internal/adapters/publisher/httpjson \
  -run ^$ \
  -bench BenchmarkClientSendBatch \
  -benchmem \
  -memprofile=profiles/httpjson.pprof
go tool pprof -top httpjson.test profiles/httpjson.pprof
```

Pooling gzip writers + buffers slashed allocations from ~900KB/op to ~130KB/op:

```
BenchmarkClientSendBatch-8  6228  190565 ns/op  130068 B/op  103 allocs/op
compress/flate.NewWriter now accounts for ~37% of alloc_space (was ~74%).
```

### Agent service – `reportOnce`

```
go test ./internal/services/agent \
  -run ^$ \
  -bench BenchmarkAgentReportOnce \
  -benchmem \
  -memprofile=profiles/agent.pprof
go tool pprof -top agent.test profiles/agent.pprof
```

Reusing the batch slice instead of allocating per tick reduced the footprint to 3KB/op:

```
BenchmarkAgentReportOnce-8  211834  5672 ns/op  3200 B/op  400 allocs/op
alloc_space is now entirely attributed to Service.buildBatch (expected, since it controls the reusable buffer).
```