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

//go:embed workers/finders.py
var pythonFinderSource string

//go:embed workers/finders.ts
var typescriptFinderSource string

var (
	finderCodeOnce sync.Once
	finderCodeMap  map[string]map[string]string
)

func finderCodeFor(name string, language string) string {
	finderCodeOnce.Do(loadFinderCode)
	return finderCodeMap[name][language]
}

func finderCodesFor(name string) map[string]string {
	finderCodeOnce.Do(loadFinderCode)

	codes := make(map[string]string, len(finderCodeMap[name]))
	for language, code := range finderCodeMap[name] {
		codes[language] = code
	}
	return codes
}

func loadFinderCode() {
	finderCodeMap = make(map[string]map[string]string)
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
	}

	for name, spec := range specsByName {
		if code := findMethodSource(spec); code != "" {
			setFinderCode(name, "go", code)
		}
		setFinderCode(name, "python", findMarkedSnippet(pythonFinderSource, name))
		setFinderCode(name, "typescript", findMarkedSnippet(typescriptFinderSource, name))
	}
}

type sourceSpec struct {
	receiver string
	method   string
	source   string
	filename string
}

func setFinderCode(name string, language string, code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	if finderCodeMap[name] == nil {
		finderCodeMap[name] = make(map[string]string)
	}
	finderCodeMap[name][language] = code
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
