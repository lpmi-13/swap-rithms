package main

import (
	_ "embed"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
)

//go:embed finder.go
var finderSource string

//go:embed scenario_algorithms.go
var scenarioAlgorithmSource string

//go:embed membership_algorithms.go
var membershipAlgorithmSource string

//go:embed workers/finders.py
var pythonFinderSource string

//go:embed workers/finders.ts
var typescriptFinderSource string

var (
	implementationCodeOnce sync.Once
	implementationCodeMap  map[string]map[string]string
)

func implementationCodeFor(name string, language string) string {
	implementationCodeOnce.Do(loadImplementationCode)
	return implementationCodeMap[name][language]
}

func implementationCodesFor(name string) map[string]string {
	implementationCodeOnce.Do(loadImplementationCode)

	codes := make(map[string]string, len(implementationCodeMap[name]))
	for language, code := range implementationCodeMap[name] {
		codes[language] = code
	}
	return codes
}

func finderCodeFor(name string, language string) string {
	return implementationCodeFor(name, language)
}

func finderCodesFor(name string) map[string]string {
	return implementationCodesFor(name)
}

func loadImplementationCode() {
	implementationCodeMap = make(map[string]map[string]string)
	specsByName := map[string]sourceSpec{
		"slice_scan":     {receiver: "*SliceScanFinder", method: "Find", source: finderSource, filename: "finder.go"},
		"binary_search":  {receiver: "*BinarySearchFinder", method: "Find", source: finderSource, filename: "finder.go"},
		"bucketed_index": {receiver: "*BucketedIndexFinder", method: "Find", source: finderSource, filename: "finder.go"},
		"map_scan":       {receiver: "*MapScanFinder", method: "Find", source: finderSource, filename: "finder.go"},
		"parallel_scan":  {receiver: "*ParallelScanFinder", method: "Find", source: finderSource, filename: "finder.go"},

		"top_k_full_sort":   {receiver: "*TopKFullSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"top_k_min_heap":    {receiver: "*TopKMinHeapAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"top_k_quickselect": {receiver: "*TopKQuickselectAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"top_k_bucketed":    {receiver: "*TopKBucketedAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"top_k_streaming":   {receiver: "*TopKStreamingAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},

		"sort_insertion": {receiver: "*InsertionSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"sort_merge":     {receiver: "*MergeSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"sort_quick":     {receiver: "*QuickSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"sort_heap":      {receiver: "*HeapSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"sort_counting":  {receiver: "*CountingSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"sort_radix":     {receiver: "*RadixSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"sort_builtin":   {receiver: "*BuiltinSortAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},

		"cache_none":   {receiver: "*NoCacheAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"cache_fifo":   {receiver: "*FIFOCacheAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"cache_lru":    {receiver: "*LRUCacheAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"cache_lfu":    {receiver: "*LFUCacheAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"cache_random": {receiver: "*RandomCacheAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"cache_ttl":    {receiver: "*TTLCacheAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},

		"text_naive":          {receiver: "*TextNaiveAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"text_lowercase":      {receiver: "*TextLowercaseAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"text_kmp":            {receiver: "*TextKMPAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"text_boyer_moore":    {receiver: "*TextBoyerMooreAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"text_trie_prefix":    {receiver: "*TextTriePrefixAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},
		"text_inverted_index": {receiver: "*TextInvertedIndexAlgorithm", method: "Run", source: scenarioAlgorithmSource, filename: "scenario_algorithms.go"},

		"scan_contains_slice":                 {receiver: "*ScanContainsSliceImplementation", method: "Run", source: membershipAlgorithmSource, filename: "membership_algorithms.go"},
		"scan_contains_sorted_slice":          {receiver: "*ScanContainsSortedSliceImplementation", method: "Run", source: membershipAlgorithmSource, filename: "membership_algorithms.go"},
		"binary_search_contains_sorted_slice": {receiver: "*BinarySearchContainsSortedSliceImplementation", method: "Run", source: membershipAlgorithmSource, filename: "membership_algorithms.go"},
		"direct_lookup_hash_set":              {receiver: "*DirectLookupHashSetImplementation", method: "Run", source: membershipAlgorithmSource, filename: "membership_algorithms.go"},
	}

	for name, spec := range specsByName {
		if code := findMethodSource(spec); code != "" {
			setImplementationCode(name, "go", code)
		}
		setImplementationCode(name, "python", findMarkedSnippet(pythonFinderSource, name))
		setImplementationCode(name, "typescript", findMarkedSnippet(typescriptFinderSource, name))
	}
}

type sourceSpec struct {
	receiver string
	method   string
	source   string
	filename string
}

func setImplementationCode(name string, language string, code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	if implementationCodeMap[name] == nil {
		implementationCodeMap[name] = make(map[string]string)
	}
	implementationCodeMap[name][language] = code
}

func findMethodSource(spec sourceSpec) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, spec.filename, spec.source, parser.ParseComments)
	if err != nil {
		return ""
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != spec.method || len(fn.Recv.List) == 0 {
			continue
		}
		if receiverTypeName(fn.Recv.List[0].Type) != spec.receiver {
			continue
		}

		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		if start < 0 || end > len(spec.source) || start >= end {
			return ""
		}
		return strings.TrimSpace(spec.source[start:end])
	}
	return ""
}

func receiverTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return "*" + receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

func findMarkedSnippet(source string, name string) string {
	startMarker := "snippet:" + name + ":start"
	endMarker := "snippet:" + name + ":end"

	start := strings.Index(source, startMarker)
	if start < 0 {
		return ""
	}
	startLineEnd := strings.IndexByte(source[start:], '\n')
	if startLineEnd < 0 {
		return ""
	}
	start += startLineEnd + 1

	end := strings.Index(source[start:], endMarker)
	if end < 0 {
		return ""
	}
	snippet := strings.TrimRight(source[start:start+end], " \t")
	for _, suffix := range []string{"\n#", "\n//"} {
		if strings.HasSuffix(snippet, suffix) {
			snippet = strings.TrimRight(snippet[:len(snippet)-len(suffix)], " \t")
			break
		}
	}
	return strings.TrimSpace(snippet)
}
