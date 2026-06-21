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
	scenarioMembership = "membership"
	scenarioTopK       = "top_k"
	scenarioSorting    = "sorting"
	scenarioCaching    = "caching"
	scenarioTextSearch = "text_search"
)

const (
	defaultLookupAlgorithm     = "slice_scan"
	defaultMembershipAlgorithm = "scan_contains"
	defaultTopKAlgorithm       = "top_k_min_heap"
	defaultSortingAlgorithm    = "sort_builtin"
	defaultCachingAlgorithm    = "cache_lru"
	defaultTextSearchAlgorithm = "text_lowercase"
)

const defaultDataStructure = "default"

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

type AxisOption struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type ScenarioAxes struct {
	Algorithms     []AxisOption `json:"algorithms"`
	DataStructures []AxisOption `json:"dataStructures"`
}

type ScenarioImplementation interface {
	Name() string
	Algorithm() string
	DataStructure() string
	Label() string
	Complexity() string
	Description() string
	Run(ScenarioRequest) []int
}

type ScenarioDefinition struct {
	Name                 string
	Label                string
	Description          string
	BusinessCase         string
	Endpoint             string
	RequestLabel         string
	RequestUnit          string
	RequestHelp          string
	RequestMin           int
	RequestMax           int
	RequestDefault       int
	QueryLabel           string
	QueryDefault         string
	QueryHelp            string
	DefaultAlgorithm     string
	DefaultDataStructure string
	Axes                 ScenarioAxes
	Implementations      map[string]ScenarioImplementation
	ImplementationOrder  []string
	Algorithms           map[string]ScenarioAlgorithm
	Order                []string
	Verify               func(got []int, want []int) bool
}

type ScenarioInfo struct {
	Name                 string               `json:"name"`
	Label                string               `json:"label"`
	Description          string               `json:"description"`
	BusinessCase         string               `json:"businessCase"`
	Endpoint             string               `json:"endpoint"`
	Request              RequestInfo          `json:"request"`
	DefaultAlgorithm     string               `json:"defaultAlgorithm"`
	DefaultDataStructure string               `json:"defaultDataStructure"`
	Axes                 ScenarioAxes         `json:"axes"`
	Implementations      []ImplementationInfo `json:"implementations"`
	Algorithms           []ImplementationInfo `json:"algorithms"`
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

	membershipImplementations := []ScenarioImplementation{
		NewScanContainsSliceImplementation(dataset.Profiles),
		NewScanContainsSortedSliceImplementation(dataset.Profiles),
		NewBinarySearchContainsSortedSliceImplementation(dataset.Profiles),
		NewDirectLookupHashSetImplementation(dataset.Profiles),
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
		scenarioMembership: {
			Name:                 scenarioMembership,
			Label:                "Membership checks",
			Description:          "Check which requested profile IDs exist in the active profile set.",
			BusinessCase:         "Example: validate imported account IDs, audience lists, or permission targets before running a bulk operation.",
			Endpoint:             "/profiles/membership",
			RequestLabel:         "Candidate ID checks",
			RequestUnit:          "IDs",
			RequestHelp:          "Allowed range: 1 to 10,000 requested IDs.",
			RequestMin:           1,
			RequestMax:           10000,
			RequestDefault:       100,
			DefaultAlgorithm:     defaultMembershipAlgorithm,
			DefaultDataStructure: "slice",
			Axes: ScenarioAxes{
				Algorithms: []AxisOption{
					{Name: "scan_contains", Label: "Scan contains", Description: "Compare each requested ID to stored IDs until a match is found."},
					{Name: "binary_search_contains", Label: "Binary search contains", Description: "Use ordering to discard half of the ID set on each step."},
					{Name: "direct_lookup", Label: "Direct lookup", Description: "Use a keyed set to test membership directly."},
				},
				DataStructures: []AxisOption{
					{Name: "slice", Label: "Slice", Description: "IDs stored in insertion order."},
					{Name: "sorted_slice", Label: "Sorted slice", Description: "IDs stored in ascending order."},
					{Name: "hash_set", Label: "Hash set", Description: "IDs stored as hash keys."},
				},
			},
			Implementations:     implementationMap(membershipImplementations),
			ImplementationOrder: implementationOrder(membershipImplementations),
			Verify:              sameIDsSameOrder,
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

	ensureScenarioDefinitions(scenarios)
	return scenarios, []string{scenarioLookup, scenarioMembership, scenarioTopK, scenarioSorting, scenarioCaching, scenarioTextSearch}
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

func implementationMap(implementations []ScenarioImplementation) map[string]ScenarioImplementation {
	result := make(map[string]ScenarioImplementation, len(implementations))
	for _, implementation := range implementations {
		result[implementationName(implementation.Algorithm(), implementation.DataStructure())] = implementation
	}
	return result
}

func implementationOrder(implementations []ScenarioImplementation) []string {
	order := make([]string, 0, len(implementations))
	for _, implementation := range implementations {
		order = append(order, implementationName(implementation.Algorithm(), implementation.DataStructure()))
	}
	return order
}

func ensureScenarioDefinitions(scenarios map[string]*ScenarioDefinition) {
	for _, def := range scenarios {
		ensureScenarioDefinition(def)
	}
}

func ensureScenarioDefinition(def *ScenarioDefinition) {
	if def.DefaultDataStructure == "" {
		def.DefaultDataStructure = defaultDataStructure
	}
	if def.Implementations == nil {
		def.Implementations = make(map[string]ScenarioImplementation, len(def.Algorithms))
		for _, name := range def.Order {
			algorithm := def.Algorithms[name]
			if algorithm == nil {
				continue
			}
			implementation := algorithmImplementation{
				ScenarioAlgorithm: algorithm,
				dataStructure:     def.DefaultDataStructure,
			}
			key := implementationName(implementation.Algorithm(), implementation.DataStructure())
			def.Implementations[key] = implementation
			def.ImplementationOrder = append(def.ImplementationOrder, key)
		}
	}
	if len(def.Axes.Algorithms) == 0 {
		def.Axes.Algorithms = algorithmAxisOptions(def)
	}
	if len(def.Axes.DataStructures) == 0 {
		def.Axes.DataStructures = []AxisOption{{
			Name:        def.DefaultDataStructure,
			Label:       dataStructureLabel(def.DefaultDataStructure),
			Description: "Default backing structure for this scenario.",
		}}
	}
}

func algorithmAxisOptions(def *ScenarioDefinition) []AxisOption {
	seen := make(map[string]bool)
	options := make([]AxisOption, 0, len(def.ImplementationOrder))
	for _, key := range def.ImplementationOrder {
		implementation := def.Implementations[key]
		if implementation == nil || seen[implementation.Algorithm()] {
			continue
		}
		seen[implementation.Algorithm()] = true
		options = append(options, AxisOption{
			Name:        implementation.Algorithm(),
			Label:       implementation.Label(),
			Description: implementation.Description(),
		})
	}
	return options
}

func dataStructureLabel(name string) string {
	if name == defaultDataStructure {
		return "Default backing structure"
	}
	return strings.ReplaceAll(name, "_", " ")
}

func scenarioInfo(def *ScenarioDefinition) ScenarioInfo {
	implementations := implementationInfosFor(def)
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
		DefaultAlgorithm:     def.DefaultAlgorithm,
		DefaultDataStructure: def.DefaultDataStructure,
		Axes:                 def.Axes,
		Implementations:      implementations,
		Algorithms:           implementations,
	}
}

func implementationInfosFor(def *ScenarioDefinition) []ImplementationInfo {
	infos := make([]ImplementationInfo, 0, len(def.ImplementationOrder))
	for _, name := range def.ImplementationOrder {
		infos = append(infos, implementationInfo(def.Implementations[name]))
	}
	return infos
}

func algorithmInfosFor(def *ScenarioDefinition) []ImplementationInfo {
	return implementationInfosFor(def)
}

func implementationInfo(implementation ScenarioImplementation) ImplementationInfo {
	return ImplementationInfo{
		Name:           implementation.Name(),
		Algorithm:      implementation.Algorithm(),
		DataStructure:  implementation.DataStructure(),
		Label:          implementation.Label(),
		Complexity:     implementation.Complexity(),
		Description:    implementation.Description(),
		Code:           implementationCodeFor(implementation.Name(), languageGo),
		CodeByLanguage: implementationCodesFor(implementation.Name()),
	}
}

func algorithmInfo(algorithm ScenarioAlgorithm) ImplementationInfo {
	return implementationInfo(algorithmImplementation{
		ScenarioAlgorithm: algorithm,
		dataStructure:     defaultDataStructure,
	})
}

type algorithmImplementation struct {
	ScenarioAlgorithm
	dataStructure string
}

func (a algorithmImplementation) Algorithm() string {
	return a.ScenarioAlgorithm.Name()
}

func (a algorithmImplementation) DataStructure() string {
	if a.dataStructure == "" {
		return defaultDataStructure
	}
	return a.dataStructure
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
