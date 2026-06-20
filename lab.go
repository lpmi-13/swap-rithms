package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Lab struct {
	dataset *Dataset
	selfURL string
	finders map[string]ProfileFinder
	order   []string

	mu              sync.RWMutex
	activeAlgorithm string

	metrics *Metrics
	loadGen *LoadGenerator
}

func NewLab(dataset *Dataset, selfURL string) *Lab {
	finders := []ProfileFinder{
		NewSliceScanFinder(dataset.Profiles),
		NewBinarySearchFinder(dataset.ProfilesSorted),
		NewBucketedIndexFinder(dataset.Profiles, dataset.ProfileMap),
		NewMapScanFinder(dataset.ProfileMap),
		NewParallelScanFinder(dataset.Profiles),
	}

	lab := &Lab{
		dataset:         dataset,
		selfURL:         selfURL,
		finders:         make(map[string]ProfileFinder, len(finders)),
		activeAlgorithm: "slice_scan",
		metrics:         NewMetrics(),
	}
	for _, finder := range finders {
		lab.finders[finder.Name()] = finder
		lab.order = append(lab.order, finder.Name())
	}
	lab.loadGen = NewLoadGenerator(lab)
	return lab
}

func (l *Lab) Close() {
	l.loadGen.Stop()
}

func (l *Lab) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", l.handleIndex)
	mux.HandleFunc("/profiles/recent", l.handleRecentProfiles)
	mux.HandleFunc("/api/state", l.handleState)
	mux.HandleFunc("/api/algorithm", l.handleAlgorithm)
	mux.HandleFunc("/api/load/start", l.handleLoadStart)
	mux.HandleFunc("/api/load/stop", l.handleLoadStop)
	mux.HandleFunc("/api/stats", l.handleStats)
	mux.HandleFunc("/metrics", l.handlePrometheusMetrics)
}

func (l *Lab) activeFinder() ProfileFinder {
	l.mu.RLock()
	name := l.activeAlgorithm
	finder := l.finders[name]
	l.mu.RUnlock()
	return finder
}

func (l *Lab) setAlgorithm(name string) error {
	name = strings.TrimSpace(name)
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.finders[name]; !ok {
		return fmt.Errorf("unknown algorithm %q", name)
	}
	l.activeAlgorithm = name
	return nil
}

func (l *Lab) algorithmInfos() []FinderInfo {
	infos := make([]FinderInfo, 0, len(l.order))
	for _, name := range l.order {
		infos = append(infos, finderInfo(l.finders[name]))
	}
	return infos
}

func (l *Lab) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (l *Lab) handleRecentProfiles(w http.ResponseWriter, r *http.Request) {
	since, err := parseSince(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	includeIDs := parseBoolDefault(r.URL.Query().Get("ids"), true)
	finder := l.activeFinder()
	start := time.Now()
	ids := finder.Find(since)
	elapsed := time.Since(start)

	l.metrics.Observe(finder.Name(), elapsed)

	response := RecentProfilesResponse{
		Algorithm:     finder.Name(),
		Since:         since,
		Count:         len(ids),
		ElapsedMicros: elapsed.Microseconds(),
	}
	if includeIDs {
		response.IDs = ids
	}

	writeJSON(w, http.StatusOK, response)
}

type RecentProfilesResponse struct {
	Algorithm     string    `json:"algorithm"`
	Since         time.Time `json:"since"`
	Count         int       `json:"count"`
	ElapsedMicros int64     `json:"elapsedMicros"`
	IDs           []int     `json:"ids,omitempty"`
}

func parseSince(r *http.Request) (time.Time, error) {
	query := r.URL.Query()
	if window := strings.TrimSpace(query.Get("window")); window != "" {
		duration, err := time.ParseDuration(window)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid window duration")
		}
		return time.Now().UTC().Add(-duration), nil
	}

	value := strings.TrimSpace(query.Get("since"))
	if value == "" {
		return time.Now().UTC().Add(-5 * time.Minute), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("since must be an RFC3339 timestamp")
	}
	return parsed.UTC(), nil
}

func parseBoolDefault(value string, fallback bool) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "true", "1", "yes", "y":
		return true
	case "false", "0", "no", "n":
		return false
	default:
		return fallback
	}
}

func (l *Lab) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, l.stateResponse())
}

func (l *Lab) stateResponse() StateResponse {
	finder := l.activeFinder()
	return StateResponse{
		ActiveAlgorithm: finder.Name(),
		Algorithms:      l.algorithmInfos(),
		ProfileCount:    len(l.dataset.Profiles),
		GeneratedAt:     l.dataset.GeneratedAt,
		Load:            l.loadGen.State(),
	}
}

type StateResponse struct {
	ActiveAlgorithm string             `json:"activeAlgorithm"`
	Algorithms      []FinderInfo       `json:"algorithms"`
	ProfileCount    int                `json:"profileCount"`
	GeneratedAt     time.Time          `json:"generatedAt"`
	Load            LoadGeneratorState `json:"load"`
}

func (l *Lab) handleAlgorithm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := l.setAlgorithm(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l.metrics.MarkAlgorithmSwitch(req.Name)
	writeJSON(w, http.StatusOK, l.stateResponse())
}

func (l *Lab) handleLoadStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := l.loadGen.Start(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, l.loadGen.State())
}

func (l *Lab) handleLoadStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	l.loadGen.Stop()
	writeJSON(w, http.StatusOK, l.loadGen.State())
}

func (l *Lab) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	finder := l.activeFinder()
	stats := l.metrics.Snapshot()
	stats.ActiveAlgorithm = finder.Name()
	stats.Load = l.loadGen.State()
	stats.Runtime = ReadRuntimeStats()
	writeJSON(w, http.StatusOK, stats)
}

func (l *Lab) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	finder := l.activeFinder()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(l.metrics.PrometheusText(finder.Name(), ReadRuntimeStats(), l.loadGen.State())))
}

func sortedIDs(ids []int) []int {
	cp := append([]int(nil), ids...)
	sort.Ints(cp)
	return cp
}
