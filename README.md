# Swap-rithms Algorithm Lab

A local laboratory for comparing real implementations of "find recently updated profile IDs" under load.

Run it:

```bash
go run .
```

Then open http://localhost:8080.

The app generates an in-memory profile dataset, serves the control panel, exposes `/profiles/recent`, and includes a load generator that calls the same HTTP endpoint. Latency and throughput shown in the UI come from measured executions, not estimated complexity labels.

Useful settings:

```bash
PROFILE_COUNT=100000 go run .
ADDR=:9099 go run .
```

The default dataset is 500,000 profiles. Use a smaller `PROFILE_COUNT` if you are on a memory-constrained machine.

Endpoints:

- `GET /profiles/recent?window=5m&ids=true` returns IDs updated inside a recent time window.
- `GET /profiles/recent?since=2026-06-20T12:00:00Z&ids=false` runs the lookup and returns only the count plus measured lookup time.
- `POST /api/algorithm` with `{"name":"binary_search"}` switches implementations.
- `POST /api/load/start` with `{"rate":50,"durationSeconds":60,"windowSeconds":300}` starts load.
- `POST /api/load/stop` stops load.
- `GET /api/stats` returns UI metrics.
- `GET /metrics` returns Prometheus-style counters and gauges.

Implemented algorithms:

- `slice_scan`: full slice scan, `O(N)`.
- `binary_search`: sorted slice plus binary search, `O(log N + K)`.
- `bucketed_index`: minute bucket index, `O(log B + K)`.
- `map_scan`: full hash map scan, `O(N)`.
- `parallel_scan`: goroutine fan-out over the full slice, `O(N/C + merge)`.
