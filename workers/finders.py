#!/usr/bin/env python3
import bisect
import concurrent.futures
import json
import os
import sys
import time

DAY_NS = 24 * 60 * 60 * 1_000_000_000
MS_NS = 1_000_000

profiles = []
sorted_profiles = []
sorted_updated_ns = []
profile_map = {}
buckets = {}
minutes = []
workers = min(max(os.cpu_count() or 2, 2), 16)


def build_dataset(count, generated_at_ns):
    global profiles, sorted_profiles, sorted_updated_ns, profile_map, buckets, minutes

    step_ns = DAY_NS // max(count, 1)
    profiles = []
    profile_map = {}
    buckets = {}

    for i in range(count):
        profile_id = i + 1
        updated_ns = generated_at_ns - DAY_NS + (i * step_ns) + (((i * 37) % 997) * MS_NS)
        profiles.append((profile_id, updated_ns))
        profile_map[profile_id] = updated_ns
        minute = (updated_ns // 1_000_000_000) // 60
        buckets.setdefault(minute, []).append(profile_id)

    sorted_profiles = sorted(profiles, key=lambda profile: (profile[1], profile[0]))
    sorted_updated_ns = [updated_ns for _, updated_ns in sorted_profiles]
    minutes = sorted(buckets.keys())


# snippet:slice_scan:start
def find_slice_scan(since_ns):
    ids = []
    for profile_id, updated_ns in profiles:
        if updated_ns > since_ns:
            ids.append(profile_id)
    return ids
# snippet:slice_scan:end


# snippet:binary_search:start
def find_binary_search(since_ns):
    index = bisect.bisect_right(sorted_updated_ns, since_ns)
    return [profile_id for profile_id, _ in sorted_profiles[index:]]
# snippet:binary_search:end


# snippet:bucketed_index:start
def find_bucketed_index(since_ns):
    minute = (since_ns // 1_000_000_000) // 60
    index = bisect.bisect_left(minutes, minute)

    ids = []
    for bucket_minute in minutes[index:]:
        for profile_id in buckets[bucket_minute]:
            if bucket_minute == minute and profile_map[profile_id] <= since_ns:
                continue
            ids.append(profile_id)
    return ids
# snippet:bucketed_index:end


# snippet:map_scan:start
def find_map_scan(since_ns):
    ids = []
    for profile_id, updated_ns in profile_map.items():
        if updated_ns > since_ns:
            ids.append(profile_id)
    ids.sort()
    return ids
# snippet:map_scan:end


# snippet:parallel_scan:start
def find_parallel_scan(since_ns):
    if not profiles:
        return []

    chunk_size = (len(profiles) + workers - 1) // workers

    def scan_chunk(start):
        end = min(start + chunk_size, len(profiles))
        ids = []
        for profile_id, updated_ns in profiles[start:end]:
            if updated_ns > since_ns:
                ids.append(profile_id)
        return ids

    starts = range(0, len(profiles), chunk_size)
    ids = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as executor:
        for part in executor.map(scan_chunk, starts):
            ids.extend(part)
    return ids
# snippet:parallel_scan:end


finders = {
    "slice_scan": find_slice_scan,
    "binary_search": find_binary_search,
    "bucketed_index": find_bucketed_index,
    "map_scan": find_map_scan,
    "parallel_scan": find_parallel_scan,
}


def write_response(value):
    sys.stdout.write(json.dumps(value, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def handle_find(request):
    finder = finders.get(request.get("algorithm", ""))
    if finder is None:
        return {"id": request.get("id"), "ok": False, "error": "unknown algorithm"}

    since_ns = int(request.get("sinceUnixNano", "0"))
    started_ns = time.perf_counter_ns()
    ids = finder(since_ns)
    elapsed_micros = (time.perf_counter_ns() - started_ns) // 1000

    response = {
        "id": request.get("id"),
        "ok": True,
        "count": len(ids),
        "elapsedMicros": elapsed_micros,
    }
    if request.get("includeIds", True):
        response["ids"] = ids
    return response


for line in sys.stdin:
    try:
        request = json.loads(line)
        request_type = request.get("type")
        if request_type == "shutdown":
            break
        if request_type == "init":
            build_dataset(int(request["profileCount"]), int(request["generatedAtUnixNano"]))
            write_response({"id": request.get("id"), "ok": True})
            continue
        if request_type == "find":
            write_response(handle_find(request))
            continue
        write_response({"id": request.get("id"), "ok": False, "error": "unknown request type"})
    except Exception as exc:
        write_response({"id": None, "ok": False, "error": str(exc)})
