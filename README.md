# Swap-rithms Algorithm Lab

A local laboratory for comparing real implementations of "find recently updated profile IDs" under load.

The lab can run the same lookup algorithms in Go, Python, or TypeScript. Go runs in-process. Python and TypeScript run as long-lived worker processes initialized with the same deterministic dataset, so the metrics come from executing code in the selected runtime without paying process startup on every request.

Run it:

```bash
go run .
```

Then open http://localhost:8080.

Python requires `python3`. TypeScript requires a recent `node` with TypeScript type stripping support, such as Node 24.

The app generates an in-memory profile dataset, serves the control panel, exposes `/profiles/recent`, and includes a load generator that calls the same HTTP endpoint. Latency and throughput shown in the UI come from measured executions, not estimated complexity labels.

Useful settings:

```bash
PROFILE_COUNT=100000 go run .
ADDR=:9099 go run .
```

The default dataset is 500,000 profiles. Use a smaller `PROFILE_COUNT` if you are on a memory-constrained machine.

## Architecture

Swap-rithms keeps the HTTP API, UI, load generator, and metrics aggregation in the Go service. The active implementation is a pair of values: `language` and `algorithm`. Changing either value updates that active pair under a lock, so new lookup requests immediately use the selected implementation without restarting the service.

```text
                       browser control panel
                     language + algorithm select
                                |
                                | POST /api/algorithm
                                v
+-------------------------------+--------------------------------+
|                          Go lab service                         |
|                                                                |
|  active implementation: language + algorithm                    |
|  examples: go:slice_scan, python:binary_search,                 |
|            typescript:bucketed_index                            |
|                                                                |
|  /profiles/recent                                               |
|        |                                                       |
|        v                                                       |
|  choose active runtime                                          |
|        |                                                       |
|        +------------------+------------------+-----------------+
|        |                  |                  |                 |
|        v                  v                  v                 |
|  Go runtime          Python worker       TypeScript worker      |
|  in-process          long-lived process  long-lived Node process|
|        |                  |                  |                 |
|        v                  v                  v                 |
|  selected Go         selected Python     selected TypeScript    |
|  algorithm           algorithm           algorithm              |
|        |                  |                  |                 |
|        +------------------+------------------+-----------------+
|                                |                               |
|                                v                               |
|                 result count, optional IDs, elapsed time        |
|                                |                               |
|                                v                               |
|                 metrics keyed by active implementation          |
+-------------------------------+--------------------------------+
                                |
                                v
                    UI charts and /metrics output
```

The Go runtime calls the selected finder directly in the same process. Python and TypeScript run as persistent worker processes using a line-delimited JSON protocol over stdin/stdout. At startup, each worker builds the same deterministic profile dataset and local indexes from `profileCount` and `generatedAt`, then handles `find` requests for the currently selected algorithm.

Hot-swapping works at two levels:

- Algorithm swap: keep the selected language and change only the algorithm, such as `go:slice_scan` to `go:binary_search`.
- Language swap: keep the selected algorithm and change only the runtime, such as `go:binary_search` to `python:binary_search` or `typescript:binary_search`.

Existing in-flight requests finish on the implementation they already selected. Subsequent requests use the new active implementation. Metrics are stored per implementation key, so switching from `python:binary_search` to `typescript:binary_search` creates separate latency and throughput series.

Endpoints:

- `GET /profiles/recent?window=5m&ids=true` returns IDs updated inside a recent time window.
- `GET /profiles/recent?since=2026-06-20T12:00:00Z&ids=false` runs the lookup and returns only the count plus measured lookup time.
- `POST /api/algorithm` with `{"language":"python","name":"binary_search"}` switches implementations.
- `POST /api/load/start` with `{"rate":50,"durationSeconds":60,"windowSeconds":300}` starts load.
- `POST /api/load/stop` stops load.
- `GET /api/stats` returns UI metrics.
- `GET /metrics` returns Prometheus-style counters and gauges.

Implemented algorithms:

- `slice_scan`: full slice scan, `O(N)`.
- `binary_search`: sorted slice plus binary search, `O(log N + K)`.
- `bucketed_index`: minute bucket index, `O(log B + K)`.
- `map_scan`: full hash map scan, `O(N)`.
- `parallel_scan`: splits the full scan into worker-sized chunks, `O(N/C + merge)`.
