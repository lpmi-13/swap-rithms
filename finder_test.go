package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFindersReturnSameIDs(t *testing.T) {
	dataset := NewDataset(10_000)
	finders := testFinders(dataset)

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
	finders := testFinders(dataset)

	for _, finder := range finders {
		info := finderInfo(finder)
		if info.Code == "" {
			t.Fatalf("%s has no source code snippet", finder.Name())
		}
		if !strings.Contains(info.Code, "func (f ") || !strings.Contains(info.Code, "Find(since time.Time) []int") {
			t.Fatalf("%s snippet does not look like a finder implementation:\n%s", finder.Name(), info.Code)
		}
		for _, language := range []string{languageGo, languagePython, languageTypeScript} {
			if info.CodeByLanguage[language] == "" {
				t.Fatalf("%s has no %s source code snippet", finder.Name(), language)
			}
		}
		if info.Code != info.CodeByLanguage[languageGo] {
			t.Fatalf("%s legacy code field does not match Go snippet", finder.Name())
		}
		if !strings.Contains(info.CodeByLanguage[languagePython], "def ") {
			t.Fatalf("%s Python snippet does not look like Python:\n%s", finder.Name(), info.CodeByLanguage[languagePython])
		}
		if !strings.Contains(info.CodeByLanguage[languageTypeScript], "function ") {
			t.Fatalf("%s TypeScript snippet does not look like TypeScript:\n%s", finder.Name(), info.CodeByLanguage[languageTypeScript])
		}
	}
}

func TestWorkerRuntimesReturnSameIDs(t *testing.T) {
	dataset := NewDataset(1_000)
	goRuntime := NewGoRuntime(testFinderMap(dataset))
	runtimes := []FinderRuntime{
		NewPythonRuntime(dataset),
		NewTypeScriptRuntime(dataset),
	}
	defer func() {
		for _, runtime := range runtimes {
			_ = runtime.Close()
		}
	}()

	sinceValues := []time.Time{
		dataset.GeneratedAt.Add(-23 * time.Hour),
		dataset.GeneratedAt.Add(-5 * time.Minute),
		dataset.GeneratedAt.Add(time.Minute),
	}

	checked := 0
	for _, runtime := range runtimes {
		if !runtime.Available() {
			t.Logf("%s runtime unavailable: %s", runtime.Label(), runtime.Error())
			continue
		}
		checked++

		for _, algorithm := range []string{"slice_scan", "binary_search", "bucketed_index", "map_scan", "parallel_scan"} {
			for _, since := range sinceValues {
				want, err := goRuntime.Find(algorithm, since, true)
				if err != nil {
					t.Fatal(err)
				}
				got, err := runtime.Find(algorithm, since, true)
				if err != nil {
					t.Fatalf("%s %s returned error: %v", runtime.Name(), algorithm, err)
				}

				if got.Count != want.Count {
					t.Fatalf("%s %s returned count %d, want %d", runtime.Name(), algorithm, got.Count, want.Count)
				}
				if !reflect.DeepEqual(sortedIDs(got.IDs), sortedIDs(want.IDs)) {
					t.Fatalf("%s %s returned different ids for since %s", runtime.Name(), algorithm, since.Format(time.RFC3339))
				}

				countOnly, err := runtime.Find(algorithm, since, false)
				if err != nil {
					t.Fatalf("%s %s count-only lookup returned error: %v", runtime.Name(), algorithm, err)
				}
				if countOnly.Count != want.Count || len(countOnly.IDs) != 0 {
					t.Fatalf("%s %s count-only lookup returned count %d/%d ids", runtime.Name(), algorithm, countOnly.Count, len(countOnly.IDs))
				}
			}
		}
	}

	if checked == 0 {
		t.Skip("no external language runtimes available")
	}
}

func TestLabCanServeSelectedLanguageRuntime(t *testing.T) {
	dataset := NewDataset(1_000)
	lab := NewLab(dataset, "http://127.0.0.1:0")
	defer lab.Close()

	if runtime := lab.runtimes[languagePython]; !runtime.Available() {
		t.Skipf("Python runtime unavailable: %s", runtime.Error())
	}
	if err := lab.setImplementation(languagePython, "binary_search"); err != nil {
		t.Fatal(err)
	}

	req := mustRequest(t, "/profiles/recent?window=30m&ids=false")
	rec := httptest.NewRecorder()
	lab.handleRecentProfiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response RecentProfilesResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Language != languagePython || response.Algorithm != "binary_search" || response.Implementation != "python:binary_search" {
		t.Fatalf("unexpected implementation in response: %+v", response)
	}
	if response.ElapsedMicros <= 0 {
		t.Fatalf("expected worker-measured elapsed time, got %d", response.ElapsedMicros)
	}

	stats := lab.metrics.Snapshot()
	if stats.Algorithms["python:binary_search"].Requests != 1 {
		t.Fatalf("expected metric for python:binary_search, got %+v", stats.Algorithms)
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

func TestLoadGeneratorRunsIndefinitelyByDefault(t *testing.T) {
	generator := NewLoadGenerator(&Lab{selfURL: "http://127.0.0.1:1"})
	if err := generator.Start(LoadRequest{Rate: 1, WindowSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	defer generator.Stop()

	state := generator.State()
	if !state.Running {
		t.Fatal("expected generator to be running")
	}
	if state.DurationSeconds != 0 {
		t.Fatalf("duration = %d, want 0 for infinite run", state.DurationSeconds)
	}
	if state.StopsAt != nil {
		t.Fatalf("stopsAt = %v, want nil for infinite run", state.StopsAt)
	}
}

func TestLoadGeneratorUsesOptionalDuration(t *testing.T) {
	generator := NewLoadGenerator(&Lab{selfURL: "http://127.0.0.1:1"})
	if err := generator.Start(LoadRequest{Rate: 1, DurationSeconds: 10, WindowSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	defer generator.Stop()

	state := generator.State()
	if !state.Running {
		t.Fatal("expected generator to be running")
	}
	if state.DurationSeconds != 10 {
		t.Fatalf("duration = %d, want 10", state.DurationSeconds)
	}
	if state.StopsAt == nil {
		t.Fatal("expected stopsAt for timed run")
	}
}

func TestLoadGeneratorUpdatesRateWhileRunning(t *testing.T) {
	generator := NewLoadGenerator(&Lab{selfURL: "http://127.0.0.1:1"})
	if err := generator.Start(LoadRequest{Rate: 1, WindowSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	defer generator.Stop()

	if err := generator.UpdateRate(25); err != nil {
		t.Fatal(err)
	}
	if state := generator.State(); state.Rate != 25 {
		t.Fatalf("rate = %d, want 25", state.Rate)
	}
	if period := generator.currentPeriod(); period != 40*time.Millisecond {
		t.Fatalf("period = %s, want 40ms", period)
	}
	if err := generator.UpdateRate(0); err == nil {
		t.Fatal("expected invalid rate error")
	}
	if state := generator.State(); state.Rate != 25 {
		t.Fatalf("invalid update changed rate to %d", state.Rate)
	}
}

func TestLoadGeneratorAllowsTenThousandRequestsPerSecond(t *testing.T) {
	if err := validateLoadRate(10_000); err != nil {
		t.Fatalf("expected 10000 req/s to be valid: %v", err)
	}
	if err := validateLoadRate(10_001); err == nil {
		t.Fatal("expected rate above 10000 req/s to be invalid")
	}
}

func TestLoadRateHandlerUpdatesRunningGenerator(t *testing.T) {
	dataset := NewDataset(100)
	lab := NewLab(dataset, "http://127.0.0.1:1")
	defer lab.Close()

	if err := lab.loadGen.Start(LoadRequest{Rate: 1, WindowSeconds: 1}); err != nil {
		t.Fatal(err)
	}

	req := mustRequest(t, "/api/load/rate")
	req.Method = http.MethodPost
	req.Body = io.NopCloser(strings.NewReader(`{"rate":40}`))
	rec := httptest.NewRecorder()
	lab.handleLoadRate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if state := lab.loadGen.State(); state.Rate != 40 {
		t.Fatalf("rate = %d, want 40", state.Rate)
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

func testFinders(dataset *Dataset) []ProfileFinder {
	return []ProfileFinder{
		NewSliceScanFinder(dataset.Profiles),
		NewBinarySearchFinder(dataset.ProfilesSorted),
		NewBucketedIndexFinder(dataset.Profiles, dataset.ProfileMap),
		NewMapScanFinder(dataset.ProfileMap),
		NewParallelScanFinder(dataset.Profiles),
	}
}

func testFinderMap(dataset *Dataset) map[string]ProfileFinder {
	finders := make(map[string]ProfileFinder)
	for _, finder := range testFinders(dataset) {
		finders[finder.Name()] = finder
	}
	return finders
}
