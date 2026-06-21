package main

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	scenarioLookup     = "lookup"
	scenarioTopK       = "top_k"
	scenarioSorting    = "sorting"
	scenarioCaching    = "caching"
	scenarioTextSearch = "text_search"
)

const (
	defaultLookupAlgorithm     = "slice_scan"
	defaultTopKAlgorithm       = "top_k_min_heap"
	defaultSortingAlgorithm    = "sort_builtin"
	defaultCachingAlgorithm    = "cache_lru"
	defaultTextSearchAlgorithm = "text_lowercase"
)

type ScenarioRequest struct {
	Since         time.Time
	SinceUnixNano string
	IncludeIDs    bool
	Limit         int
	Query         string
	ProfileID     int
}

func (r ScenarioRequest) normalized() ScenarioRequest {
	if r.SinceUnixNano == "" && !r.Since.IsZero() {
		r.SinceUnixNano = formatUnixNano(r.Since)
	}
	if r.Query != "" {
		r.Query = strings.ToLower(strings.TrimSpace(r.Query))
	}
	return r
}

type ScenarioAlgorithm interface {
	Name() string
	Label() string
	Complexity() string
	Description() string
	Run(ScenarioRequest) []int
}

type ScenarioDefinition struct {
	Name             string
	Label            string
	Description      string
	BusinessCase     string
	Endpoint         string
	RequestLabel     string
	RequestUnit      string
	RequestHelp      string
	RequestMin       int
	RequestMax       int
	RequestDefault   int
	QueryLabel       string
	QueryDefault     string
	QueryHelp        string
	DefaultAlgorithm string
	Algorithms       map[string]ScenarioAlgorithm
	Order            []string
	Verify           func(got []int, want []int) bool
}

type ScenarioInfo struct {
	Name             string       `json:"name"`
	Label            string       `json:"label"`
	Description      string       `json:"description"`
	BusinessCase     string       `json:"businessCase"`
	Endpoint         string       `json:"endpoint"`
	Request          RequestInfo  `json:"request"`
	DefaultAlgorithm string       `json:"defaultAlgorithm"`
	Algorithms       []FinderInfo `json:"algorithms"`
}

type RequestInfo struct {
	Label        string `json:"label"`
	Unit         string `json:"unit"`
	Help         string `json:"help"`
	Min          int    `json:"min"`
	Max          int    `json:"max"`
	Default      int    `json:"default"`
	QueryLabel   string `json:"queryLabel,omitempty"`
	QueryDefault string `json:"queryDefault,omitempty"`
	QueryHelp    string `json:"queryHelp,omitempty"`
}

func NewScenarioRegistry(dataset *Dataset) (map[string]*ScenarioDefinition, []string) {
	lookupAlgorithms := []ScenarioAlgorithm{
		lookupAlgorithm{NewSliceScanFinder(dataset.Profiles)},
		lookupAlgorithm{NewBinarySearchFinder(dataset.ProfilesSorted)},
		lookupAlgorithm{NewBucketedIndexFinder(dataset.Profiles, dataset.ProfileMap)},
		lookupAlgorithm{NewMapScanFinder(dataset.ProfileMap)},
		lookupAlgorithm{NewParallelScanFinder(dataset.Profiles)},
	}

	topKAlgorithms := []ScenarioAlgorithm{
		NewTopKFullSortAlgorithm(dataset.Profiles),
		NewTopKMinHeapAlgorithm(dataset.Profiles),
		NewTopKQuickselectAlgorithm(dataset.Profiles),
		NewTopKBucketedAlgorithm(dataset.Profiles),
		NewTopKStreamingAlgorithm(dataset.Profiles),
	}

	sortingProfiles := dataset.Profiles
	if len(sortingProfiles) > 5000 {
		sortingProfiles = sortingProfiles[:5000]
	}
	sortingAlgorithms := []ScenarioAlgorithm{
		NewInsertionSortAlgorithm(sortingProfiles),
		NewMergeSortAlgorithm(sortingProfiles),
		NewQuickSortAlgorithm(sortingProfiles),
		NewHeapSortAlgorithm(sortingProfiles),
		NewCountingSortAlgorithm(sortingProfiles),
		NewRadixSortAlgorithm(sortingProfiles),
		NewBuiltinSortAlgorithm(sortingProfiles),
	}

	cacheAlgorithms := []ScenarioAlgorithm{
		NewNoCacheAlgorithm(dataset.ProfileMap),
		NewFIFOCacheAlgorithm(dataset.ProfileMap, 256),
		NewLRUCacheAlgorithm(dataset.ProfileMap, 256),
		NewLFUCacheAlgorithm(dataset.ProfileMap, 256),
		NewRandomCacheAlgorithm(dataset.ProfileMap, 256),
		NewTTLCacheAlgorithm(dataset.ProfileMap, 256, time.Minute),
	}

	textCorpus := newSearchCorpus(dataset.Profiles)
	textSearchAlgorithms := []ScenarioAlgorithm{
		NewTextNaiveAlgorithm(dataset.Profiles),
		NewTextLowercaseAlgorithm(textCorpus),
		NewTextKMPAlgorithm(textCorpus),
		NewTextBoyerMooreAlgorithm(textCorpus),
		NewTextTriePrefixAlgorithm(textCorpus),
		NewTextInvertedIndexAlgorithm(textCorpus),
	}

	scenarios := map[string]*ScenarioDefinition{
		scenarioLookup: {
			Name:             scenarioLookup,
			Label:            "Lookup and indexing",
			Description:      "Find profiles updated within a recent time window.",
			BusinessCase:     "Example: power an operations dashboard that asks \"which accounts changed in the last 5 minutes?\" without scanning every record on each refresh.",
			Endpoint:         "/profiles/recent",
			RequestLabel:     "Recent window seconds",
			RequestUnit:      "seconds",
			RequestHelp:      "Allowed range: 1 to 86,400 seconds.",
			RequestMin:       1,
			RequestMax:       86400,
			RequestDefault:   300,
			DefaultAlgorithm: defaultLookupAlgorithm,
			Algorithms:       algorithmMap(lookupAlgorithms),
			Order:            algorithmOrder(lookupAlgorithms),
			Verify:           sameIDsIgnoreOrder,
		},
		scenarioTopK: {
			Name:             scenarioTopK,
			Label:            "Top-K and ranking",
			Description:      "Find the highest scoring profiles without sorting the whole population.",
			BusinessCase:     "Example: retrieve the top 10 posts for a trending feed, the top products for a storefront module, or the highest-score profiles for a leaderboard.",
			Endpoint:         "/profiles/top",
			RequestLabel:     "Top K profiles",
			RequestUnit:      "profiles",
			RequestHelp:      "Allowed range: 1 to 10,000 profiles.",
			RequestMin:       1,
			RequestMax:       10000,
			RequestDefault:   100,
			DefaultAlgorithm: defaultTopKAlgorithm,
			Algorithms:       algorithmMap(topKAlgorithms),
			Order:            algorithmOrder(topKAlgorithms),
			Verify:           sameIDsSameOrder,
		},
		scenarioSorting: {
			Name:             scenarioSorting,
			Label:            "Sorting",
			Description:      "Sort a deterministic profile sample by score for exports and leaderboards.",
			BusinessCase:     "Example: produce a stable ranked page for an admin export, search result page, or leaderboard where every item must appear in exact score order.",
			Endpoint:         "/profiles/sorted",
			RequestLabel:     "Sorted result limit",
			RequestUnit:      "profiles",
			RequestHelp:      "Allowed range: 1 to 5,000 profiles.",
			RequestMin:       1,
			RequestMax:       5000,
			RequestDefault:   100,
			DefaultAlgorithm: defaultSortingAlgorithm,
			Algorithms:       algorithmMap(sortingAlgorithms),
			Order:            algorithmOrder(sortingAlgorithms),
			Verify:           sameIDsSameOrder,
		},
		scenarioCaching: {
			Name:             scenarioCaching,
			Label:            "Caching strategies",
			Description:      "Serve repeated profile requests with different eviction policies.",
			BusinessCase:     "Example: keep hot profile, post, or product detail lookups close to the app when many users repeatedly request the same small set of records.",
			Endpoint:         "/profiles/cache",
			RequestLabel:     "Hot key range",
			RequestUnit:      "profiles",
			RequestHelp:      "Allowed range: 1 to 10,000 hot profile IDs.",
			RequestMin:       1,
			RequestMax:       10000,
			RequestDefault:   100,
			DefaultAlgorithm: defaultCachingAlgorithm,
			Algorithms:       algorithmMap(cacheAlgorithms),
			Order:            algorithmOrder(cacheAlgorithms),
			Verify:           sameIDsSameOrder,
		},
		scenarioTextSearch: {
			Name:             scenarioTextSearch,
			Label:            "Text search",
			Description:      "Search display names, email prefixes, and generated profile bios.",
			BusinessCase:     "Example: back a search box that finds matching users, posts, or catalog entries as the query changes.",
			Endpoint:         "/profiles/search",
			RequestLabel:     "Search result limit",
			RequestUnit:      "profiles",
			RequestHelp:      "Allowed range: 1 to 10,000 profiles.",
			RequestMin:       1,
			RequestMax:       10000,
			RequestDefault:   100,
			QueryLabel:       "Search query",
			QueryDefault:     "platform",
			QueryHelp:        "Searches normalized name, email, region, and bio text.",
			DefaultAlgorithm: defaultTextSearchAlgorithm,
			Algorithms:       algorithmMap(textSearchAlgorithms),
			Order:            algorithmOrder(textSearchAlgorithms),
			Verify:           sameIDsIgnoreOrder,
		},
	}

	return scenarios, []string{scenarioLookup, scenarioTopK, scenarioSorting, scenarioCaching, scenarioTextSearch}
}

func algorithmMap(algorithms []ScenarioAlgorithm) map[string]ScenarioAlgorithm {
	result := make(map[string]ScenarioAlgorithm, len(algorithms))
	for _, algorithm := range algorithms {
		result[algorithm.Name()] = algorithm
	}
	return result
}

func algorithmOrder(algorithms []ScenarioAlgorithm) []string {
	order := make([]string, 0, len(algorithms))
	for _, algorithm := range algorithms {
		order = append(order, algorithm.Name())
	}
	return order
}

func scenarioInfo(def *ScenarioDefinition) ScenarioInfo {
	return ScenarioInfo{
		Name:         def.Name,
		Label:        def.Label,
		Description:  def.Description,
		BusinessCase: def.BusinessCase,
		Endpoint:     def.Endpoint,
		Request: RequestInfo{
			Label:        def.RequestLabel,
			Unit:         def.RequestUnit,
			Help:         def.RequestHelp,
			Min:          def.RequestMin,
			Max:          def.RequestMax,
			Default:      def.RequestDefault,
			QueryLabel:   def.QueryLabel,
			QueryDefault: def.QueryDefault,
			QueryHelp:    def.QueryHelp,
		},
		DefaultAlgorithm: def.DefaultAlgorithm,
		Algorithms:       algorithmInfosFor(def),
	}
}

func algorithmInfosFor(def *ScenarioDefinition) []FinderInfo {
	infos := make([]FinderInfo, 0, len(def.Order))
	for _, name := range def.Order {
		infos = append(infos, algorithmInfo(def.Algorithms[name]))
	}
	return infos
}

func algorithmInfo(algorithm ScenarioAlgorithm) FinderInfo {
	return FinderInfo{
		Name:           algorithm.Name(),
		Label:          algorithm.Label(),
		Complexity:     algorithm.Complexity(),
		Description:    algorithm.Description(),
		Code:           finderCodeFor(algorithm.Name(), languageGo),
		CodeByLanguage: finderCodesFor(algorithm.Name()),
	}
}

type lookupAlgorithm struct {
	ProfileFinder
}

func (a lookupAlgorithm) Run(request ScenarioRequest) []int {
	return a.Find(request.Since)
}

func sameIDsSameOrder(got []int, want []int) bool {
	return reflect.DeepEqual(got, want)
}

func sameIDsIgnoreOrder(got []int, want []int) bool {
	got = sortedIDs(got)
	want = sortedIDs(want)
	return reflect.DeepEqual(got, want)
}

func clampScenarioLimit(value int, def *ScenarioDefinition) int {
	if value <= 0 {
		value = def.RequestDefault
	}
	return max(def.RequestMin, min(value, def.RequestMax))
}

func sortedScoreItems(profiles []Profile) []scoreItem {
	items := scoreItems(profiles)
	sortScoreItems(items)
	return items
}

func scoreItems(profiles []Profile) []scoreItem {
	items := make([]scoreItem, len(profiles))
	for i, profile := range profiles {
		items[i] = scoreItem{ID: profile.ID, Score: profile.Score}
	}
	return items
}

func sortScoreItems(items []scoreItem) {
	sort.Slice(items, func(i, j int) bool {
		return betterScore(items[i], items[j])
	})
}

func idsFromScoreItems(items []scoreItem, limit int) []int {
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	ids := make([]int, limit)
	for i := 0; i < limit; i++ {
		ids[i] = items[i].ID
	}
	return ids
}

type scoreItem struct {
	ID    int
	Score int
}

func betterScore(a scoreItem, b scoreItem) bool {
	if a.Score == b.Score {
		return a.ID < b.ID
	}
	return a.Score > b.Score
}

func worseScore(a scoreItem, b scoreItem) bool {
	if a.Score == b.Score {
		return a.ID > b.ID
	}
	return a.Score < b.Score
}

func formatUnixNano(value time.Time) string {
	return strconv.FormatInt(value.UnixNano(), 10)
}
