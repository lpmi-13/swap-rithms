package main

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFindersReturnSameIDs(t *testing.T) {
	dataset := NewDataset(10_000)
	finders := []ProfileFinder{
		NewSliceScanFinder(dataset.Profiles),
		NewBinarySearchFinder(dataset.ProfilesSorted),
		NewBucketedIndexFinder(dataset.Profiles, dataset.ProfileMap),
		NewMapScanFinder(dataset.ProfileMap),
		NewParallelScanFinder(dataset.Profiles),
	}

	sinceValues := []time.Time{
		dataset.GeneratedAt.Add(-23 * time.Hour),
		dataset.GeneratedAt.Add(-5 * time.Minute),
		dataset.GeneratedAt.Add(time.Minute),
	}

	for _, since := range sinceValues {
		want := sortedIDs(finders[0].Find(since))
		for _, finder := range finders[1:] {
			got := sortedIDs(finder.Find(since))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s returned different ids for since %s: got %d ids, want %d ids", finder.Name(), since.Format(time.RFC3339), len(got), len(want))
			}
		}
	}
}

func TestFinderInfoIncludesSourceCode(t *testing.T) {
	dataset := NewDataset(100)
	finders := []ProfileFinder{
		NewSliceScanFinder(dataset.Profiles),
		NewBinarySearchFinder(dataset.ProfilesSorted),
		NewBucketedIndexFinder(dataset.Profiles, dataset.ProfileMap),
		NewMapScanFinder(dataset.ProfileMap),
		NewParallelScanFinder(dataset.Profiles),
	}

	for _, finder := range finders {
		info := finderInfo(finder)
		if info.Code == "" {
			t.Fatalf("%s has no source code snippet", finder.Name())
		}
		if !strings.Contains(info.Code, "func (f ") || !strings.Contains(info.Code, "Find(since time.Time) []int") {
			t.Fatalf("%s snippet does not look like a finder implementation:\n%s", finder.Name(), info.Code)
		}
	}
}

func TestParseSinceWindow(t *testing.T) {
	req := mustRequest(t, "/profiles/recent?window=10s")
	since, err := parseSince(req)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(since) < 9*time.Second || time.Since(since) > 11*time.Second {
		t.Fatalf("window parse produced unexpected since: %s", since)
	}
}

func mustRequest(t *testing.T, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
