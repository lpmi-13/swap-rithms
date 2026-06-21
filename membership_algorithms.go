package main

import "sort"

type ScanContainsSliceImplementation struct {
	ids []int
}

func NewScanContainsSliceImplementation(profiles []Profile) *ScanContainsSliceImplementation {
	return &ScanContainsSliceImplementation{ids: profileIDs(profiles)}
}

func (a *ScanContainsSliceImplementation) Name() string          { return "scan_contains_slice" }
func (a *ScanContainsSliceImplementation) Algorithm() string     { return "scan_contains" }
func (a *ScanContainsSliceImplementation) DataStructure() string { return "slice" }
func (a *ScanContainsSliceImplementation) Label() string         { return "Scan contains / slice" }
func (a *ScanContainsSliceImplementation) Complexity() string    { return "O(QN)" }
func (a *ScanContainsSliceImplementation) Description() string {
	return "Checks each requested ID by scanning an unsorted slice until it finds a match."
}
func (a *ScanContainsSliceImplementation) Run(request ScenarioRequest) []int {
	candidates := membershipCandidates(request.Limit, len(a.ids))
	found := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		for _, id := range a.ids {
			if id == candidate {
				found = append(found, candidate)
				break
			}
		}
	}
	return found
}

type ScanContainsSortedSliceImplementation struct {
	ids []int
}

func NewScanContainsSortedSliceImplementation(profiles []Profile) *ScanContainsSortedSliceImplementation {
	ids := profileIDs(profiles)
	sort.Ints(ids)
	return &ScanContainsSortedSliceImplementation{ids: ids}
}

func (a *ScanContainsSortedSliceImplementation) Name() string {
	return "scan_contains_sorted_slice"
}
func (a *ScanContainsSortedSliceImplementation) Algorithm() string     { return "scan_contains" }
func (a *ScanContainsSortedSliceImplementation) DataStructure() string { return "sorted_slice" }
func (a *ScanContainsSortedSliceImplementation) Label() string {
	return "Scan contains / sorted slice"
}
func (a *ScanContainsSortedSliceImplementation) Complexity() string { return "O(QN)" }
func (a *ScanContainsSortedSliceImplementation) Description() string {
	return "Uses the same scan algorithm against IDs stored in sorted order."
}
func (a *ScanContainsSortedSliceImplementation) Run(request ScenarioRequest) []int {
	candidates := membershipCandidates(request.Limit, len(a.ids))
	found := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		for _, id := range a.ids {
			if id == candidate {
				found = append(found, candidate)
				break
			}
		}
	}
	return found
}

type BinarySearchContainsSortedSliceImplementation struct {
	ids []int
}

func NewBinarySearchContainsSortedSliceImplementation(profiles []Profile) *BinarySearchContainsSortedSliceImplementation {
	ids := profileIDs(profiles)
	sort.Ints(ids)
	return &BinarySearchContainsSortedSliceImplementation{ids: ids}
}

func (a *BinarySearchContainsSortedSliceImplementation) Name() string {
	return "binary_search_contains_sorted_slice"
}
func (a *BinarySearchContainsSortedSliceImplementation) Algorithm() string {
	return "binary_search_contains"
}
func (a *BinarySearchContainsSortedSliceImplementation) DataStructure() string { return "sorted_slice" }
func (a *BinarySearchContainsSortedSliceImplementation) Label() string {
	return "Binary search / sorted slice"
}
func (a *BinarySearchContainsSortedSliceImplementation) Complexity() string { return "O(Q log N)" }
func (a *BinarySearchContainsSortedSliceImplementation) Description() string {
	return "Checks each requested ID with binary search over a sorted slice."
}
func (a *BinarySearchContainsSortedSliceImplementation) Run(request ScenarioRequest) []int {
	candidates := membershipCandidates(request.Limit, len(a.ids))
	found := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		idx := sort.SearchInts(a.ids, candidate)
		if idx < len(a.ids) && a.ids[idx] == candidate {
			found = append(found, candidate)
		}
	}
	return found
}

type DirectLookupHashSetImplementation struct {
	ids map[int]struct{}
}

func NewDirectLookupHashSetImplementation(profiles []Profile) *DirectLookupHashSetImplementation {
	ids := make(map[int]struct{}, len(profiles))
	for _, profile := range profiles {
		ids[profile.ID] = struct{}{}
	}
	return &DirectLookupHashSetImplementation{ids: ids}
}

func (a *DirectLookupHashSetImplementation) Name() string          { return "direct_lookup_hash_set" }
func (a *DirectLookupHashSetImplementation) Algorithm() string     { return "direct_lookup" }
func (a *DirectLookupHashSetImplementation) DataStructure() string { return "hash_set" }
func (a *DirectLookupHashSetImplementation) Label() string         { return "Direct lookup / hash set" }
func (a *DirectLookupHashSetImplementation) Complexity() string    { return "O(Q)" }
func (a *DirectLookupHashSetImplementation) Description() string {
	return "Uses profile IDs as hash keys, so each membership check is a direct lookup."
}
func (a *DirectLookupHashSetImplementation) Run(request ScenarioRequest) []int {
	candidates := membershipCandidates(request.Limit, len(a.ids))
	found := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := a.ids[candidate]; ok {
			found = append(found, candidate)
		}
	}
	return found
}

func profileIDs(profiles []Profile) []int {
	ids := make([]int, len(profiles))
	for i, profile := range profiles {
		ids[i] = profile.ID
	}
	return ids
}

func membershipCandidates(limit int, profileCount int) []int {
	if limit <= 0 {
		limit = 100
	}
	span := max(1, profileCount*2)
	candidates := make([]int, limit)
	for i := range candidates {
		candidates[i] = 1 + ((i * 7919) % span)
	}
	return candidates
}
