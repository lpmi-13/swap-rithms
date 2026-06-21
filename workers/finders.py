#!/usr/bin/env python3
import bisect
import concurrent.futures
import heapq
import json
import os
import sys
import time
from collections import OrderedDict

DAY_NS = 24 * 60 * 60 * 1_000_000_000
SIGNUP_WINDOW_NS = 180 * DAY_NS
MS_NS = 1_000_000
REGIONS = ["na", "eu", "apac", "latam", "mea"]
TOPICS = ["platform", "analytics", "billing", "support", "security", "growth", "mobile", "search"]
ROLES = ["admin", "operator", "creator", "reviewer", "developer"]
MAX_SCORE = 1000

profiles = []
sort_profiles = []
sorted_profiles = []
sorted_updated_ns = []
profile_map = {}
profile_ids = []
sorted_profile_ids = []
profile_ids_set = set()
buckets = {}
minutes = []
score_buckets = [[] for _ in range(MAX_SCORE + 1)]
sort_score_buckets = [[] for _ in range(MAX_SCORE + 1)]
search_docs = None
docs_by_id = None
prefix_index = None
inverted_index = None
workers = min(max(os.cpu_count() or 2, 2), 16)

fifo_cache = {}
fifo_order = []
lru_cache = OrderedDict()
lfu_cache = {}
lfu_counts = {}
random_cache = {}
ttl_cache = {}
CACHE_CAPACITY = 256
CACHE_TTL_SECONDS = 60


def build_dataset(count, generated_at_ns):
    global profiles, sort_profiles, sorted_profiles, sorted_updated_ns, profile_map, buckets, minutes
    global score_buckets, sort_score_buckets, search_docs, docs_by_id, prefix_index, inverted_index
    global profile_ids, sorted_profile_ids, profile_ids_set

    step_ns = DAY_NS // max(count, 1)
    signup_step_ns = SIGNUP_WINDOW_NS // max(count, 1)
    profiles = []
    profile_map = {}
    profile_ids = []
    profile_ids_set = set()
    buckets = {}
    score_buckets = [[] for _ in range(MAX_SCORE + 1)]

    for i in range(count):
        profile_id = i + 1
        updated_ns = generated_at_ns - DAY_NS + (i * step_ns) + (((i * 37) % 997) * MS_NS)
        signup_ns = generated_at_ns - SIGNUP_WINDOW_NS + (i * signup_step_ns) + (((i * 53) % 997) * MS_NS)
        score = ((i + 1) * 7919) % 1001
        region = REGIONS[i % len(REGIONS)]
        bio = f"{ROLES[i % len(ROLES)]} {region} user focused on {TOPICS[(i * 7) % len(TOPICS)]} workflows in {region}"
        profile = {
            "id": profile_id,
            "updated_ns": updated_ns,
            "signup_ns": signup_ns,
            "score": score,
            "region": region,
            "name": f"User {profile_id:06d}",
            "email": f"user{profile_id:06d}@example.test",
            "bio": bio,
        }
        profiles.append(profile)
        profile_map[profile_id] = profile
        profile_ids.append(profile_id)
        profile_ids_set.add(profile_id)
        minute = (updated_ns // 1_000_000_000) // 60
        buckets.setdefault(minute, []).append(profile_id)
        score_buckets[score].append(profile_id)

    sorted_profile_ids = sorted(profile_ids)
    sorted_profiles = sorted(profiles, key=lambda profile: (profile["updated_ns"], profile["id"]))
    sorted_updated_ns = [profile["updated_ns"] for profile in sorted_profiles]
    minutes = sorted(buckets.keys())

    sort_profiles = profiles[:5000]
    sort_score_buckets = [[] for _ in range(MAX_SCORE + 1)]
    for profile in sort_profiles:
        sort_score_buckets[profile["score"]].append(profile["id"])

    search_docs = None
    docs_by_id = None
    prefix_index = None
    inverted_index = None


def get_search_docs():
    global search_docs
    if search_docs is None:
        search_docs = build_search_docs(profiles)
    return search_docs


def get_docs_by_id():
    global docs_by_id
    if docs_by_id is None:
        docs_by_id = {doc["id"]: doc["text"] for doc in get_search_docs()}
    return docs_by_id


def get_prefix_index():
    global prefix_index
    if prefix_index is not None:
        return prefix_index

    index = {}
    for doc in get_search_docs():
        for token in doc["tokens"]:
            for i in range(1, len(token) + 1):
                prefix = token[:i]
                ids = index.setdefault(prefix, [])
                if not ids or ids[-1] != doc["id"]:
                    ids.append(doc["id"])
    prefix_index = index
    return prefix_index


def get_inverted_index():
    global inverted_index
    if inverted_index is not None:
        return inverted_index

    index = {}
    for doc in get_search_docs():
        seen = set()
        for token in doc["tokens"]:
            if token not in seen:
                index.setdefault(token, []).append(doc["id"])
                seen.add(token)
    inverted_index = index
    return inverted_index


# snippet:slice_scan:start
def find_slice_scan(request):
    since_ns = int(request.get("sinceUnixNano", "0"))
    ids = []
    for profile in profiles:
        if profile["updated_ns"] > since_ns:
            ids.append(profile["id"])
    return ids
# snippet:slice_scan:end


# snippet:binary_search:start
def find_binary_search(request):
    since_ns = int(request.get("sinceUnixNano", "0"))
    index = bisect.bisect_right(sorted_updated_ns, since_ns)
    return [profile["id"] for profile in sorted_profiles[index:]]
# snippet:binary_search:end


# snippet:bucketed_index:start
def find_bucketed_index(request):
    since_ns = int(request.get("sinceUnixNano", "0"))
    minute = (since_ns // 1_000_000_000) // 60
    index = bisect.bisect_left(minutes, minute)

    ids = []
    for bucket_minute in minutes[index:]:
        for profile_id in buckets[bucket_minute]:
            if bucket_minute == minute and profile_map[profile_id]["updated_ns"] <= since_ns:
                continue
            ids.append(profile_id)
    return ids
# snippet:bucketed_index:end


# snippet:map_scan:start
def find_map_scan(request):
    since_ns = int(request.get("sinceUnixNano", "0"))
    ids = []
    for profile_id, profile in profile_map.items():
        if profile["updated_ns"] > since_ns:
            ids.append(profile_id)
    ids.sort()
    return ids
# snippet:map_scan:end


# snippet:parallel_scan:start
def find_parallel_scan(request):
    since_ns = int(request.get("sinceUnixNano", "0"))
    if not profiles:
        return []

    chunk_size = (len(profiles) + workers - 1) // workers

    def scan_chunk(start):
        end = min(start + chunk_size, len(profiles))
        ids = []
        for profile in profiles[start:end]:
            if profile["updated_ns"] > since_ns:
                ids.append(profile["id"])
        return ids

    starts = range(0, len(profiles), chunk_size)
    ids = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as executor:
        for part in executor.map(scan_chunk, starts):
            ids.extend(part)
    return ids
# snippet:parallel_scan:end


def membership_candidates(request):
    limit = int(request.get("limit", 100))
    if limit <= 0:
        limit = 100
    span = max(1, len(profile_ids) * 2)
    return [1 + ((i * 7919) % span) for i in range(limit)]


# snippet:scan_contains_slice:start
def run_scan_contains_slice(request):
    ids = []
    for candidate in membership_candidates(request):
        for profile_id in profile_ids:
            if profile_id == candidate:
                ids.append(candidate)
                break
    return ids
# snippet:scan_contains_slice:end


# snippet:scan_contains_sorted_slice:start
def run_scan_contains_sorted_slice(request):
    ids = []
    for candidate in membership_candidates(request):
        for profile_id in sorted_profile_ids:
            if profile_id == candidate:
                ids.append(candidate)
                break
    return ids
# snippet:scan_contains_sorted_slice:end


# snippet:binary_search_contains_sorted_slice:start
def run_binary_search_contains_sorted_slice(request):
    ids = []
    for candidate in membership_candidates(request):
        index = bisect.bisect_left(sorted_profile_ids, candidate)
        if index < len(sorted_profile_ids) and sorted_profile_ids[index] == candidate:
            ids.append(candidate)
    return ids
# snippet:binary_search_contains_sorted_slice:end


# snippet:direct_lookup_hash_set:start
def run_direct_lookup_hash_set(request):
    ids = []
    for candidate in membership_candidates(request):
        if candidate in profile_ids_set:
            ids.append(candidate)
    return ids
# snippet:direct_lookup_hash_set:end


# snippet:top_k_full_sort:start
def run_top_k_full_sort(request):
    limit = int(request.get("limit", 100))
    ranked = sorted(profiles, key=lambda profile: (-profile["score"], profile["id"]))
    return [profile["id"] for profile in ranked[:limit]]
# snippet:top_k_full_sort:end


# snippet:top_k_min_heap:start
def run_top_k_min_heap(request):
    limit = int(request.get("limit", 100))
    heap = []
    for profile in profiles:
        item = (profile["score"], -profile["id"], profile["id"])
        if len(heap) < limit:
            heapq.heappush(heap, item)
        elif limit > 0 and item > heap[0]:
            heapq.heapreplace(heap, item)
    ranked = sorted(heap, key=lambda item: (-item[0], item[2]))
    return [item[2] for item in ranked]
# snippet:top_k_min_heap:end


# snippet:top_k_quickselect:start
def run_top_k_quickselect(request):
    limit = int(request.get("limit", 100))
    items = [(profile["score"], profile["id"]) for profile in profiles]
    quickselect_scores(items, limit)
    ranked = sorted(items[:limit], key=lambda item: (-item[0], item[1]))
    return [profile_id for _, profile_id in ranked]
# snippet:top_k_quickselect:end


# snippet:top_k_bucketed:start
def run_top_k_bucketed(request):
    limit = int(request.get("limit", 100))
    ids = []
    for score in range(MAX_SCORE, -1, -1):
        for profile_id in score_buckets[score]:
            ids.append(profile_id)
            if len(ids) == limit:
                return ids
    return ids
# snippet:top_k_bucketed:end


# snippet:top_k_streaming:start
def run_top_k_streaming(request):
    return run_top_k_min_heap(request)
# snippet:top_k_streaming:end


# snippet:sort_insertion:start
def run_sort_insertion(request):
    limit = int(request.get("limit", 100))
    items = [(profile["score"], profile["id"]) for profile in sort_profiles]
    for i in range(1, len(items)):
        current = items[i]
        j = i - 1
        while j >= 0 and better_score(current, items[j]):
            items[j + 1] = items[j]
            j -= 1
        items[j + 1] = current
    return [profile_id for _, profile_id in items[:limit]]
# snippet:sort_insertion:end


# snippet:sort_merge:start
def run_sort_merge(request):
    limit = int(request.get("limit", 100))
    items = merge_sort_scores([(profile["score"], profile["id"]) for profile in sort_profiles])
    return [profile_id for _, profile_id in items[:limit]]
# snippet:sort_merge:end


# snippet:sort_quick:start
def run_sort_quick(request):
    limit = int(request.get("limit", 100))
    items = [(profile["score"], profile["id"]) for profile in sort_profiles]
    quick_sort_scores(items, 0, len(items) - 1)
    return [profile_id for _, profile_id in items[:limit]]
# snippet:sort_quick:end


# snippet:sort_heap:start
def run_sort_heap(request):
    limit = int(request.get("limit", 100))
    heap = [(-profile["score"], profile["id"]) for profile in sort_profiles]
    heapq.heapify(heap)
    ids = []
    while heap and len(ids) < limit:
        _, profile_id = heapq.heappop(heap)
        ids.append(profile_id)
    return ids
# snippet:sort_heap:end


# snippet:sort_counting:start
def run_sort_counting(request):
    limit = int(request.get("limit", 100))
    ids = []
    for score in range(MAX_SCORE, -1, -1):
        for profile_id in sort_score_buckets[score]:
            ids.append(profile_id)
            if len(ids) == limit:
                return ids
    return ids
# snippet:sort_counting:end


# snippet:sort_radix:start
def run_sort_radix(request):
    limit = int(request.get("limit", 100))
    max_id = sort_profiles[-1]["id"] if sort_profiles else 0
    base = max_id + 1
    items = [(profile["score"], profile["id"]) for profile in sort_profiles]
    keys = [score * base + (max_id - profile_id) for score, profile_id in items]
    items = radix_sort_by_key(items, keys)
    items.reverse()
    return [profile_id for _, profile_id in items[:limit]]
# snippet:sort_radix:end


# snippet:sort_builtin:start
def run_sort_builtin(request):
    limit = int(request.get("limit", 100))
    ranked = sorted(sort_profiles, key=lambda profile: (-profile["score"], profile["id"]))
    return [profile["id"] for profile in ranked[:limit]]
# snippet:sort_builtin:end


# snippet:cache_none:start
def run_cache_none(request):
    profile_id = int(request.get("profileId", 1))
    if profile_id not in profile_map:
        return []
    return [profile_id]
# snippet:cache_none:end


# snippet:cache_fifo:start
def run_cache_fifo(request):
    profile_id = int(request.get("profileId", 1))
    if profile_id in fifo_cache:
        return [profile_id]
    profile = profile_map.get(profile_id)
    if profile is None:
        return []
    if len(fifo_cache) >= CACHE_CAPACITY and fifo_order:
        fifo_cache.pop(fifo_order.pop(0), None)
    fifo_cache[profile_id] = profile
    fifo_order.append(profile_id)
    return [profile_id]
# snippet:cache_fifo:end


# snippet:cache_lru:start
def run_cache_lru(request):
    profile_id = int(request.get("profileId", 1))
    if profile_id in lru_cache:
        lru_cache.move_to_end(profile_id, last=False)
        return [profile_id]
    profile = profile_map.get(profile_id)
    if profile is None:
        return []
    if len(lru_cache) >= CACHE_CAPACITY:
        lru_cache.popitem(last=True)
    lru_cache[profile_id] = profile
    lru_cache.move_to_end(profile_id, last=False)
    return [profile_id]
# snippet:cache_lru:end


# snippet:cache_lfu:start
def run_cache_lfu(request):
    profile_id = int(request.get("profileId", 1))
    if profile_id in lfu_cache:
        lfu_counts[profile_id] += 1
        return [profile_id]
    profile = profile_map.get(profile_id)
    if profile is None:
        return []
    if len(lfu_cache) >= CACHE_CAPACITY:
        victim = min(lfu_cache.keys(), key=lambda cached_id: (lfu_counts[cached_id], -cached_id))
        lfu_cache.pop(victim, None)
        lfu_counts.pop(victim, None)
    lfu_cache[profile_id] = profile
    lfu_counts[profile_id] = 1
    return [profile_id]
# snippet:cache_lfu:end


# snippet:cache_random:start
def run_cache_random(request):
    profile_id = int(request.get("profileId", 1))
    if profile_id in random_cache:
        return [profile_id]
    profile = profile_map.get(profile_id)
    if profile is None:
        return []
    if len(random_cache) >= CACHE_CAPACITY:
        keys = sorted(random_cache.keys())
        random_cache.pop(keys[(profile_id * 1103515245 + len(keys)) % len(keys)], None)
    random_cache[profile_id] = profile
    return [profile_id]
# snippet:cache_random:end


# snippet:cache_ttl:start
def run_cache_ttl(request):
    profile_id = int(request.get("profileId", 1))
    now = time.monotonic()
    entry = ttl_cache.get(profile_id)
    if entry and now < entry[1]:
        return [profile_id]
    profile = profile_map.get(profile_id)
    if profile is None:
        return []
    if len(ttl_cache) >= CACHE_CAPACITY:
        ttl_cache.pop(next(iter(ttl_cache)), None)
    ttl_cache[profile_id] = (profile, now + CACHE_TTL_SECONDS)
    return [profile_id]
# snippet:cache_ttl:end


# snippet:text_naive:start
def run_text_naive(request):
    query = request.get("query", "platform").lower().strip()
    limit = int(request.get("limit", 100))
    ids = []
    for profile in profiles:
        if query in profile_search_text(profile):
            ids.append(profile["id"])
            if len(ids) == limit:
                break
    return ids
# snippet:text_naive:end


# snippet:text_lowercase:start
def run_text_lowercase(request):
    query = request.get("query", "platform").lower().strip()
    limit = int(request.get("limit", 100))
    return search_with(get_search_docs(), query, limit, lambda text, pattern: pattern in text)
# snippet:text_lowercase:end


# snippet:text_kmp:start
def run_text_kmp(request):
    query = request.get("query", "platform").lower().strip()
    limit = int(request.get("limit", 100))
    return search_with(get_search_docs(), query, limit, contains_kmp)
# snippet:text_kmp:end


# snippet:text_boyer_moore:start
def run_text_boyer_moore(request):
    query = request.get("query", "platform").lower().strip()
    limit = int(request.get("limit", 100))
    return search_with(get_search_docs(), query, limit, contains_boyer_moore)
# snippet:text_boyer_moore:end


# snippet:text_trie_prefix:start
def run_text_trie_prefix(request):
    query = request.get("query", "platform").lower().strip()
    limit = int(request.get("limit", 100))
    candidates = get_prefix_index().get(query, [])
    if not candidates or " " in query:
        return search_with(get_search_docs(), query, limit, lambda text, pattern: pattern in text)
    ids = []
    texts_by_id = get_docs_by_id()
    for profile_id in candidates:
        if query in texts_by_id[profile_id]:
            ids.append(profile_id)
            if len(ids) == limit:
                break
    return ids
# snippet:text_trie_prefix:end


# snippet:text_inverted_index:start
def run_text_inverted_index(request):
    query = request.get("query", "platform").lower().strip()
    limit = int(request.get("limit", 100))
    tokens = query.split()
    if not tokens:
        return []
    candidates = get_inverted_index().get(tokens[0], [])
    if not candidates:
        return search_with(get_search_docs(), query, limit, lambda text, pattern: pattern in text)
    texts_by_id = get_docs_by_id()
    ids = []
    for profile_id in candidates:
        if query in texts_by_id[profile_id]:
            ids.append(profile_id)
            if len(ids) == limit:
                break
    return ids
# snippet:text_inverted_index:end


def better_score(a, b):
    if a[0] == b[0]:
        return a[1] < b[1]
    return a[0] > b[0]


def partition_scores(items, left, right, pivot):
    pivot_value = items[pivot]
    items[pivot], items[right] = items[right], items[pivot]
    store = left
    for i in range(left, right):
        if better_score(items[i], pivot_value):
            items[store], items[i] = items[i], items[store]
            store += 1
    items[right], items[store] = items[store], items[right]
    return store


def quickselect_scores(items, limit):
    if limit <= 0 or limit >= len(items):
        return
    left, right = 0, len(items) - 1
    target = limit - 1
    while left < right:
        pivot = partition_scores(items, left, right, (left + right) // 2)
        if pivot == target:
            return
        if pivot > target:
            right = pivot - 1
        else:
            left = pivot + 1


def quick_sort_scores(items, left, right):
    if left >= right:
        return
    pivot = partition_scores(items, left, right, (left + right) // 2)
    quick_sort_scores(items, left, pivot - 1)
    quick_sort_scores(items, pivot + 1, right)


def merge_sort_scores(items):
    if len(items) <= 1:
        return items
    mid = len(items) // 2
    left = merge_sort_scores(items[:mid])
    right = merge_sort_scores(items[mid:])
    merged = []
    i = j = 0
    while i < len(left) and j < len(right):
        if better_score(left[i], right[j]):
            merged.append(left[i])
            i += 1
        else:
            merged.append(right[j])
            j += 1
    merged.extend(left[i:])
    merged.extend(right[j:])
    return merged


def radix_sort_by_key(items, keys):
    if len(items) <= 1:
        return items
    max_key = max(keys) if keys else 0
    exp = 1
    while max_key // exp > 0:
        buckets_for_digit = [[] for _ in range(10)]
        for item, key in zip(items, keys):
            buckets_for_digit[(key // exp) % 10].append((item, key))
        pairs = [pair for bucket in buckets_for_digit for pair in bucket]
        items = [pair[0] for pair in pairs]
        keys = [pair[1] for pair in pairs]
        exp *= 10
    return items


def build_search_docs(source_profiles):
    docs = []
    for profile in source_profiles:
        text = profile_search_text(profile)
        docs.append({"id": profile["id"], "text": text, "tokens": text.split()})
    return docs


def profile_search_text(profile):
    return f'{profile["name"]} {profile["email"]} {profile["region"]} {profile["bio"]}'.lower()


def search_with(docs, query, limit, contains):
    if not query:
        return []
    ids = []
    for doc in docs:
        if contains(doc["text"], query):
            ids.append(doc["id"])
            if len(ids) == limit:
                break
    return ids


def contains_kmp(text, pattern):
    if pattern == "":
        return True
    if len(pattern) > len(text):
        return False
    lps = [0] * len(pattern)
    length = 0
    i = 1
    while i < len(pattern):
        if pattern[i] == pattern[length]:
            length += 1
            lps[i] = length
            i += 1
        elif length:
            length = lps[length - 1]
        else:
            i += 1
    i = j = 0
    while i < len(text):
        if text[i] == pattern[j]:
            i += 1
            j += 1
            if j == len(pattern):
                return True
        elif j:
            j = lps[j - 1]
        else:
            i += 1
    return False


def contains_boyer_moore(text, pattern):
    if pattern == "":
        return True
    if len(pattern) > len(text):
        return False
    last = {char: i for i, char in enumerate(pattern)}
    shift = 0
    while shift <= len(text) - len(pattern):
        j = len(pattern) - 1
        while j >= 0 and pattern[j] == text[shift + j]:
            j -= 1
        if j < 0:
            return True
        shift += max(1, j - last.get(text[shift + j], -1))
    return False


def default_data_structures(runners):
    return {name: {"default": runner} for name, runner in runners.items()}


algorithms = {
    "lookup": default_data_structures({
        "slice_scan": find_slice_scan,
        "binary_search": find_binary_search,
        "bucketed_index": find_bucketed_index,
        "map_scan": find_map_scan,
        "parallel_scan": find_parallel_scan,
    }),
    "membership": {
        "scan_contains": {
            "slice": run_scan_contains_slice,
            "sorted_slice": run_scan_contains_sorted_slice,
        },
        "binary_search_contains": {
            "sorted_slice": run_binary_search_contains_sorted_slice,
        },
        "direct_lookup": {
            "hash_set": run_direct_lookup_hash_set,
        },
    },
    "top_k": default_data_structures({
        "top_k_full_sort": run_top_k_full_sort,
        "top_k_min_heap": run_top_k_min_heap,
        "top_k_quickselect": run_top_k_quickselect,
        "top_k_bucketed": run_top_k_bucketed,
        "top_k_streaming": run_top_k_streaming,
    }),
    "sorting": default_data_structures({
        "sort_insertion": run_sort_insertion,
        "sort_merge": run_sort_merge,
        "sort_quick": run_sort_quick,
        "sort_heap": run_sort_heap,
        "sort_counting": run_sort_counting,
        "sort_radix": run_sort_radix,
        "sort_builtin": run_sort_builtin,
    }),
    "caching": default_data_structures({
        "cache_none": run_cache_none,
        "cache_fifo": run_cache_fifo,
        "cache_lru": run_cache_lru,
        "cache_lfu": run_cache_lfu,
        "cache_random": run_cache_random,
        "cache_ttl": run_cache_ttl,
    }),
    "text_search": default_data_structures({
        "text_naive": run_text_naive,
        "text_lowercase": run_text_lowercase,
        "text_kmp": run_text_kmp,
        "text_boyer_moore": run_text_boyer_moore,
        "text_trie_prefix": run_text_trie_prefix,
        "text_inverted_index": run_text_inverted_index,
    }),
}


def write_response(value):
    sys.stdout.write(json.dumps(value, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def handle_run(request):
    scenario = request.get("scenario") or "lookup"
    algorithm = request.get("algorithm", "")
    data_structure = request.get("dataStructure") or "default"
    runner = algorithms.get(scenario, {}).get(algorithm, {}).get(data_structure)
    if runner is None:
        return {"id": request.get("id"), "ok": False, "error": "unknown implementation"}

    started_ns = time.perf_counter_ns()
    ids = runner(request)
    elapsed_micros = (time.perf_counter_ns() - started_ns) // 1000

    response = {
        "id": request.get("id"),
        "ok": True,
        "count": len(ids),
        "elapsedMicros": elapsed_micros,
    }
    if request.get("includeIds", True):
        id_limit = int(request.get("idLimit") or 0)
        response["ids"] = ids[:id_limit] if id_limit > 0 else ids
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
        if request_type in ("run", "find"):
            write_response(handle_run(request))
            continue
        write_response({"id": request.get("id"), "ok": False, "error": "unknown request type"})
    except Exception as exc:
        write_response({"id": None, "ok": False, "error": str(exc)})
