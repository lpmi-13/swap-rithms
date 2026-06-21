import { cpus } from "node:os";
import { stdin, stdout } from "node:process";
import { createInterface } from "node:readline";
import { performance } from "node:perf_hooks";

const DAY_NS = 24n * 60n * 60n * 1_000_000_000n;
const SIGNUP_WINDOW_NS = 180n * DAY_NS;
const MS_NS = 1_000_000n;
const REGIONS = ["na", "eu", "apac", "latam", "mea"];
const TOPICS = ["platform", "analytics", "billing", "support", "security", "growth", "mobile", "search"];
const ROLES = ["admin", "operator", "creator", "reviewer", "developer"];
const MAX_SCORE = 1000;
const CACHE_CAPACITY = 256;
const CACHE_TTL_MS = 60_000;

interface Profile {
  id: number;
  updatedNs: bigint;
  signupNs: bigint;
  score: number;
  region: string;
  name: string;
  email: string;
  bio: string;
}

interface ScoreItem {
  id: number;
  score: number;
}

interface SearchDoc {
  id: number;
  text: string;
  tokens: string[];
}

interface WorkerRequest {
  id?: number;
  type?: string;
  profileCount?: number;
  generatedAtUnixNano?: string;
  scenario?: string;
  algorithm?: string;
  dataStructure?: string;
  sinceUnixNano?: string;
  includeIds?: boolean;
  limit?: number;
  query?: string;
  profileId?: number;
}

let profiles: Profile[] = [];
let sortProfiles: Profile[] = [];
let sortedProfiles: Profile[] = [];
let sortedUpdatedNs: bigint[] = [];
let profileMap = new Map<number, Profile>();
let profileIDs: number[] = [];
let sortedProfileIDs: number[] = [];
let profileIDSet = new Set<number>();
let buckets = new Map<bigint, number[]>();
let minutes: bigint[] = [];
let scoreBuckets: number[][] = Array.from({ length: MAX_SCORE + 1 }, () => []);
let sortScoreBuckets: number[][] = Array.from({ length: MAX_SCORE + 1 }, () => []);
let searchDocs: SearchDoc[] | undefined;
let docsByID: Map<number, string> | undefined;
let prefixIndex: Map<string, number[]> | undefined;
let invertedIndex: Map<string, number[]> | undefined;
let workers = Math.min(Math.max(cpus().length || 2, 2), 16);

const fifoCache = new Map<number, Profile>();
const fifoOrder: number[] = [];
const lruCache = new Map<number, Profile>();
const lfuCache = new Map<number, Profile>();
const lfuCounts = new Map<number, number>();
const randomCache = new Map<number, Profile>();
const ttlCache = new Map<number, { profile: Profile; expires: number }>();

function buildDataset(count: number, generatedAtNs: bigint): void {
  const stepNs = DAY_NS / BigInt(Math.max(count, 1));
  const signupStepNs = SIGNUP_WINDOW_NS / BigInt(Math.max(count, 1));

  profiles = [];
  profileMap = new Map<number, Profile>();
  profileIDs = [];
  profileIDSet = new Set<number>();
  buckets = new Map<bigint, number[]>();
  scoreBuckets = Array.from({ length: MAX_SCORE + 1 }, () => []);

  for (let i = 0; i < count; i++) {
    const id = i + 1;
    const updatedNs = generatedAtNs - DAY_NS + BigInt(i) * stepNs + BigInt((i * 37) % 997) * MS_NS;
    const signupNs = generatedAtNs - SIGNUP_WINDOW_NS + BigInt(i) * signupStepNs + BigInt((i * 53) % 997) * MS_NS;
    const score = ((i + 1) * 7919) % 1001;
    const region = REGIONS[i % REGIONS.length];
    const bio = `${ROLES[i % ROLES.length]} ${region} user focused on ${TOPICS[(i * 7) % TOPICS.length]} workflows in ${region}`;
    const profile = {
      id,
      updatedNs,
      signupNs,
      score,
      region,
      name: `User ${String(id).padStart(6, "0")}`,
      email: `user${String(id).padStart(6, "0")}@example.test`,
      bio,
    };
    profiles.push(profile);
    profileMap.set(id, profile);
    profileIDs.push(id);
    profileIDSet.add(id);
    const minute = updatedNs / 1_000_000_000n / 60n;
    const bucket = buckets.get(minute);
    if (bucket) bucket.push(id);
    else buckets.set(minute, [id]);
    scoreBuckets[score].push(id);
  }

  sortedProfileIDs = [...profileIDs].sort((a, b) => a - b);
  sortedProfiles = [...profiles].sort((a, b) => {
    if (a.updatedNs === b.updatedNs) return a.id - b.id;
    return a.updatedNs < b.updatedNs ? -1 : 1;
  });
  sortedUpdatedNs = sortedProfiles.map((profile) => profile.updatedNs);
  minutes = [...buckets.keys()].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));

  sortProfiles = profiles.slice(0, 5000);
  sortScoreBuckets = Array.from({ length: MAX_SCORE + 1 }, () => []);
  for (const profile of sortProfiles) sortScoreBuckets[profile.score].push(profile.id);

  searchDocs = undefined;
  docsByID = undefined;
  prefixIndex = undefined;
  invertedIndex = undefined;
}

function getSearchDocs(): SearchDoc[] {
  if (!searchDocs) searchDocs = buildSearchDocs(profiles);
  return searchDocs;
}

function getDocsByID(): Map<number, string> {
  if (!docsByID) docsByID = new Map(getSearchDocs().map((doc) => [doc.id, doc.text]));
  return docsByID;
}

function getPrefixIndex(): Map<string, number[]> {
  if (prefixIndex) return prefixIndex;

  const index = new Map<string, number[]>();
  for (const doc of getSearchDocs()) {
    for (const token of doc.tokens) {
      for (let i = 1; i <= token.length; i++) pushIndex(index, token.slice(0, i), doc.id);
    }
  }
  prefixIndex = index;
  return prefixIndex;
}

function getInvertedIndex(): Map<string, number[]> {
  if (invertedIndex) return invertedIndex;

  const index = new Map<string, number[]>();
  for (const doc of getSearchDocs()) {
    const seen = new Set<string>();
    for (const token of doc.tokens) {
      if (!seen.has(token)) {
        pushIndex(index, token, doc.id);
        seen.add(token);
      }
    }
  }
  invertedIndex = index;
  return invertedIndex;
}

// snippet:slice_scan:start
function findSliceScan(request: WorkerRequest): number[] {
  const sinceNs = BigInt(request.sinceUnixNano || "0");
  const ids: number[] = [];
  for (const profile of profiles) {
    if (profile.updatedNs > sinceNs) ids.push(profile.id);
  }
  return ids;
}
// snippet:slice_scan:end

// snippet:binary_search:start
function firstGreaterThan(values: bigint[], target: bigint): number {
  let low = 0;
  let high = values.length;
  while (low < high) {
    const mid = Math.floor((low + high) / 2);
    if (values[mid] > target) high = mid;
    else low = mid + 1;
  }
  return low;
}

function firstNumberAtLeast(values: number[], target: number): number {
  let low = 0;
  let high = values.length;
  while (low < high) {
    const mid = Math.floor((low + high) / 2);
    if (values[mid] >= target) high = mid;
    else low = mid + 1;
  }
  return low;
}

function findBinarySearch(request: WorkerRequest): number[] {
  const sinceNs = BigInt(request.sinceUnixNano || "0");
  const index = firstGreaterThan(sortedUpdatedNs, sinceNs);
  return sortedProfiles.slice(index).map((profile) => profile.id);
}
// snippet:binary_search:end

// snippet:bucketed_index:start
function firstMinuteAtLeast(values: bigint[], target: bigint): number {
  let low = 0;
  let high = values.length;
  while (low < high) {
    const mid = Math.floor((low + high) / 2);
    if (values[mid] >= target) high = mid;
    else low = mid + 1;
  }
  return low;
}

function findBucketedIndex(request: WorkerRequest): number[] {
  const sinceNs = BigInt(request.sinceUnixNano || "0");
  const minute = sinceNs / 1_000_000_000n / 60n;
  const index = firstMinuteAtLeast(minutes, minute);

  const ids: number[] = [];
  for (const bucketMinute of minutes.slice(index)) {
    for (const id of buckets.get(bucketMinute) || []) {
      if (bucketMinute === minute && (profileMap.get(id)?.updatedNs || 0n) <= sinceNs) continue;
      ids.push(id);
    }
  }
  return ids;
}
// snippet:bucketed_index:end

// snippet:map_scan:start
function findMapScan(request: WorkerRequest): number[] {
  const sinceNs = BigInt(request.sinceUnixNano || "0");
  const ids: number[] = [];
  for (const [id, profile] of profileMap) {
    if (profile.updatedNs > sinceNs) ids.push(id);
  }
  ids.sort((a, b) => a - b);
  return ids;
}
// snippet:map_scan:end

// snippet:parallel_scan:start
function findParallelScan(request: WorkerRequest): number[] {
  const sinceNs = BigInt(request.sinceUnixNano || "0");
  if (profiles.length === 0) return [];

  const chunkSize = Math.ceil(profiles.length / workers);
  const parts: number[][] = [];

  for (let worker = 0; worker < workers; worker++) {
    const start = worker * chunkSize;
    const end = Math.min(start + chunkSize, profiles.length);
    if (start >= end) continue;

    const local: number[] = [];
    for (const profile of profiles.slice(start, end)) {
      if (profile.updatedNs > sinceNs) local.push(profile.id);
    }
    parts.push(local);
  }

  return parts.flat();
}
// snippet:parallel_scan:end

function membershipCandidates(request: WorkerRequest): number[] {
  const limit = request.limit && request.limit > 0 ? request.limit : 100;
  const span = Math.max(1, profileIDs.length * 2);
  return Array.from({ length: limit }, (_, i) => 1 + ((i * 7919) % span));
}

// snippet:scan_contains_slice:start
function runScanContainsSlice(request: WorkerRequest): number[] {
  const ids: number[] = [];
  for (const candidate of membershipCandidates(request)) {
    for (const id of profileIDs) {
      if (id === candidate) {
        ids.push(candidate);
        break;
      }
    }
  }
  return ids;
}
// snippet:scan_contains_slice:end

// snippet:scan_contains_sorted_slice:start
function runScanContainsSortedSlice(request: WorkerRequest): number[] {
  const ids: number[] = [];
  for (const candidate of membershipCandidates(request)) {
    for (const id of sortedProfileIDs) {
      if (id === candidate) {
        ids.push(candidate);
        break;
      }
    }
  }
  return ids;
}
// snippet:scan_contains_sorted_slice:end

// snippet:binary_search_contains_sorted_slice:start
function runBinarySearchContainsSortedSlice(request: WorkerRequest): number[] {
  const ids: number[] = [];
  for (const candidate of membershipCandidates(request)) {
    const index = firstNumberAtLeast(sortedProfileIDs, candidate);
    if (index < sortedProfileIDs.length && sortedProfileIDs[index] === candidate) ids.push(candidate);
  }
  return ids;
}
// snippet:binary_search_contains_sorted_slice:end

// snippet:direct_lookup_hash_set:start
function runDirectLookupHashSet(request: WorkerRequest): number[] {
  const ids: number[] = [];
  for (const candidate of membershipCandidates(request)) {
    if (profileIDSet.has(candidate)) ids.push(candidate);
  }
  return ids;
}
// snippet:direct_lookup_hash_set:end

// snippet:top_k_full_sort:start
function runTopKFullSort(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  return sortedScoreItems(scoreItems(profiles)).slice(0, limit).map((item) => item.id);
}
// snippet:top_k_full_sort:end

// snippet:top_k_min_heap:start
function runTopKMinHeap(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const heap: ScoreItem[] = [];
  for (const profile of profiles) {
    const item = { id: profile.id, score: profile.score };
    if (heap.length < limit) pushMinHeap(heap, item);
    else if (limit > 0 && betterScore(item, heap[0])) replaceMinHeap(heap, item);
  }
  return sortedScoreItems(heap).map((item) => item.id);
}
// snippet:top_k_min_heap:end

// snippet:top_k_quickselect:start
function runTopKQuickselect(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const items = scoreItems(profiles);
  quickselectScores(items, limit);
  return sortedScoreItems(items.slice(0, limit)).map((item) => item.id);
}
// snippet:top_k_quickselect:end

// snippet:top_k_bucketed:start
function runTopKBucketed(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const ids: number[] = [];
  for (let score = MAX_SCORE; score >= 0 && ids.length < limit; score--) {
    for (const id of scoreBuckets[score]) {
      ids.push(id);
      if (ids.length === limit) break;
    }
  }
  return ids;
}
// snippet:top_k_bucketed:end

// snippet:top_k_streaming:start
function runTopKStreaming(request: WorkerRequest): number[] {
  return runTopKMinHeap(request);
}
// snippet:top_k_streaming:end

// snippet:sort_insertion:start
function runSortInsertion(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const items = scoreItems(sortProfiles);
  for (let i = 1; i < items.length; i++) {
    const current = items[i];
    let j = i - 1;
    while (j >= 0 && betterScore(current, items[j])) {
      items[j + 1] = items[j];
      j--;
    }
    items[j + 1] = current;
  }
  return items.slice(0, limit).map((item) => item.id);
}
// snippet:sort_insertion:end

// snippet:sort_merge:start
function runSortMerge(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  return mergeSortScores(scoreItems(sortProfiles)).slice(0, limit).map((item) => item.id);
}
// snippet:sort_merge:end

// snippet:sort_quick:start
function runSortQuick(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const items = scoreItems(sortProfiles);
  quickSortScores(items, 0, items.length - 1);
  return items.slice(0, limit).map((item) => item.id);
}
// snippet:sort_quick:end

// snippet:sort_heap:start
function runSortHeap(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const heap = scoreItems(sortProfiles);
  heapifyMax(heap);
  const ids: number[] = [];
  while (heap.length > 0 && ids.length < limit) ids.push(popMaxHeap(heap).id);
  return ids;
}
// snippet:sort_heap:end

// snippet:sort_counting:start
function runSortCounting(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const ids: number[] = [];
  for (let score = MAX_SCORE; score >= 0 && ids.length < limit; score--) {
    for (const id of sortScoreBuckets[score]) {
      ids.push(id);
      if (ids.length === limit) break;
    }
  }
  return ids;
}
// snippet:sort_counting:end

// snippet:sort_radix:start
function runSortRadix(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  const maxID = sortProfiles.length ? sortProfiles[sortProfiles.length - 1].id : 0;
  const base = maxID + 1;
  const items = scoreItems(sortProfiles);
  const keys = items.map((item) => item.score * base + (maxID - item.id));
  const sorted = radixSortByKey(items, keys).reverse();
  return sorted.slice(0, limit).map((item) => item.id);
}
// snippet:sort_radix:end

// snippet:sort_builtin:start
function runSortBuiltin(request: WorkerRequest): number[] {
  const limit = request.limit || 100;
  return sortedScoreItems(scoreItems(sortProfiles)).slice(0, limit).map((item) => item.id);
}
// snippet:sort_builtin:end

// snippet:cache_none:start
function runCacheNone(request: WorkerRequest): number[] {
  const id = request.profileId || 1;
  return profileMap.has(id) ? [id] : [];
}
// snippet:cache_none:end

// snippet:cache_fifo:start
function runCacheFIFO(request: WorkerRequest): number[] {
  const id = request.profileId || 1;
  if (fifoCache.has(id)) return [id];
  const profile = profileMap.get(id);
  if (!profile) return [];
  if (fifoCache.size >= CACHE_CAPACITY && fifoOrder.length) fifoCache.delete(fifoOrder.shift() || 0);
  fifoCache.set(id, profile);
  fifoOrder.push(id);
  return [id];
}
// snippet:cache_fifo:end

// snippet:cache_lru:start
function runCacheLRU(request: WorkerRequest): number[] {
  const id = request.profileId || 1;
  const cached = lruCache.get(id);
  if (cached) {
    lruCache.delete(id);
    lruCache.set(id, cached);
    return [id];
  }
  const profile = profileMap.get(id);
  if (!profile) return [];
  if (lruCache.size >= CACHE_CAPACITY) lruCache.delete(lruCache.keys().next().value);
  lruCache.set(id, profile);
  return [id];
}
// snippet:cache_lru:end

// snippet:cache_lfu:start
function runCacheLFU(request: WorkerRequest): number[] {
  const id = request.profileId || 1;
  if (lfuCache.has(id)) {
    lfuCounts.set(id, (lfuCounts.get(id) || 0) + 1);
    return [id];
  }
  const profile = profileMap.get(id);
  if (!profile) return [];
  if (lfuCache.size >= CACHE_CAPACITY) {
    let victim = 0;
    for (const cachedID of lfuCache.keys()) {
      if (!victim || (lfuCounts.get(cachedID) || 0) < (lfuCounts.get(victim) || 0) || ((lfuCounts.get(cachedID) || 0) === (lfuCounts.get(victim) || 0) && cachedID > victim)) victim = cachedID;
    }
    lfuCache.delete(victim);
    lfuCounts.delete(victim);
  }
  lfuCache.set(id, profile);
  lfuCounts.set(id, 1);
  return [id];
}
// snippet:cache_lfu:end

// snippet:cache_random:start
function runCacheRandom(request: WorkerRequest): number[] {
  const id = request.profileId || 1;
  if (randomCache.has(id)) return [id];
  const profile = profileMap.get(id);
  if (!profile) return [];
  if (randomCache.size >= CACHE_CAPACITY) {
    const keys = [...randomCache.keys()].sort((a, b) => a - b);
    randomCache.delete(keys[(id * 1103515245 + keys.length) % keys.length]);
  }
  randomCache.set(id, profile);
  return [id];
}
// snippet:cache_random:end

// snippet:cache_ttl:start
function runCacheTTL(request: WorkerRequest): number[] {
  const id = request.profileId || 1;
  const now = Date.now();
  const cached = ttlCache.get(id);
  if (cached && now < cached.expires) return [id];
  const profile = profileMap.get(id);
  if (!profile) return [];
  if (ttlCache.size >= CACHE_CAPACITY) ttlCache.delete(ttlCache.keys().next().value);
  ttlCache.set(id, { profile, expires: now + CACHE_TTL_MS });
  return [id];
}
// snippet:cache_ttl:end

// snippet:text_naive:start
function runTextNaive(request: WorkerRequest): number[] {
  const query = (request.query || "platform").toLowerCase().trim();
  const limit = request.limit || 100;
  const ids: number[] = [];
  for (const profile of profiles) {
    if (profileSearchText(profile).includes(query)) ids.push(profile.id);
    if (ids.length === limit) break;
  }
  return ids;
}
// snippet:text_naive:end

// snippet:text_lowercase:start
function runTextLowercase(request: WorkerRequest): number[] {
  const query = (request.query || "platform").toLowerCase().trim();
  const limit = request.limit || 100;
  return searchWith(getSearchDocs(), query, limit, (text, pattern) => text.includes(pattern));
}
// snippet:text_lowercase:end

// snippet:text_kmp:start
function runTextKMP(request: WorkerRequest): number[] {
  const query = (request.query || "platform").toLowerCase().trim();
  const limit = request.limit || 100;
  return searchWith(getSearchDocs(), query, limit, containsKMP);
}
// snippet:text_kmp:end

// snippet:text_boyer_moore:start
function runTextBoyerMoore(request: WorkerRequest): number[] {
  const query = (request.query || "platform").toLowerCase().trim();
  const limit = request.limit || 100;
  return searchWith(getSearchDocs(), query, limit, containsBoyerMoore);
}
// snippet:text_boyer_moore:end

// snippet:text_trie_prefix:start
function runTextTriePrefix(request: WorkerRequest): number[] {
  const query = (request.query || "platform").toLowerCase().trim();
  const limit = request.limit || 100;
  const candidates = getPrefixIndex().get(query) || [];
  if (!candidates.length || query.includes(" ")) {
    return searchWith(getSearchDocs(), query, limit, (text, pattern) => text.includes(pattern));
  }
  const textsByID = getDocsByID();
  const ids: number[] = [];
  for (const id of candidates) {
    if ((textsByID.get(id) || "").includes(query)) ids.push(id);
    if (ids.length === limit) break;
  }
  return ids;
}
// snippet:text_trie_prefix:end

// snippet:text_inverted_index:start
function runTextInvertedIndex(request: WorkerRequest): number[] {
  const query = (request.query || "platform").toLowerCase().trim();
  const limit = request.limit || 100;
  const tokens = query.split(/\s+/).filter(Boolean);
  if (!tokens.length) return [];
  const candidates = getInvertedIndex().get(tokens[0]) || [];
  if (!candidates.length) return searchWith(getSearchDocs(), query, limit, (text, pattern) => text.includes(pattern));
  const textsByID = getDocsByID();
  const ids: number[] = [];
  for (const id of candidates) {
    if ((textsByID.get(id) || "").includes(query)) ids.push(id);
    if (ids.length === limit) break;
  }
  return ids;
}
// snippet:text_inverted_index:end

function scoreItems(source: Profile[]): ScoreItem[] {
  return source.map((profile) => ({ id: profile.id, score: profile.score }));
}

function sortedScoreItems(items: ScoreItem[]): ScoreItem[] {
  return [...items].sort((a, b) => (a.score === b.score ? a.id - b.id : b.score - a.score));
}

function betterScore(a: ScoreItem, b: ScoreItem): boolean {
  if (a.score === b.score) return a.id < b.id;
  return a.score > b.score;
}

function worseScore(a: ScoreItem, b: ScoreItem): boolean {
  if (a.score === b.score) return a.id > b.id;
  return a.score < b.score;
}

function pushMinHeap(heap: ScoreItem[], item: ScoreItem): void {
  heap.push(item);
  siftMinUp(heap, heap.length - 1);
}

function replaceMinHeap(heap: ScoreItem[], item: ScoreItem): void {
  heap[0] = item;
  siftMinDown(heap, 0);
}

function siftMinUp(heap: ScoreItem[], index: number): void {
  while (index > 0) {
    const parent = Math.floor((index - 1) / 2);
    if (!worseScore(heap[index], heap[parent])) break;
    [heap[index], heap[parent]] = [heap[parent], heap[index]];
    index = parent;
  }
}

function siftMinDown(heap: ScoreItem[], index: number): void {
  for (;;) {
    let smallest = index;
    const left = index * 2 + 1;
    const right = left + 1;
    if (left < heap.length && worseScore(heap[left], heap[smallest])) smallest = left;
    if (right < heap.length && worseScore(heap[right], heap[smallest])) smallest = right;
    if (smallest === index) return;
    [heap[index], heap[smallest]] = [heap[smallest], heap[index]];
    index = smallest;
  }
}

function heapifyMax(heap: ScoreItem[]): void {
  for (let i = Math.floor(heap.length / 2) - 1; i >= 0; i--) siftMaxDown(heap, i);
}

function popMaxHeap(heap: ScoreItem[]): ScoreItem {
  const item = heap[0];
  const last = heap.pop();
  if (heap.length && last) {
    heap[0] = last;
    siftMaxDown(heap, 0);
  }
  return item;
}

function siftMaxDown(heap: ScoreItem[], index: number): void {
  for (;;) {
    let best = index;
    const left = index * 2 + 1;
    const right = left + 1;
    if (left < heap.length && betterScore(heap[left], heap[best])) best = left;
    if (right < heap.length && betterScore(heap[right], heap[best])) best = right;
    if (best === index) return;
    [heap[index], heap[best]] = [heap[best], heap[index]];
    index = best;
  }
}

function partitionScores(items: ScoreItem[], left: number, right: number, pivot: number): number {
  const pivotValue = items[pivot];
  [items[pivot], items[right]] = [items[right], items[pivot]];
  let store = left;
  for (let i = left; i < right; i++) {
    if (betterScore(items[i], pivotValue)) {
      [items[store], items[i]] = [items[i], items[store]];
      store++;
    }
  }
  [items[right], items[store]] = [items[store], items[right]];
  return store;
}

function quickselectScores(items: ScoreItem[], limit: number): void {
  if (limit <= 0 || limit >= items.length) return;
  let left = 0;
  let right = items.length - 1;
  const target = limit - 1;
  while (left < right) {
    const pivot = partitionScores(items, left, right, Math.floor((left + right) / 2));
    if (pivot === target) return;
    if (pivot > target) right = pivot - 1;
    else left = pivot + 1;
  }
}

function quickSortScores(items: ScoreItem[], left: number, right: number): void {
  if (left >= right) return;
  const pivot = partitionScores(items, left, right, Math.floor((left + right) / 2));
  quickSortScores(items, left, pivot - 1);
  quickSortScores(items, pivot + 1, right);
}

function mergeSortScores(items: ScoreItem[]): ScoreItem[] {
  if (items.length <= 1) return items;
  const mid = Math.floor(items.length / 2);
  const left = mergeSortScores(items.slice(0, mid));
  const right = mergeSortScores(items.slice(mid));
  const merged: ScoreItem[] = [];
  let i = 0;
  let j = 0;
  while (i < left.length && j < right.length) {
    if (betterScore(left[i], right[j])) merged.push(left[i++]);
    else merged.push(right[j++]);
  }
  return merged.concat(left.slice(i), right.slice(j));
}

function radixSortByKey(items: ScoreItem[], keys: number[]): ScoreItem[] {
  if (items.length <= 1) return items;
  let maxKey = Math.max(...keys, 0);
  for (let exp = 1; Math.floor(maxKey / exp) > 0; exp *= 10) {
    const bucketsByDigit: Array<Array<{ item: ScoreItem; key: number }>> = Array.from({ length: 10 }, () => []);
    for (let i = 0; i < items.length; i++) bucketsByDigit[Math.floor(keys[i] / exp) % 10].push({ item: items[i], key: keys[i] });
    const pairs = bucketsByDigit.flat();
    items = pairs.map((pair) => pair.item);
    keys = pairs.map((pair) => pair.key);
  }
  return items;
}

function buildSearchDocs(source: Profile[]): SearchDoc[] {
  return source.map((profile) => {
    const text = profileSearchText(profile);
    return { id: profile.id, text, tokens: text.split(/\s+/).filter(Boolean) };
  });
}

function profileSearchText(profile: Profile): string {
  return `${profile.name} ${profile.email} ${profile.region} ${profile.bio}`.toLowerCase();
}

function pushIndex(index: Map<string, number[]>, key: string, id: number): void {
  const ids = index.get(key);
  if (!ids) {
    index.set(key, [id]);
    return;
  }
  if (ids[ids.length - 1] !== id) ids.push(id);
}

function searchWith(docs: SearchDoc[], query: string, limit: number, contains: (text: string, pattern: string) => boolean): number[] {
  if (!query) return [];
  const ids: number[] = [];
  for (const doc of docs) {
    if (contains(doc.text, query)) ids.push(doc.id);
    if (ids.length === limit) break;
  }
  return ids;
}

function containsKMP(text: string, pattern: string): boolean {
  if (pattern === "") return true;
  if (pattern.length > text.length) return false;
  const lps = Array(pattern.length).fill(0);
  for (let i = 1, length = 0; i < pattern.length;) {
    if (pattern[i] === pattern[length]) lps[i++] = ++length;
    else if (length) length = lps[length - 1];
    else i++;
  }
  for (let i = 0, j = 0; i < text.length;) {
    if (text[i] === pattern[j]) {
      i++;
      j++;
      if (j === pattern.length) return true;
    } else if (j) j = lps[j - 1];
    else i++;
  }
  return false;
}

function containsBoyerMoore(text: string, pattern: string): boolean {
  if (pattern === "") return true;
  if (pattern.length > text.length) return false;
  const last = new Map<string, number>();
  for (let i = 0; i < pattern.length; i++) last.set(pattern[i], i);
  for (let shift = 0; shift <= text.length - pattern.length;) {
    let j = pattern.length - 1;
    while (j >= 0 && pattern[j] === text[shift + j]) j--;
    if (j < 0) return true;
    shift += Math.max(1, j - (last.get(text[shift + j]) ?? -1));
  }
  return false;
}

type Runner = (request: WorkerRequest) => number[];

function defaultDataStructures(runners: [string, Runner][]): Map<string, Map<string, Runner>> {
  return new Map(runners.map(([name, runner]) => [name, new Map([["default", runner]])]));
}

const algorithms = new Map<string, Map<string, Map<string, Runner>>>([
  ["lookup", defaultDataStructures([
    ["slice_scan", findSliceScan],
    ["binary_search", findBinarySearch],
    ["bucketed_index", findBucketedIndex],
    ["map_scan", findMapScan],
    ["parallel_scan", findParallelScan],
  ])],
  ["membership", new Map([
    ["scan_contains", new Map([
      ["slice", runScanContainsSlice],
      ["sorted_slice", runScanContainsSortedSlice],
    ])],
    ["binary_search_contains", new Map([
      ["sorted_slice", runBinarySearchContainsSortedSlice],
    ])],
    ["direct_lookup", new Map([
      ["hash_set", runDirectLookupHashSet],
    ])],
  ])],
  ["top_k", defaultDataStructures([
    ["top_k_full_sort", runTopKFullSort],
    ["top_k_min_heap", runTopKMinHeap],
    ["top_k_quickselect", runTopKQuickselect],
    ["top_k_bucketed", runTopKBucketed],
    ["top_k_streaming", runTopKStreaming],
  ])],
  ["sorting", defaultDataStructures([
    ["sort_insertion", runSortInsertion],
    ["sort_merge", runSortMerge],
    ["sort_quick", runSortQuick],
    ["sort_heap", runSortHeap],
    ["sort_counting", runSortCounting],
    ["sort_radix", runSortRadix],
    ["sort_builtin", runSortBuiltin],
  ])],
  ["caching", defaultDataStructures([
    ["cache_none", runCacheNone],
    ["cache_fifo", runCacheFIFO],
    ["cache_lru", runCacheLRU],
    ["cache_lfu", runCacheLFU],
    ["cache_random", runCacheRandom],
    ["cache_ttl", runCacheTTL],
  ])],
  ["text_search", defaultDataStructures([
    ["text_naive", runTextNaive],
    ["text_lowercase", runTextLowercase],
    ["text_kmp", runTextKMP],
    ["text_boyer_moore", runTextBoyerMoore],
    ["text_trie_prefix", runTextTriePrefix],
    ["text_inverted_index", runTextInvertedIndex],
  ])],
]);

function writeResponse(value: unknown): void {
  stdout.write(JSON.stringify(value) + "\n");
}

function handleRun(request: WorkerRequest): unknown {
  const scenario = request.scenario || "lookup";
  const dataStructure = request.dataStructure || "default";
  const runner = algorithms.get(scenario)?.get(request.algorithm || "")?.get(dataStructure);
  if (!runner) return { id: request.id, ok: false, error: "unknown implementation" };

  const started = performance.now();
  const ids = runner(request);
  const elapsedMicros = Math.round((performance.now() - started) * 1000);

  const response: { id?: number; ok: boolean; count: number; elapsedMicros: number; ids?: number[] } = {
    id: request.id,
    ok: true,
    count: ids.length,
    elapsedMicros,
  };
  if (request.includeIds !== false) response.ids = ids;
  return response;
}

const lines = createInterface({ input: stdin, crlfDelay: Infinity });

lines.on("line", (line) => {
  try {
    const request = JSON.parse(line) as WorkerRequest;
    if (request.type === "shutdown") {
      lines.close();
      return;
    }
    if (request.type === "init") {
      buildDataset(request.profileCount || 0, BigInt(request.generatedAtUnixNano || "0"));
      writeResponse({ id: request.id, ok: true });
      return;
    }
    if (request.type === "run" || request.type === "find") {
      writeResponse(handleRun(request));
      return;
    }
    writeResponse({ id: request.id, ok: false, error: "unknown request type" });
  } catch (error) {
    writeResponse({ id: undefined, ok: false, error: error instanceof Error ? error.message : String(error) });
  }
});
