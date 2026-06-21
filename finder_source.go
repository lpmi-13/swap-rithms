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
	typesByName := map[string]string{
		"slice_scan":     "*SliceScanFinder",
		"binary_search":  "*BinarySearchFinder",
		"bucketed_index": "*BucketedIndexFinder",
		"map_scan":       "*MapScanFinder",
		"parallel_scan":  "*ParallelScanFinder",
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "finder.go", finderSource, parser.ParseComments)
	if err != nil {
		return
	}

	for name, receiver := range typesByName {
		if code := findMethodSource(fset, file, receiver, "Find"); code != "" {
			setFinderCode(name, "go", code)
		}
		setFinderCode(name, "python", findMarkedSnippet(pythonFinderSource, name))
		setFinderCode(name, "typescript", findMarkedSnippet(typescriptFinderSource, name))
	}
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

func findMethodSource(fset *token.FileSet, file *ast.File, receiver string, method string) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != method || len(fn.Recv.List) == 0 {
			continue
		}
		if receiverTypeName(fn.Recv.List[0].Type) != receiver {
			continue
		}

		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		if start < 0 || end > len(finderSource) || start >= end {
			return ""
		}
		return strings.TrimSpace(finderSource[start:end])
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
	return strings.TrimSpace(source[start : start+end])
}
