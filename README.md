# Swap-rithms Algorithm Lab

A local laboratory for comparing real implementations of practical profile-data algorithms under load.

The lab can run the same scenario algorithms in Go, Python, or TypeScript. Go runs in-process. Python and TypeScript run as long-lived worker processes initialized with the same deterministic dataset, so the metrics come from executing code in the selected runtime without paying process startup on every request.

Run it with local language runtimes:

```bash
go run .
```

Then open http://localhost:8080.

Python requires `python3`. TypeScript requires a recent `node` with TypeScript type stripping support, such as Node 24.

Or run it with Docker only:

```bash
docker build -t swap-rithms .
docker run --rm -p 8080:8080 swap-rithms
```

Then open http://localhost:8080. The local Docker image includes the compiled Go service, `python3`, and Node 24, so you do not need Go, Python, or Node installed on the host.

Use environment variables the same way in Docker:

```bash
docker run --rm -p 9099:9099 -e ADDR=:9099 -e PROFILE_COUNT=100000 swap-rithms
```

The app generates an in-memory profile dataset with timestamps, scores, regions, bios, and searchable text. The control panel lets you choose a scenario, language, and algorithm; the load generator calls the matching scenario endpoint. Latency and throughput shown in the UI come from measured executions, not estimated complexity labels.

Useful settings:

```bash
PROFILE_COUNT=100000 go run .
ADDR=:9099 go run .
```

The default dataset is 500,000 profiles. Use a smaller `PROFILE_COUNT` if you are on a memory-constrained machine.

## iximiuz Labs rootFS deployment

The iximiuz Labs playground uses a custom rootFS image built from `playground/iximiuz/Dockerfile`. The rootFS build script creates a temporary Docker context, builds the Go service into the image, installs Python and Node 24 for the worker runtimes, and updates `playground/iximiuz/manifest.yaml` to point at the selected image tag.

Authenticate to GHCR before pushing:

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
```

Build the rootFS image locally with a specific tag:

```bash
IMAGE_TAG=v2 scripts/build-rootfs-image.sh
```

Build and push the same tag to GHCR:

```bash
IMAGE_TAG=v2 PUSH_ROOTFS_IMAGE=1 scripts/build-rootfs-image.sh
```

The default package is `ghcr.io/lpmi-13/swap-rithms-rootfs`. To push to a different GHCR package:

```bash
ROOTFS_IMAGE_REPO=ghcr.io/<owner>/swap-rithms-rootfs IMAGE_TAG=v2 PUSH_ROOTFS_IMAGE=1 scripts/build-rootfs-image.sh
```

To update the iximiuz Labs manifest to a given rootFS tag, run the build script with that tag and leave manifest updates enabled:

```bash
IMAGE_TAG=v2 scripts/build-rootfs-image.sh
```

This writes `oci://ghcr.io/lpmi-13/swap-rithms-rootfs:v2` into `playground/iximiuz/manifest.yaml` and updates the script's checked-in default tag. Use `UPDATE_MANIFEST=0` when you want to build or push an image without changing checked-in playground files.

## Architecture

Swap-rithms keeps the HTTP API, UI, load generator, and metrics aggregation in the Go service. The active implementation is a triple of values: `scenario`, `language`, and `algorithm`. Changing any of those values updates the active triple under a lock, so new requests immediately use the selected implementation without restarting the service.

```text
                       browser control panel
              scenario + language + algorithm select
                                |
                                | POST /api/algorithm
                                v
+-------------------------------+--------------------------------+
|                          Go lab service                         |
|                                                                |
|  active implementation: scenario + language + algorithm          |
|  examples: lookup:go:slice_scan, top_k:python:top_k_min_heap,   |
|            text_search:typescript:text_inverted_index            |
|                                                                |
|  /profiles/recent  /profiles/top  /profiles/sorted              |
|  /profiles/search  /profiles/cache                              |
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
|  scenario algorithm  scenario algorithm  scenario algorithm     |
|        |                  |                  |                 |
|        +------------------+------------------+-----------------+
|                                |                               |
|                                v                               |
|                 result count, optional IDs, elapsed time        |
|                                |                               |
|                                v                               |
|        metrics keyed by scenario:language:algorithm             |
+-------------------------------+--------------------------------+
                                |
                                v
                    UI charts and /metrics output
```

The Go runtime calls the selected algorithm directly in the same process. Python and TypeScript run as persistent worker processes using a line-delimited JSON protocol over stdin/stdout. At startup, each worker builds the same deterministic profile dataset and local indexes from `profileCount` and `generatedAt`, then handles `run` requests for the currently selected scenario algorithm.

Hot-swapping works at three levels:

- Scenario swap: change the business task, such as `lookup` to `top_k`.
- Algorithm swap: keep the selected scenario and language and change only the algorithm, such as `lookup:go:slice_scan` to `lookup:go:binary_search`.
- Language swap: keep the selected scenario and algorithm and change only the runtime, such as `top_k:go:top_k_min_heap` to `top_k:python:top_k_min_heap`.

Existing in-flight requests finish on the implementation they already selected. Subsequent requests use the new active implementation. Metrics are stored per implementation key, so switching from `top_k:python:top_k_min_heap` to `top_k:typescript:top_k_min_heap` creates separate latency and throughput series.

Endpoints:

- `GET /profiles/recent?window=5m&ids=true` returns IDs updated inside a recent time window.
- `GET /profiles/recent?since=2026-06-20T12:00:00Z&ids=false` runs the lookup and returns only the count plus measured lookup time.
- `GET /profiles/top?k=100&ids=true` returns the top K profiles by generated activity score.
- `GET /profiles/sorted?limit=100&ids=true` sorts a deterministic profile sample by score and returns the requested prefix.
- `GET /profiles/search?q=platform&limit=100&ids=true` searches normalized name, email, region, and bio text.
- `GET /profiles/cache?id=42&ids=true` serves one profile request through the selected cache strategy.
- `POST /api/algorithm` with `{"scenario":"top_k","language":"python","name":"top_k_min_heap"}` switches implementations.
- `POST /api/load/start` with `{"rate":50,"durationSeconds":0,"windowSeconds":300}` starts load indefinitely; set `durationSeconds` to a positive value to stop automatically.
- `POST /api/load/rate` with `{"rate":100}` updates the active load generator rate without restarting it.
- `POST /api/load/stop` stops load.
- `GET /api/stats` returns UI metrics.
- `GET /metrics` returns Prometheus-style counters and gauges.

Implemented scenarios:

- Lookup and indexing: `slice_scan`, `binary_search`, `bucketed_index`, `map_scan`, `parallel_scan`.
- Top-K and ranking: `top_k_full_sort`, `top_k_min_heap`, `top_k_quickselect`, `top_k_bucketed`, `top_k_streaming`.
- Sorting: `sort_insertion`, `sort_merge`, `sort_quick`, `sort_heap`, `sort_counting`, `sort_radix`, `sort_builtin`.
- Caching strategies: `cache_none`, `cache_fifo`, `cache_lru`, `cache_lfu`, `cache_random`, `cache_ttl`.
- Text search: `text_naive`, `text_lowercase`, `text_kmp`, `text_boyer_moore`, `text_trie_prefix`, `text_inverted_index`.
