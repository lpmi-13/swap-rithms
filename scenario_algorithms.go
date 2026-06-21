package main

import (
	"container/heap"
	"container/list"
	"sort"
	"strings"
	"sync"
	"time"
)

type TopKFullSortAlgorithm struct {
	profiles []Profile
}

func NewTopKFullSortAlgorithm(profiles []Profile) *TopKFullSortAlgorithm {
	return &TopKFullSortAlgorithm{profiles: profiles}
}

func (a *TopKFullSortAlgorithm) Name() string       { return "top_k_full_sort" }
func (a *TopKFullSortAlgorithm) Label() string      { return "Full sort then take K" }
func (a *TopKFullSortAlgorithm) Complexity() string { return "O(N log N)" }
func (a *TopKFullSortAlgorithm) Description() string {
	return "Sorts every profile by score, then returns the first K IDs."
}
func (a *TopKFullSortAlgorithm) Run(request ScenarioRequest) []int {
	items := sortedScoreItems(a.profiles)
	return idsFromScoreItems(items, min(request.Limit, len(items)))
}

type TopKMinHeapAlgorithm struct {
	profiles []Profile
}

func NewTopKMinHeapAlgorithm(profiles []Profile) *TopKMinHeapAlgorithm {
	return &TopKMinHeapAlgorithm{profiles: profiles}
}

func (a *TopKMinHeapAlgorithm) Name() string       { return "top_k_min_heap" }
func (a *TopKMinHeapAlgorithm) Label() string      { return "Min-heap of size K" }
func (a *TopKMinHeapAlgorithm) Complexity() string { return "O(N log K)" }
func (a *TopKMinHeapAlgorithm) Description() string {
	return "Keeps only the current K best profiles in a small heap."
}
func (a *TopKMinHeapAlgorithm) Run(request ScenarioRequest) []int {
	limit := min(request.Limit, len(a.profiles))
	h := &scoreMinHeap{}
	heap.Init(h)
	for _, profile := range a.profiles {
		item := scoreItem{ID: profile.ID, Score: profile.Score}
		if h.Len() < limit {
			heap.Push(h, item)
			continue
		}
		if limit > 0 && betterScore(item, (*h)[0]) {
			heap.Pop(h)
			heap.Push(h, item)
		}
	}
	items := append([]scoreItem(nil), (*h)...)
	sortScoreItems(items)
	return idsFromScoreItems(items, limit)
}

type TopKQuickselectAlgorithm struct {
	profiles []Profile
}

func NewTopKQuickselectAlgorithm(profiles []Profile) *TopKQuickselectAlgorithm {
	return &TopKQuickselectAlgorithm{profiles: profiles}
}

func (a *TopKQuickselectAlgorithm) Name() string       { return "top_k_quickselect" }
func (a *TopKQuickselectAlgorithm) Label() string      { return "Quickselect" }
func (a *TopKQuickselectAlgorithm) Complexity() string { return "O(N) avg + O(K log K)" }
func (a *TopKQuickselectAlgorithm) Description() string {
	return "Partitions around the Kth ranked score, then sorts only the selected K."
}
func (a *TopKQuickselectAlgorithm) Run(request ScenarioRequest) []int {
	limit := min(request.Limit, len(a.profiles))
	items := scoreItems(a.profiles)
	quickselectScore(items, limit)
	items = items[:limit]
	sortScoreItems(items)
	return idsFromScoreItems(items, limit)
}

type TopKBucketedAlgorithm struct {
	buckets [1001][]int
}

func NewTopKBucketedAlgorithm(profiles []Profile) *TopKBucketedAlgorithm {
	algorithm := &TopKBucketedAlgorithm{}
	for _, profile := range profiles {
		algorithm.buckets[profile.Score] = append(algorithm.buckets[profile.Score], profile.ID)
	}
	return algorithm
}

func (a *TopKBucketedAlgorithm) Name() string       { return "top_k_bucketed" }
func (a *TopKBucketedAlgorithm) Label() string      { return "Bucketed ranking" }
func (a *TopKBucketedAlgorithm) Complexity() string { return "O(S + K)" }
func (a *TopKBucketedAlgorithm) Description() string {
	return "Uses bounded score buckets, then walks scores from high to low."
}
func (a *TopKBucketedAlgorithm) Run(request ScenarioRequest) []int {
	ids := make([]int, 0, request.Limit)
	for score := len(a.buckets) - 1; score >= 0 && len(ids) < request.Limit; score-- {
		for _, id := range a.buckets[score] {
			ids = append(ids, id)
			if len(ids) == request.Limit {
				break
			}
		}
	}
	return ids
}

type TopKStreamingAlgorithm struct {
	profiles []Profile
}

func NewTopKStreamingAlgorithm(profiles []Profile) *TopKStreamingAlgorithm {
	return &TopKStreamingAlgorithm{profiles: profiles}
}

func (a *TopKStreamingAlgorithm) Name() string       { return "top_k_streaming" }
func (a *TopKStreamingAlgorithm) Label() string      { return "Streaming top-K" }
func (a *TopKStreamingAlgorithm) Complexity() string { return "O(N log K)" }
func (a *TopKStreamingAlgorithm) Description() string {
	return "Treats profiles as an online stream and keeps a bounded leaderboard."
}
func (a *TopKStreamingAlgorithm) Run(request ScenarioRequest) []int {
	return NewTopKMinHeapAlgorithm(a.profiles).Run(request)
}

type InsertionSortAlgorithm struct {
	profiles []Profile
}

func NewInsertionSortAlgorithm(profiles []Profile) *InsertionSortAlgorithm {
	return &InsertionSortAlgorithm{profiles: profiles}
}

func (a *InsertionSortAlgorithm) Name() string       { return "sort_insertion" }
func (a *InsertionSortAlgorithm) Label() string      { return "Insertion sort" }
func (a *InsertionSortAlgorithm) Complexity() string { return "O(N^2)" }
func (a *InsertionSortAlgorithm) Description() string {
	return "Builds the ordered list one profile at a time."
}
func (a *InsertionSortAlgorithm) Run(request ScenarioRequest) []int {
	items := scoreItems(a.profiles)
	for i := 1; i < len(items); i++ {
		current := items[i]
		j := i - 1
		for j >= 0 && betterScore(current, items[j]) {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = current
	}
	return idsFromScoreItems(items, min(request.Limit, len(items)))
}

type MergeSortAlgorithm struct {
	profiles []Profile
}

func NewMergeSortAlgorithm(profiles []Profile) *MergeSortAlgorithm {
	return &MergeSortAlgorithm{profiles: profiles}
}

func (a *MergeSortAlgorithm) Name() string       { return "sort_merge" }
func (a *MergeSortAlgorithm) Label() string      { return "Merge sort" }
func (a *MergeSortAlgorithm) Complexity() string { return "O(N log N)" }
func (a *MergeSortAlgorithm) Description() string {
	return "Recursively sorts halves, then merges them into score order."
}
func (a *MergeSortAlgorithm) Run(request ScenarioRequest) []int {
	items := scoreItems(a.profiles)
	mergeSortScore(items)
	return idsFromScoreItems(items, min(request.Limit, len(items)))
}

type QuickSortAlgorithm struct {
	profiles []Profile
}

func NewQuickSortAlgorithm(profiles []Profile) *QuickSortAlgorithm {
	return &QuickSortAlgorithm{profiles: profiles}
}

func (a *QuickSortAlgorithm) Name() string       { return "sort_quick" }
func (a *QuickSortAlgorithm) Label() string      { return "Quick sort" }
func (a *QuickSortAlgorithm) Complexity() string { return "O(N log N) avg" }
func (a *QuickSortAlgorithm) Description() string {
	return "Partitions around pivots until every score segment is ordered."
}
func (a *QuickSortAlgorithm) Run(request ScenarioRequest) []int {
	items := scoreItems(a.profiles)
	quickSortScore(items, 0, len(items)-1)
	return idsFromScoreItems(items, min(request.Limit, len(items)))
}

type HeapSortAlgorithm struct {
	profiles []Profile
}

func NewHeapSortAlgorithm(profiles []Profile) *HeapSortAlgorithm {
	return &HeapSortAlgorithm{profiles: profiles}
}

func (a *HeapSortAlgorithm) Name() string       { return "sort_heap" }
func (a *HeapSortAlgorithm) Label() string      { return "Heap sort" }
func (a *HeapSortAlgorithm) Complexity() string { return "O(N log N)" }
func (a *HeapSortAlgorithm) Description() string {
	return "Heapifies all profiles, then repeatedly removes the current best."
}
func (a *HeapSortAlgorithm) Run(request ScenarioRequest) []int {
	h := scoreMaxHeap(scoreItems(a.profiles))
	heap.Init(&h)
	limit := min(request.Limit, len(h))
	ids := make([]int, 0, limit)
	for len(ids) < limit {
		ids = append(ids, heap.Pop(&h).(scoreItem).ID)
	}
	return ids
}

type CountingSortAlgorithm struct {
	buckets [1001][]int
	total   int
}

func NewCountingSortAlgorithm(profiles []Profile) *CountingSortAlgorithm {
	algorithm := &CountingSortAlgorithm{total: len(profiles)}
	for _, profile := range profiles {
		algorithm.buckets[profile.Score] = append(algorithm.buckets[profile.Score], profile.ID)
	}
	return algorithm
}

func (a *CountingSortAlgorithm) Name() string       { return "sort_counting" }
func (a *CountingSortAlgorithm) Label() string      { return "Counting sort" }
func (a *CountingSortAlgorithm) Complexity() string { return "O(N + S)" }
func (a *CountingSortAlgorithm) Description() string {
	return "Uses the bounded score range as direct buckets."
}
func (a *CountingSortAlgorithm) Run(request ScenarioRequest) []int {
	limit := min(request.Limit, a.total)
	ids := make([]int, 0, limit)
	for score := len(a.buckets) - 1; score >= 0 && len(ids) < limit; score-- {
		for _, id := range a.buckets[score] {
			ids = append(ids, id)
			if len(ids) == limit {
				break
			}
		}
	}
	return ids
}

type RadixSortAlgorithm struct {
	profiles []Profile
	maxID    int
}

func NewRadixSortAlgorithm(profiles []Profile) *RadixSortAlgorithm {
	maxID := 0
	for _, profile := range profiles {
		maxID = max(maxID, profile.ID)
	}
	return &RadixSortAlgorithm{profiles: profiles, maxID: maxID}
}

func (a *RadixSortAlgorithm) Name() string       { return "sort_radix" }
func (a *RadixSortAlgorithm) Label() string      { return "Radix sort" }
func (a *RadixSortAlgorithm) Complexity() string { return "O(WN)" }
func (a *RadixSortAlgorithm) Description() string {
	return "Sorts fixed-width score and ID rank keys digit by digit."
}
func (a *RadixSortAlgorithm) Run(request ScenarioRequest) []int {
	items := scoreItems(a.profiles)
	keys := make([]int, len(items))
	base := a.maxID + 1
	for i, item := range items {
		keys[i] = item.Score*base + (a.maxID - item.ID)
	}
	radixSortByKey(items, keys)
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return idsFromScoreItems(items, min(request.Limit, len(items)))
}

type BuiltinSortAlgorithm struct {
	profiles []Profile
}

func NewBuiltinSortAlgorithm(profiles []Profile) *BuiltinSortAlgorithm {
	return &BuiltinSortAlgorithm{profiles: profiles}
}

func (a *BuiltinSortAlgorithm) Name() string       { return "sort_builtin" }
func (a *BuiltinSortAlgorithm) Label() string      { return "Built-in sort" }
func (a *BuiltinSortAlgorithm) Complexity() string { return "O(N log N)" }
func (a *BuiltinSortAlgorithm) Description() string {
	return "Uses the language runtime's production sort as a baseline."
}
func (a *BuiltinSortAlgorithm) Run(request ScenarioRequest) []int {
	items := sortedScoreItems(a.profiles)
	return idsFromScoreItems(items, min(request.Limit, len(items)))
}

type NoCacheAlgorithm struct {
	profiles map[int]Profile
}

func NewNoCacheAlgorithm(profiles map[int]Profile) *NoCacheAlgorithm {
	return &NoCacheAlgorithm{profiles: profiles}
}

func (a *NoCacheAlgorithm) Name() string       { return "cache_none" }
func (a *NoCacheAlgorithm) Label() string      { return "No cache baseline" }
func (a *NoCacheAlgorithm) Complexity() string { return "O(1)" }
func (a *NoCacheAlgorithm) Description() string {
	return "Reads every requested profile directly from the backing map."
}
func (a *NoCacheAlgorithm) Run(request ScenarioRequest) []int {
	if _, ok := a.profiles[request.ProfileID]; !ok {
		return nil
	}
	return []int{request.ProfileID}
}

type FIFOCacheAlgorithm struct {
	mu       sync.Mutex
	profiles map[int]Profile
	capacity int
	cache    map[int]Profile
	order    []int
}

func NewFIFOCacheAlgorithm(profiles map[int]Profile, capacity int) *FIFOCacheAlgorithm {
	return &FIFOCacheAlgorithm{profiles: profiles, capacity: capacity, cache: make(map[int]Profile, capacity)}
}

func (a *FIFOCacheAlgorithm) Name() string       { return "cache_fifo" }
func (a *FIFOCacheAlgorithm) Label() string      { return "FIFO cache" }
func (a *FIFOCacheAlgorithm) Complexity() string { return "O(1)" }
func (a *FIFOCacheAlgorithm) Description() string {
	return "Evicts the oldest cached profile when the cache reaches capacity."
}
func (a *FIFOCacheAlgorithm) Run(request ScenarioRequest) []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.cache[request.ProfileID]; ok {
		return []int{request.ProfileID}
	}
	profile, ok := a.profiles[request.ProfileID]
	if !ok {
		return nil
	}
	if len(a.cache) >= a.capacity && len(a.order) > 0 {
		delete(a.cache, a.order[0])
		a.order = a.order[1:]
	}
	a.cache[request.ProfileID] = profile
	a.order = append(a.order, request.ProfileID)
	return []int{request.ProfileID}
}

type LRUCacheAlgorithm struct {
	mu       sync.Mutex
	profiles map[int]Profile
	capacity int
	cache    map[int]*list.Element
	order    *list.List
}

type cacheEntry struct {
	id      int
	profile Profile
}

func NewLRUCacheAlgorithm(profiles map[int]Profile, capacity int) *LRUCacheAlgorithm {
	return &LRUCacheAlgorithm{profiles: profiles, capacity: capacity, cache: make(map[int]*list.Element, capacity), order: list.New()}
}

func (a *LRUCacheAlgorithm) Name() string       { return "cache_lru" }
func (a *LRUCacheAlgorithm) Label() string      { return "LRU cache" }
func (a *LRUCacheAlgorithm) Complexity() string { return "O(1)" }
func (a *LRUCacheAlgorithm) Description() string {
	return "Evicts the least recently used profile."
}
func (a *LRUCacheAlgorithm) Run(request ScenarioRequest) []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if element := a.cache[request.ProfileID]; element != nil {
		a.order.MoveToFront(element)
		return []int{request.ProfileID}
	}
	profile, ok := a.profiles[request.ProfileID]
	if !ok {
		return nil
	}
	if len(a.cache) >= a.capacity {
		oldest := a.order.Back()
		if oldest != nil {
			delete(a.cache, oldest.Value.(cacheEntry).id)
			a.order.Remove(oldest)
		}
	}
	a.cache[request.ProfileID] = a.order.PushFront(cacheEntry{id: request.ProfileID, profile: profile})
	return []int{request.ProfileID}
}

type LFUCacheAlgorithm struct {
	mu       sync.Mutex
	profiles map[int]Profile
	capacity int
	cache    map[int]Profile
	counts   map[int]int
}

func NewLFUCacheAlgorithm(profiles map[int]Profile, capacity int) *LFUCacheAlgorithm {
	return &LFUCacheAlgorithm{profiles: profiles, capacity: capacity, cache: make(map[int]Profile, capacity), counts: make(map[int]int, capacity)}
}

func (a *LFUCacheAlgorithm) Name() string       { return "cache_lfu" }
func (a *LFUCacheAlgorithm) Label() string      { return "LFU cache" }
func (a *LFUCacheAlgorithm) Complexity() string { return "O(C)" }
func (a *LFUCacheAlgorithm) Description() string {
	return "Evicts the profile with the lowest access count."
}
func (a *LFUCacheAlgorithm) Run(request ScenarioRequest) []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.cache[request.ProfileID]; ok {
		a.counts[request.ProfileID]++
		return []int{request.ProfileID}
	}
	profile, ok := a.profiles[request.ProfileID]
	if !ok {
		return nil
	}
	if len(a.cache) >= a.capacity {
		victim := 0
		for id := range a.cache {
			if victim == 0 || a.counts[id] < a.counts[victim] || (a.counts[id] == a.counts[victim] && id > victim) {
				victim = id
			}
		}
		delete(a.cache, victim)
		delete(a.counts, victim)
	}
	a.cache[request.ProfileID] = profile
	a.counts[request.ProfileID] = 1
	return []int{request.ProfileID}
}

type RandomCacheAlgorithm struct {
	mu       sync.Mutex
	profiles map[int]Profile
	capacity int
	cache    map[int]Profile
}

func NewRandomCacheAlgorithm(profiles map[int]Profile, capacity int) *RandomCacheAlgorithm {
	return &RandomCacheAlgorithm{profiles: profiles, capacity: capacity, cache: make(map[int]Profile, capacity)}
}

func (a *RandomCacheAlgorithm) Name() string       { return "cache_random" }
func (a *RandomCacheAlgorithm) Label() string      { return "Random eviction" }
func (a *RandomCacheAlgorithm) Complexity() string { return "O(C)" }
func (a *RandomCacheAlgorithm) Description() string {
	return "Evicts a deterministic pseudo-random cached profile."
}
func (a *RandomCacheAlgorithm) Run(request ScenarioRequest) []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.cache[request.ProfileID]; ok {
		return []int{request.ProfileID}
	}
	profile, ok := a.profiles[request.ProfileID]
	if !ok {
		return nil
	}
	if len(a.cache) >= a.capacity {
		keys := make([]int, 0, len(a.cache))
		for id := range a.cache {
			keys = append(keys, id)
		}
		sort.Ints(keys)
		delete(a.cache, keys[(request.ProfileID*1103515245+len(keys))%len(keys)])
	}
	a.cache[request.ProfileID] = profile
	return []int{request.ProfileID}
}

type TTLCacheAlgorithm struct {
	mu       sync.Mutex
	profiles map[int]Profile
	capacity int
	ttl      time.Duration
	cache    map[int]ttlEntry
	order    []int
}

type ttlEntry struct {
	profile Profile
	expires time.Time
}

func NewTTLCacheAlgorithm(profiles map[int]Profile, capacity int, ttl time.Duration) *TTLCacheAlgorithm {
	return &TTLCacheAlgorithm{profiles: profiles, capacity: capacity, ttl: ttl, cache: make(map[int]ttlEntry, capacity)}
}

func (a *TTLCacheAlgorithm) Name() string       { return "cache_ttl" }
func (a *TTLCacheAlgorithm) Label() string      { return "TTL cache" }
func (a *TTLCacheAlgorithm) Complexity() string { return "O(1)" }
func (a *TTLCacheAlgorithm) Description() string {
	return "Caches profiles until their fixed time-to-live expires."
}
func (a *TTLCacheAlgorithm) Run(request ScenarioRequest) []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if entry, ok := a.cache[request.ProfileID]; ok && now.Before(entry.expires) {
		return []int{request.ProfileID}
	}
	profile, ok := a.profiles[request.ProfileID]
	if !ok {
		return nil
	}
	if len(a.cache) >= a.capacity && len(a.order) > 0 {
		delete(a.cache, a.order[0])
		a.order = a.order[1:]
	}
	a.cache[request.ProfileID] = ttlEntry{profile: profile, expires: now.Add(a.ttl)}
	a.order = append(a.order, request.ProfileID)
	return []int{request.ProfileID}
}

type TextNaiveAlgorithm struct {
	profiles []Profile
}

func NewTextNaiveAlgorithm(profiles []Profile) *TextNaiveAlgorithm {
	return &TextNaiveAlgorithm{profiles: profiles}
}

func (a *TextNaiveAlgorithm) Name() string       { return "text_naive" }
func (a *TextNaiveAlgorithm) Label() string      { return "Naive substring scan" }
func (a *TextNaiveAlgorithm) Complexity() string { return "O(NM)" }
func (a *TextNaiveAlgorithm) Description() string {
	return "Normalizes each profile text and checks for the query substring."
}
func (a *TextNaiveAlgorithm) Run(request ScenarioRequest) []int {
	ids := make([]int, 0, request.Limit)
	for _, profile := range a.profiles {
		if strings.Contains(profileSearchText(profile), request.Query) {
			ids = append(ids, profile.ID)
			if len(ids) == request.Limit {
				break
			}
		}
	}
	return ids
}

type TextLowercaseAlgorithm struct {
	corpus *searchCorpus
}

func NewTextLowercaseAlgorithm(corpus *searchCorpus) *TextLowercaseAlgorithm {
	return &TextLowercaseAlgorithm{corpus: corpus}
}

func (a *TextLowercaseAlgorithm) Name() string       { return "text_lowercase" }
func (a *TextLowercaseAlgorithm) Label() string      { return "Pre-normalized scan" }
func (a *TextLowercaseAlgorithm) Complexity() string { return "O(NM)" }
func (a *TextLowercaseAlgorithm) Description() string {
	return "Precomputes lowercase searchable text, then scans it directly."
}
func (a *TextLowercaseAlgorithm) Run(request ScenarioRequest) []int {
	return searchDocs(a.corpus.docs(), request.Query, request.Limit, strings.Contains)
}

type TextKMPAlgorithm struct {
	corpus *searchCorpus
}

func NewTextKMPAlgorithm(corpus *searchCorpus) *TextKMPAlgorithm {
	return &TextKMPAlgorithm{corpus: corpus}
}

func (a *TextKMPAlgorithm) Name() string       { return "text_kmp" }
func (a *TextKMPAlgorithm) Label() string      { return "KMP substring search" }
func (a *TextKMPAlgorithm) Complexity() string { return "O(N + M)" }
func (a *TextKMPAlgorithm) Description() string {
	return "Builds a query prefix table to avoid rechecking characters."
}
func (a *TextKMPAlgorithm) Run(request ScenarioRequest) []int {
	return searchDocs(a.corpus.docs(), request.Query, request.Limit, containsKMP)
}

type TextBoyerMooreAlgorithm struct {
	corpus *searchCorpus
}

func NewTextBoyerMooreAlgorithm(corpus *searchCorpus) *TextBoyerMooreAlgorithm {
	return &TextBoyerMooreAlgorithm{corpus: corpus}
}

func (a *TextBoyerMooreAlgorithm) Name() string       { return "text_boyer_moore" }
func (a *TextBoyerMooreAlgorithm) Label() string      { return "Boyer-Moore style search" }
func (a *TextBoyerMooreAlgorithm) Complexity() string { return "O(N/M) best" }
func (a *TextBoyerMooreAlgorithm) Description() string {
	return "Uses bad-character skips to jump through each profile text."
}
func (a *TextBoyerMooreAlgorithm) Run(request ScenarioRequest) []int {
	return searchDocs(a.corpus.docs(), request.Query, request.Limit, containsBoyerMoore)
}

type TextTriePrefixAlgorithm struct {
	corpus      *searchCorpus
	once        sync.Once
	docs        []searchDoc
	docsByID    map[int]string
	prefixIndex map[string][]int
}

func NewTextTriePrefixAlgorithm(corpus *searchCorpus) *TextTriePrefixAlgorithm {
	return &TextTriePrefixAlgorithm{corpus: corpus}
}

func (a *TextTriePrefixAlgorithm) Name() string       { return "text_trie_prefix" }
func (a *TextTriePrefixAlgorithm) Label() string      { return "Trie prefix index" }
func (a *TextTriePrefixAlgorithm) Complexity() string { return "O(Q + C)" }
func (a *TextTriePrefixAlgorithm) Description() string {
	return "Uses token prefixes as candidates, then verifies exact substring matches."
}
func (a *TextTriePrefixAlgorithm) Run(request ScenarioRequest) []int {
	a.once.Do(a.buildIndex)
	candidates := a.prefixIndex[request.Query]
	if len(candidates) == 0 || strings.Contains(request.Query, " ") {
		return searchDocs(a.docs, request.Query, request.Limit, strings.Contains)
	}
	ids := make([]int, 0, min(len(candidates), request.Limit))
	for _, id := range candidates {
		if strings.Contains(a.docsByID[id], request.Query) {
			ids = append(ids, id)
			if len(ids) == request.Limit {
				break
			}
		}
	}
	return ids
}

func (a *TextTriePrefixAlgorithm) buildIndex() {
	docs := a.corpus.docs()
	a.docs = docs
	a.docsByID = make(map[int]string, len(docs))
	a.prefixIndex = make(map[string][]int)
	for _, doc := range docs {
		a.docsByID[doc.id] = doc.text
		for _, token := range doc.tokens {
			for i := 1; i <= len(token); i++ {
				prefix := token[:i]
				ids := a.prefixIndex[prefix]
				if len(ids) == 0 || ids[len(ids)-1] != doc.id {
					a.prefixIndex[prefix] = append(ids, doc.id)
				}
			}
		}
	}
}

type TextInvertedIndexAlgorithm struct {
	corpus   *searchCorpus
	once     sync.Once
	docs     []searchDoc
	docsByID map[int]string
	index    map[string][]int
}

func NewTextInvertedIndexAlgorithm(corpus *searchCorpus) *TextInvertedIndexAlgorithm {
	return &TextInvertedIndexAlgorithm{corpus: corpus}
}

func (a *TextInvertedIndexAlgorithm) Name() string       { return "text_inverted_index" }
func (a *TextInvertedIndexAlgorithm) Label() string      { return "Inverted index" }
func (a *TextInvertedIndexAlgorithm) Complexity() string { return "O(T + C)" }
func (a *TextInvertedIndexAlgorithm) Description() string {
	return "Finds token candidates from an inverted index, then verifies substring matches."
}
func (a *TextInvertedIndexAlgorithm) Run(request ScenarioRequest) []int {
	a.once.Do(a.buildIndex)
	tokens := strings.Fields(request.Query)
	if len(tokens) == 0 {
		return nil
	}
	candidates := a.index[tokens[0]]
	if len(candidates) == 0 {
		return searchDocs(a.docs, request.Query, request.Limit, strings.Contains)
	}
	ids := make([]int, 0, min(len(candidates), request.Limit))
	for _, id := range candidates {
		if strings.Contains(a.docsByID[id], request.Query) {
			ids = append(ids, id)
			if len(ids) == request.Limit {
				break
			}
		}
	}
	return ids
}

func (a *TextInvertedIndexAlgorithm) buildIndex() {
	docs := a.corpus.docs()
	a.docs = docs
	a.docsByID = make(map[int]string, len(docs))
	a.index = make(map[string][]int)
	for _, doc := range docs {
		a.docsByID[doc.id] = doc.text
		seen := make(map[string]bool, len(doc.tokens))
		for _, token := range doc.tokens {
			if seen[token] {
				continue
			}
			seen[token] = true
			a.index[token] = append(a.index[token], doc.id)
		}
	}
}

type scoreMinHeap []scoreItem

func (h scoreMinHeap) Len() int               { return len(h) }
func (h scoreMinHeap) Less(i int, j int) bool { return worseScore(h[i], h[j]) }
func (h scoreMinHeap) Swap(i int, j int)      { h[i], h[j] = h[j], h[i] }
func (h *scoreMinHeap) Push(value any)        { *h = append(*h, value.(scoreItem)) }
func (h *scoreMinHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}

type scoreMaxHeap []scoreItem

func (h scoreMaxHeap) Len() int               { return len(h) }
func (h scoreMaxHeap) Less(i int, j int) bool { return betterScore(h[i], h[j]) }
func (h scoreMaxHeap) Swap(i int, j int)      { h[i], h[j] = h[j], h[i] }
func (h *scoreMaxHeap) Push(value any)        { *h = append(*h, value.(scoreItem)) }
func (h *scoreMaxHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}

func quickselectScore(items []scoreItem, limit int) {
	if limit <= 0 || limit >= len(items) {
		return
	}
	left, right := 0, len(items)-1
	target := limit - 1
	for left < right {
		pivot := partitionScore(items, left, right, (left+right)/2)
		switch {
		case pivot == target:
			return
		case pivot > target:
			right = pivot - 1
		default:
			left = pivot + 1
		}
	}
}

func quickSortScore(items []scoreItem, left int, right int) {
	if left >= right {
		return
	}
	pivot := partitionScore(items, left, right, (left+right)/2)
	quickSortScore(items, left, pivot-1)
	quickSortScore(items, pivot+1, right)
}

func partitionScore(items []scoreItem, left int, right int, pivot int) int {
	pivotValue := items[pivot]
	items[pivot], items[right] = items[right], items[pivot]
	store := left
	for i := left; i < right; i++ {
		if betterScore(items[i], pivotValue) {
			items[store], items[i] = items[i], items[store]
			store++
		}
	}
	items[right], items[store] = items[store], items[right]
	return store
}

func mergeSortScore(items []scoreItem) {
	if len(items) <= 1 {
		return
	}
	buffer := make([]scoreItem, len(items))
	var sortRange func(start int, end int)
	sortRange = func(start int, end int) {
		if end-start <= 1 {
			return
		}
		mid := start + (end-start)/2
		sortRange(start, mid)
		sortRange(mid, end)
		i, j, k := start, mid, start
		for i < mid && j < end {
			if betterScore(items[i], items[j]) {
				buffer[k] = items[i]
				i++
			} else {
				buffer[k] = items[j]
				j++
			}
			k++
		}
		for i < mid {
			buffer[k] = items[i]
			i++
			k++
		}
		for j < end {
			buffer[k] = items[j]
			j++
			k++
		}
		copy(items[start:end], buffer[start:end])
	}
	sortRange(0, len(items))
}

func radixSortByKey(items []scoreItem, keys []int) {
	if len(items) <= 1 {
		return
	}
	maxKey := 0
	for _, key := range keys {
		maxKey = max(maxKey, key)
	}
	output := make([]scoreItem, len(items))
	keyOutput := make([]int, len(keys))
	for exp := 1; maxKey/exp > 0; exp *= 10 {
		counts := [10]int{}
		for _, key := range keys {
			counts[(key/exp)%10]++
		}
		for i := 1; i < len(counts); i++ {
			counts[i] += counts[i-1]
		}
		for i := len(items) - 1; i >= 0; i-- {
			digit := (keys[i] / exp) % 10
			counts[digit]--
			pos := counts[digit]
			output[pos] = items[i]
			keyOutput[pos] = keys[i]
		}
		copy(items, output)
		copy(keys, keyOutput)
	}
}

type searchDoc struct {
	id     int
	text   string
	tokens []string
}

type searchCorpus struct {
	profiles []Profile
	once     sync.Once
	cached   []searchDoc
}

func newSearchCorpus(profiles []Profile) *searchCorpus {
	return &searchCorpus{profiles: profiles}
}

func (c *searchCorpus) docs() []searchDoc {
	c.once.Do(func() {
		c.cached = buildSearchDocs(c.profiles)
	})
	return c.cached
}

func buildSearchDocs(profiles []Profile) []searchDoc {
	docs := make([]searchDoc, len(profiles))
	for i, profile := range profiles {
		text := profileSearchText(profile)
		docs[i] = searchDoc{id: profile.ID, text: text, tokens: strings.Fields(text)}
	}
	return docs
}

func profileSearchText(profile Profile) string {
	return strings.ToLower(profile.Name + " " + profile.Email + " " + profile.Region + " " + profile.Bio)
}

func searchDocs(docs []searchDoc, query string, limit int, contains func(string, string) bool) []int {
	if query == "" {
		return nil
	}
	ids := make([]int, 0, min(limit, len(docs)))
	for _, doc := range docs {
		if contains(doc.text, query) {
			ids = append(ids, doc.id)
			if len(ids) == limit {
				break
			}
		}
	}
	return ids
}

func containsKMP(text string, pattern string) bool {
	if pattern == "" {
		return true
	}
	if len(pattern) > len(text) {
		return false
	}
	lps := make([]int, len(pattern))
	for i, length := 1, 0; i < len(pattern); {
		if pattern[i] == pattern[length] {
			length++
			lps[i] = length
			i++
		} else if length != 0 {
			length = lps[length-1]
		} else {
			lps[i] = 0
			i++
		}
	}
	for i, j := 0, 0; i < len(text); {
		if text[i] == pattern[j] {
			i++
			j++
			if j == len(pattern) {
				return true
			}
		} else if j != 0 {
			j = lps[j-1]
		} else {
			i++
		}
	}
	return false
}

func containsBoyerMoore(text string, pattern string) bool {
	if pattern == "" {
		return true
	}
	if len(pattern) > len(text) {
		return false
	}
	last := [256]int{}
	for i := range last {
		last[i] = -1
	}
	for i := 0; i < len(pattern); i++ {
		last[pattern[i]] = i
	}
	for shift := 0; shift <= len(text)-len(pattern); {
		j := len(pattern) - 1
		for j >= 0 && pattern[j] == text[shift+j] {
			j--
		}
		if j < 0 {
			return true
		}
		shift += max(1, j-last[text[shift+j]])
	}
	return false
}
