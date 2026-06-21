import { cpus } from "node:os";
import { stdin, stdout } from "node:process";
import { createInterface } from "node:readline";
import { performance } from "node:perf_hooks";

const DAY_NS = 24n * 60n * 60n * 1_000_000_000n;
const MS_NS = 1_000_000n;

interface Profile {
  id: number;
  updatedNs: bigint;
}

interface WorkerRequest {
  id?: number;
  type?: string;
  profileCount?: number;
  generatedAtUnixNano?: string;
  algorithm?: string;
  sinceUnixNano?: string;
  includeIds?: boolean;
}

let profiles: Profile[] = [];
let sortedProfiles: Profile[] = [];
let sortedUpdatedNs: bigint[] = [];
let profileMap = new Map<number, bigint>();
let buckets = new Map<bigint, number[]>();
let minutes: bigint[] = [];
let workers = Math.min(Math.max(cpus().length || 2, 2), 16);

function buildDataset(count: number, generatedAtNs: bigint): void {
  const stepNs = DAY_NS / BigInt(Math.max(count, 1));

  profiles = [];
  profileMap = new Map<number, bigint>();
  buckets = new Map<bigint, number[]>();

  for (let i = 0; i < count; i++) {
    const id = i + 1;
    const updatedNs = generatedAtNs - DAY_NS + BigInt(i) * stepNs + BigInt((i * 37) % 997) * MS_NS;
    profiles.push({ id, updatedNs });
    profileMap.set(id, updatedNs);

    const minute = updatedNs / 1_000_000_000n / 60n;
    const bucket = buckets.get(minute);
    if (bucket) bucket.push(id);
    else buckets.set(minute, [id]);
  }

  sortedProfiles = [...profiles].sort((a, b) => {
    if (a.updatedNs === b.updatedNs) return a.id - b.id;
    return a.updatedNs < b.updatedNs ? -1 : 1;
  });
  sortedUpdatedNs = sortedProfiles.map((profile) => profile.updatedNs);
  minutes = [...buckets.keys()].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
}

// snippet:slice_scan:start
function findSliceScan(sinceNs: bigint): number[] {
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

function findBinarySearch(sinceNs: bigint): number[] {
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

function findBucketedIndex(sinceNs: bigint): number[] {
  const minute = sinceNs / 1_000_000_000n / 60n;
  const index = firstMinuteAtLeast(minutes, minute);

  const ids: number[] = [];
  for (const bucketMinute of minutes.slice(index)) {
    for (const id of buckets.get(bucketMinute) || []) {
      if (bucketMinute === minute && (profileMap.get(id) || 0n) <= sinceNs) continue;
      ids.push(id);
    }
  }
  return ids;
}
// snippet:bucketed_index:end

// snippet:map_scan:start
function findMapScan(sinceNs: bigint): number[] {
  const ids: number[] = [];
  for (const [id, updatedNs] of profileMap) {
    if (updatedNs > sinceNs) ids.push(id);
  }
  ids.sort((a, b) => a - b);
  return ids;
}
// snippet:map_scan:end

// snippet:parallel_scan:start
function findParallelScan(sinceNs: bigint): number[] {
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

const finders = new Map<string, (sinceNs: bigint) => number[]>([
  ["slice_scan", findSliceScan],
  ["binary_search", findBinarySearch],
  ["bucketed_index", findBucketedIndex],
  ["map_scan", findMapScan],
  ["parallel_scan", findParallelScan],
]);

function writeResponse(value: unknown): void {
  stdout.write(JSON.stringify(value) + "\n");
}

function handleFind(request: WorkerRequest): unknown {
  const finder = finders.get(request.algorithm || "");
  if (!finder) return { id: request.id, ok: false, error: "unknown algorithm" };

  const sinceNs = BigInt(request.sinceUnixNano || "0");
  const started = performance.now();
  const ids = finder(sinceNs);
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
    if (request.type === "find") {
      writeResponse(handleFind(request));
      return;
    }
    writeResponse({ id: request.id, ok: false, error: "unknown request type" });
  } catch (error) {
    writeResponse({ id: undefined, ok: false, error: error instanceof Error ? error.message : String(error) });
  }
});
