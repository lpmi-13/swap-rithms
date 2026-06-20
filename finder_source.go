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

var (
	finderCodeOnce sync.Once
	finderCodeMap  map[string]string
)

func finderCodeFor(name string) string {
	finderCodeOnce.Do(loadFinderCode)
	return finderCodeMap[name]
}

func loadFinderCode() {
	finderCodeMap = make(map[string]string)
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
			finderCodeMap[name] = code
		}
	}
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
