package main

import (
	"runtime"
	"sort"
	"sync"
	"time"
)

type ProfileFinder interface {
	Name() string
	Label() string
	Complexity() string
	Description() string
	Find(since time.Time) []int
}

type FinderInfo struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Complexity  string `json:"complexity"`
	Description string `json:"description"`
	Code        string `json:"code"`
}

func finderInfo(f ProfileFinder) FinderInfo {
	return FinderInfo{
		Name:        f.Name(),
		Label:       f.Label(),
		Complexity:  f.Complexity(),
		Description: f.Description(),
		Code:        finderCodeFor(f.Name()),
	}
}

type SliceScanFinder struct {
	profiles []Profile
}

func NewSliceScanFinder(profiles []Profile) *SliceScanFinder {
	return &SliceScanFinder{profiles: profiles}
}

func (f *SliceScanFinder) Name() string {
	return "slice_scan"
}

func (f *SliceScanFinder) Label() string {
	return "Slice scan"
}

func (f *SliceScanFinder) Complexity() string {
	return "O(N)"
}

func (f *SliceScanFinder) Description() string {
	return "Ranges over every profile and filters by LastUpdated."
}

func (f *SliceScanFinder) Find(since time.Time) []int {
	ids := make([]int, 0, 1024)
	for _, p := range f.profiles {
		if p.LastUpdated.After(since) {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

type BinarySearchFinder struct {
	profiles []Profile
}

func NewBinarySearchFinder(sortedProfiles []Profile) *BinarySearchFinder {
	return &BinarySearchFinder{profiles: sortedProfiles}
}

func (f *BinarySearchFinder) Name() string {
	return "binary_search"
}

func (f *BinarySearchFinder) Label() string {
	return "Binary search"
}

func (f *BinarySearchFinder) Complexity() string {
	return "O(log N + K)"
}

func (f *BinarySearchFinder) Description() string {
	return "Keeps profiles sorted by LastUpdated and jumps directly to the first match."
}

func (f *BinarySearchFinder) Find(since time.Time) []int {
	idx := sort.Search(len(f.profiles), func(i int) bool {
		return f.profiles[i].LastUpdated.After(since)
	})
	ids := make([]int, len(f.profiles)-idx)
	for i, p := range f.profiles[idx:] {
		ids[i] = p.ID
	}
	return ids
}

type MapScanFinder struct {
	profiles map[int]Profile
}

func NewMapScanFinder(profileMap map[int]Profile) *MapScanFinder {
	return &MapScanFinder{profiles: profileMap}
}

func (f *MapScanFinder) Name() string {
	return "map_scan"
}

func (f *MapScanFinder) Label() string {
	return "Map scan"
}

func (f *MapScanFinder) Complexity() string {
	return "O(N)"
}

func (f *MapScanFinder) Description() string {
	return "Stores profiles in a hash map but still has to iterate every value."
}

func (f *MapScanFinder) Find(since time.Time) []int {
	ids := make([]int, 0, 1024)
	for _, p := range f.profiles {
		if p.LastUpdated.After(since) {
			ids = append(ids, p.ID)
		}
	}
	sort.Ints(ids)
	return ids
}

type ParallelScanFinder struct {
	profiles []Profile
	workers  int
}

func NewParallelScanFinder(profiles []Profile) *ParallelScanFinder {
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 16 {
		workers = 16
	}
	return &ParallelScanFinder{
		profiles: profiles,
		workers:  workers,
	}
}

func (f *ParallelScanFinder) Name() string {
	return "parallel_scan"
}

func (f *ParallelScanFinder) Label() string {
	return "Parallel scan"
}

func (f *ParallelScanFinder) Complexity() string {
	return "O(N/C + merge)"
}

func (f *ParallelScanFinder) Description() string {
	return "Splits the full slice scan across goroutines, then merges each worker's result."
}

func (f *ParallelScanFinder) Find(since time.Time) []int {
	if len(f.profiles) == 0 {
		return nil
	}

	workers := min(f.workers, len(f.profiles))
	chunkSize := (len(f.profiles) + workers - 1) / workers
	parts := make([][]int, workers)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start := worker * chunkSize
		end := min(start+chunkSize, len(f.profiles))
		if start >= end {
			continue
		}

		wg.Add(1)
		go func(worker int, start int, end int) {
			defer wg.Done()
			local := make([]int, 0, 256)
			for _, p := range f.profiles[start:end] {
				if p.LastUpdated.After(since) {
					local = append(local, p.ID)
				}
			}
			parts[worker] = local
		}(worker, start, end)
	}

	wg.Wait()

	total := 0
	for _, part := range parts {
		total += len(part)
	}

	ids := make([]int, 0, total)
	for _, part := range parts {
		ids = append(ids, part...)
	}
	return ids
}

type BucketedIndexFinder struct {
	profileMap map[int]Profile
	minutes    []int64
	buckets    map[int64][]int
}

func NewBucketedIndexFinder(profiles []Profile, profileMap map[int]Profile) *BucketedIndexFinder {
	buckets := make(map[int64][]int)
	for _, p := range profiles {
		minute := p.LastUpdated.Unix() / 60
		buckets[minute] = append(buckets[minute], p.ID)
	}

	minutes := make([]int64, 0, len(buckets))
	for minute := range buckets {
		minutes = append(minutes, minute)
	}
	sort.Slice(minutes, func(i, j int) bool {
		return minutes[i] < minutes[j]
	})

	return &BucketedIndexFinder{
		profileMap: profileMap,
		minutes:    minutes,
		buckets:    buckets,
	}
}

func (f *BucketedIndexFinder) Name() string {
	return "bucketed_index"
}

func (f *BucketedIndexFinder) Label() string {
	return "Bucketed index"
}

func (f *BucketedIndexFinder) Complexity() string {
	return "O(log B + K)"
}

func (f *BucketedIndexFinder) Description() string {
	return "Maintains an auxiliary minute index, then scans only buckets that can contain matches."
}

func (f *BucketedIndexFinder) Find(since time.Time) []int {
	minute := since.Unix() / 60
	idx := sort.Search(len(f.minutes), func(i int) bool {
		return f.minutes[i] >= minute
	})

	ids := make([]int, 0, 1024)
	for _, bucketMinute := range f.minutes[idx:] {
		for _, id := range f.buckets[bucketMinute] {
			if bucketMinute == minute {
				if f.profileMap[id].LastUpdated.After(since) {
					ids = append(ids, id)
				}
				continue
			}
			ids = append(ids, id)
		}
	}

	return ids
}
