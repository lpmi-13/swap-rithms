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
	if response.Scenario != scenarioLookup || response.Language != languagePython || response.Algorithm != "binary_search" || response.DataStructure != defaultDataStructure || response.Implementation != "lookup:python:binary_search:default" {
		t.Fatalf("unexpected implementation in response: %+v", response)
	}
	if response.ElapsedMicros <= 0 {
		t.Fatalf("expected worker-measured elapsed time, got %d", response.ElapsedMicros)
	}

	stats := lab.metrics.Snapshot()
	if stats.Algorithms["lookup:python:binary_search:default"].Requests != 1 {
		t.Fatalf("expected metric for lookup:python:binary_search:default, got %+v", stats.Algorithms)
	}
}

func TestScenarioMetadataIncludesAxesAndImplementations(t *testing.T) {
	dataset := NewDataset(500)
	scenarios, _ := NewScenarioRegistry(dataset)

	lookup := scenarios[scenarioLookup]
	if lookup.DefaultDataStructure != defaultDataStructure {
		t.Fatalf("lookup default data structure = %q, want %q", lookup.DefaultDataStructure, defaultDataStructure)
	}
	if findImplementation(lookup, lookup.DefaultAlgorithm, lookup.DefaultDataStructure) == nil {
		t.Fatal("lookup default implementation is not valid")
	}

	membership := scenarios[scenarioMembership]
	if membership.DefaultAlgorithm != defaultMembershipAlgorithm || membership.DefaultDataStructure != "slice" {
		t.Fatalf("unexpected membership defaults: %s/%s", membership.DefaultAlgorithm, membership.DefaultDataStructure)
	}
	if len(membership.Axes.Algorithms) != 3 || len(membership.Axes.DataStructures) != 3 {
		t.Fatalf("unexpected membership axes: %+v", membership.Axes)
	}
	if findImplementation(membership, "binary_search_contains", "sorted_slice") == nil {
		t.Fatal("expected binary_search_contains/sorted_slice implementation")
	}
	if validAxisSelection(membership, "direct_lookup", "sorted_slice") {
		t.Fatal("direct_lookup/sorted_slice should be invalid")
	}
}

func TestImplementationEndpointSwitchesDataStructure(t *testing.T) {
	dataset := NewDataset(1_000)
	lab := NewLab(dataset, "http://127.0.0.1:0")
	defer lab.Close()

	req := mustRequest(t, "/api/implementation")
	req.Method = http.MethodPost
	req.Body = io.NopCloser(strings.NewReader(`{"scenario":"membership","language":"go","algorithm":"binary_search_contains","dataStructure":"sorted_slice"}`))
	rec := httptest.NewRecorder()
	lab.handleImplementation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response StateResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.ActiveScenario != scenarioMembership || response.ActiveAlgorithm != "binary_search_contains" || response.ActiveDataStructure != "sorted_slice" {
		t.Fatalf("unexpected active implementation: %+v", response)
	}
	if response.ActiveImplementation != "membership:go:binary_search_contains:sorted_slice" {
		t.Fatalf("active implementation = %q", response.ActiveImplementation)
	}

	req = mustRequest(t, "/api/implementation")
	req.Method = http.MethodPost
	req.Body = io.NopCloser(strings.NewReader(`{"scenario":"membership","language":"go","algorithm":"direct_lookup","dataStructure":"sorted_slice"}`))
	rec = httptest.NewRecorder()
	lab.handleImplementation(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid pair status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAlgorithmEndpointUsesDefaultDataStructure(t *testing.T) {
	dataset := NewDataset(1_000)
	lab := NewLab(dataset, "http://127.0.0.1:0")
	defer lab.Close()

	req := mustRequest(t, "/api/algorithm")
	req.Method = http.MethodPost
	req.Body = io.NopCloser(strings.NewReader(`{"scenario":"membership","language":"go","name":"scan_contains"}`))
	rec := httptest.NewRecorder()
	lab.handleAlgorithm(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response StateResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.ActiveDataStructure != "slice" || response.ActiveImplementation != "membership:go:scan_contains:slice" {
		t.Fatalf("unexpected legacy wrapper selection: %+v", response)
	}
}

func TestPrometheusMetricsUseStructuredImplementationLabels(t *testing.T) {
	metrics := NewMetrics()
	metrics.Observe(scenarioMembership, languageGo, "direct_lookup", "hash_set", time.Millisecond)

	text := metrics.PrometheusText("membership:go:direct_lookup:hash_set", RuntimeStats{CPUPercent: -1}, LoadGeneratorState{})
	for _, want := range []string{
		`scenario="membership"`,
		`language="go"`,
		`algorithm="direct_lookup"`,
		`data_structure="hash_set"`,
		`implementation="membership:go:direct_lookup:hash_set"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("prometheus output missing %s:\n%s", want, text)
		}
	}
}

func TestNewLabDefersExternalRuntimeStartup(t *testing.T) {
	dataset := NewDataset(100)
	lab := NewLab(dataset, "http://127.0.0.1:0")
	defer lab.Close()

	for _, language := range []string{languagePython, languageTypeScript} {
		runtime, ok := lab.runtimes[language].(*WorkerRuntime)
		if !ok {
			t.Fatalf("%s runtime has type %T, want *WorkerRuntime", language, lab.runtimes[language])
		}

		runtime.mu.Lock()
		started := runtime.cmd != nil
		runtime.mu.Unlock()
		if started {
			t.Fatalf("%s runtime started during lab construction", language)
		}
	}
}

func TestScenarioAlgorithmsReturnEquivalentIDs(t *testing.T) {
	dataset := NewDataset(2_000)
	scenarios, order := NewScenarioRegistry(dataset)

	for _, scenarioName := range order {
		scenario := scenarios[scenarioName]
		request := testScenarioRequest(dataset, scenarioName)
		want := scenario.Implementations[scenario.ImplementationOrder[0]].Run(request)
		for _, implementationName := range scenario.ImplementationOrder[1:] {
			got := scenario.Implementations[implementationName].Run(request)
			if !scenario.Verify(got, want) {
				t.Fatalf("%s/%s returned different ids: got %v want %v", scenarioName, implementationName, got[:min(len(got), 5)], want[:min(len(want), 5)])
			}
		}
	}
}

func TestScenarioAlgorithmInfoIncludesSourceCode(t *testing.T) {
	dataset := NewDataset(100)
	scenarios, order := NewScenarioRegistry(dataset)

	for _, scenarioName := range order {
		scenario := scenarios[scenarioName]
		for _, implementationName := range scenario.ImplementationOrder {
			info := implementationInfo(scenario.Implementations[implementationName])
			if info.Code == "" {
				t.Fatalf("%s/%s has no Go source code snippet", scenarioName, implementationName)
			}
			for _, language := range []string{languageGo, languagePython, languageTypeScript} {
				if info.CodeByLanguage[language] == "" {
					t.Fatalf("%s/%s has no %s source code snippet", scenarioName, implementationName, language)
				}
			}
		}
	}
}

func TestWorkerRuntimesReturnSameScenarioResults(t *testing.T) {
	dataset := NewDataset(750)
	scenarios, order := NewScenarioRegistry(dataset)
	goImplementations := make(map[string]map[string]ScenarioImplementation, len(scenarios))
	for name, scenario := range scenarios {
		goImplementations[name] = scenario.Implementations
	}
	goRuntime := NewGoRuntime(goImplementations)
	runtimes := []FinderRuntime{
		NewPythonRuntime(dataset),
		NewTypeScriptRuntime(dataset),
	}
	defer func() {
		for _, runtime := range runtimes {
			_ = runtime.Close()
		}
	}()

	checked := 0
	for _, runtime := range runtimes {
		if !runtime.Available() {
			t.Logf("%s runtime unavailable: %s", runtime.Label(), runtime.Error())
			continue
		}
		checked++

		for _, scenarioName := range order {
			scenario := scenarios[scenarioName]
			request := testScenarioRequest(dataset, scenarioName)
			for _, implementationName := range scenario.ImplementationOrder {
				implementation := scenario.Implementations[implementationName]
				want, err := goRuntime.Run(scenarioName, implementation.Algorithm(), implementation.DataStructure(), request)
				if err != nil {
					t.Fatal(err)
				}
				got, err := runtime.Run(scenarioName, implementation.Algorithm(), implementation.DataStructure(), request)
				if err != nil {
					t.Fatalf("%s %s/%s returned error: %v", runtime.Name(), scenarioName, implementationName, err)
				}
				if got.Count != want.Count {
					t.Fatalf("%s %s/%s returned count %d, want %d", runtime.Name(), scenarioName, implementationName, got.Count, want.Count)
				}
				if !scenario.Verify(got.IDs, want.IDs) {
					t.Fatalf("%s %s/%s returned different ids: got %v want %v", runtime.Name(), scenarioName, implementationName, got.IDs[:min(len(got.IDs), 5)], want.IDs[:min(len(want.IDs), 5)])
				}
			}
		}
	}

	if checked == 0 {
		t.Skip("no external language runtimes available")
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

func TestParseSinceRejectsOutOfRangeWindow(t *testing.T) {
	for _, path := range []string{"/profiles/recent?window=0s", "/profiles/recent?window=25h"} {
		req := mustRequest(t, path)
		if _, err := parseSince(req); err == nil {
			t.Fatalf("expected %s to be rejected", path)
		}
	}
}

func TestRecentProfilesCapsReturnedIDs(t *testing.T) {
	dataset := NewDataset(maxResponseIDs + 2_000)
	lab := NewLab(dataset, "http://127.0.0.1:0")
	defer lab.Close()

	req := mustRequest(t, "/profiles/recent?window=24h&ids=true")
	rec := httptest.NewRecorder()
	lab.handleRecentProfiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response RecentProfilesResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.IDs) != maxResponseIDs {
		t.Fatalf("returned %d ids, want cap %d", len(response.IDs), maxResponseIDs)
	}
	if !response.IDsTruncated {
		t.Fatal("expected idsTruncated to be true")
	}
	if response.Count <= len(response.IDs) {
		t.Fatalf("count = %d, ids = %d; expected full count with capped ids", response.Count, len(response.IDs))
	}
}

func TestJSONHandlersRejectOversizedBodies(t *testing.T) {
	lab := NewLab(NewDataset(100), "http://127.0.0.1:0")
	defer lab.Close()

	req := mustRequest(t, "/api/load/rate")
	req.Method = http.MethodPost
	req.Body = io.NopCloser(strings.NewReader(strings.Repeat(" ", int(maxJSONBodyBytes)+1) + "{}"))
	rec := httptest.NewRecorder()
	lab.handleLoadRate(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func TestJSONHandlersRejectTrailingValues(t *testing.T) {
	lab := NewLab(NewDataset(100), "http://127.0.0.1:0")
	defer lab.Close()

	req := mustRequest(t, "/api/load/start")
	req.Method = http.MethodPost
	req.Body = io.NopCloser(strings.NewReader(`{"rate":1,"windowSeconds":1} {}`))
	rec := httptest.NewRecorder()
	lab.handleLoadStart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestWorkerRuntimesRespectIDLimit(t *testing.T) {
	dataset := NewDataset(1_000)
	runtimes := []FinderRuntime{
		NewPythonRuntime(dataset),
		NewTypeScriptRuntime(dataset),
	}
	defer func() {
		for _, runtime := range runtimes {
			_ = runtime.Close()
		}
	}()

	checked := 0
	for _, runtime := range runtimes {
		if !runtime.Available() {
			t.Logf("%s runtime unavailable: %s", runtime.Label(), runtime.Error())
			continue
		}
		checked++

		got, err := runtime.Run(scenarioLookup, "slice_scan", defaultDataStructure, ScenarioRequest{
			Since:      dataset.GeneratedAt.Add(-24 * time.Hour),
			IncludeIDs: true,
			IDLimit:    5,
		})
		if err != nil {
			t.Fatalf("%s returned error: %v", runtime.Name(), err)
		}
		if got.Count <= len(got.IDs) {
			t.Fatalf("%s count = %d, ids = %d; expected capped ids with full count", runtime.Name(), got.Count, len(got.IDs))
		}
		if len(got.IDs) != 5 {
			t.Fatalf("%s returned %d ids, want 5", runtime.Name(), len(got.IDs))
		}
	}

	if checked == 0 {
		t.Skip("no external language runtimes available")
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

func testScenarioRequest(dataset *Dataset, scenario string) ScenarioRequest {
	switch scenario {
	case scenarioLookup:
		return ScenarioRequest{
			Since:      dataset.GeneratedAt.Add(-5 * time.Minute),
			IncludeIDs: true,
		}
	case scenarioMembership:
		return ScenarioRequest{
			Limit:      200,
			IncludeIDs: true,
		}
	case scenarioTopK:
		return ScenarioRequest{
			Limit:      25,
			IncludeIDs: true,
		}
	case scenarioSorting:
		return ScenarioRequest{
			Limit:      25,
			IncludeIDs: true,
		}
	case scenarioCaching:
		return ScenarioRequest{
			ProfileID:  42,
			Limit:      100,
			IncludeIDs: true,
		}
	case scenarioTextSearch:
		return ScenarioRequest{
			Query:      "platform",
			Limit:      25,
			IncludeIDs: true,
		}
	default:
		return ScenarioRequest{IncludeIDs: true}
	}
}
