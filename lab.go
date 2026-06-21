package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Lab struct {
	dataset   *Dataset
	selfURL   string
	finders   map[string]ProfileFinder
	order     []string
	runtimes  map[string]FinderRuntime
	languages []string

	mu              sync.RWMutex
	activeLanguage  string
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
		runtimes:        make(map[string]FinderRuntime, 3),
		languages:       []string{languageGo, languagePython, languageTypeScript},
		activeLanguage:  languageGo,
		activeAlgorithm: "slice_scan",
		metrics:         NewMetrics(),
	}
	for _, finder := range finders {
		lab.finders[finder.Name()] = finder
		lab.order = append(lab.order, finder.Name())
	}
	lab.runtimes[languageGo] = NewGoRuntime(lab.finders)
	lab.runtimes[languagePython] = NewPythonRuntime(dataset)
	lab.runtimes[languageTypeScript] = NewTypeScriptRuntime(dataset)
	lab.loadGen = NewLoadGenerator(lab)
	return lab
}

func (l *Lab) Close() {
	l.loadGen.Stop()
	for _, runtime := range l.runtimes {
		if err := runtime.Close(); err != nil {
			log.Printf("close %s runtime: %v", runtime.Name(), err)
		}
	}
}

func (l *Lab) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", l.handleIndex)
	mux.HandleFunc("/profiles/recent", l.handleRecentProfiles)
	mux.HandleFunc("/api/state", l.handleState)
	mux.HandleFunc("/api/algorithm", l.handleAlgorithm)
	mux.HandleFunc("/api/load/start", l.handleLoadStart)
	mux.HandleFunc("/api/load/rate", l.handleLoadRate)
	mux.HandleFunc("/api/load/stop", l.handleLoadStop)
	mux.HandleFunc("/api/stats", l.handleStats)
	mux.HandleFunc("/metrics", l.handlePrometheusMetrics)
}

func (l *Lab) activeRuntime() (string, string, FinderRuntime) {
	l.mu.RLock()
	language := l.activeLanguage
	algorithm := l.activeAlgorithm
	runtime := l.runtimes[language]
	l.mu.RUnlock()
	return language, algorithm, runtime
}

func (l *Lab) setImplementation(language string, algorithm string) error {
	language = strings.TrimSpace(language)
	algorithm = strings.TrimSpace(algorithm)
	l.mu.Lock()
	defer l.mu.Unlock()

	if language == "" {
		language = l.activeLanguage
	}
	if algorithm == "" {
		algorithm = l.activeAlgorithm
	}

	runtime := l.runtimes[language]
	if runtime == nil {
		return fmt.Errorf("unknown language %q", language)
	}
	if !runtime.Available() {
		return fmt.Errorf("%s runtime unavailable: %s", runtime.Label(), runtime.Error())
	}
	if _, ok := l.finders[algorithm]; !ok {
		return fmt.Errorf("unknown algorithm %q", algorithm)
	}

	l.activeLanguage = language
	l.activeAlgorithm = algorithm
	return nil
}

func (l *Lab) algorithmInfos() []FinderInfo {
	infos := make([]FinderInfo, 0, len(l.order))
	for _, name := range l.order {
		infos = append(infos, finderInfo(l.finders[name]))
	}
	return infos
}

func (l *Lab) languageInfos() []LanguageInfo {
	infos := make([]LanguageInfo, 0, len(l.languages))
	for _, name := range l.languages {
		runtime := l.runtimes[name]
		infos = append(infos, LanguageInfo{
			Name:      runtime.Name(),
			Label:     runtime.Label(),
			Available: runtime.Available(),
			Error:     runtime.Error(),
		})
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
	language, algorithm, runtime := l.activeRuntime()
	if runtime == nil {
		writeError(w, http.StatusInternalServerError, "active runtime unavailable")
		return
	}

	result, err := runtime.Find(algorithm, since, includeIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	implementation := implementationKey(language, algorithm)
	l.metrics.Observe(implementation, result.Elapsed)

	response := RecentProfilesResponse{
		Language:       language,
		Algorithm:      algorithm,
		Implementation: implementation,
		Since:          since,
		Count:          result.Count,
		ElapsedMicros:  result.Elapsed.Microseconds(),
	}
	if includeIDs {
		response.IDs = result.IDs
	}

	writeJSON(w, http.StatusOK, response)
}

type RecentProfilesResponse struct {
	Language       string    `json:"language"`
	Algorithm      string    `json:"algorithm"`
	Implementation string    `json:"implementation"`
	Since          time.Time `json:"since"`
	Count          int       `json:"count"`
	ElapsedMicros  int64     `json:"elapsedMicros"`
	IDs            []int     `json:"ids,omitempty"`
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
	language, algorithm, _ := l.activeRuntime()
	return StateResponse{
		ActiveLanguage:       language,
		ActiveAlgorithm:      algorithm,
		ActiveImplementation: implementationKey(language, algorithm),
		Languages:            l.languageInfos(),
		Algorithms:           l.algorithmInfos(),
		ProfileCount:         len(l.dataset.Profiles),
		GeneratedAt:          l.dataset.GeneratedAt,
		Load:                 l.loadGen.State(),
	}
}

type StateResponse struct {
	ActiveLanguage       string             `json:"activeLanguage"`
	ActiveAlgorithm      string             `json:"activeAlgorithm"`
	ActiveImplementation string             `json:"activeImplementation"`
	Languages            []LanguageInfo     `json:"languages"`
	Algorithms           []FinderInfo       `json:"algorithms"`
	ProfileCount         int                `json:"profileCount"`
	GeneratedAt          time.Time          `json:"generatedAt"`
	Load                 LoadGeneratorState `json:"load"`
}

func (l *Lab) handleAlgorithm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := l.setImplementation(req.Language, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	language, algorithm, _ := l.activeRuntime()
	l.metrics.MarkAlgorithmSwitch(implementationKey(language, algorithm))
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

func (l *Lab) handleLoadRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Rate int `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := l.loadGen.UpdateRate(req.Rate); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, l.loadGen.State())
}

func (l *Lab) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	language, algorithm, _ := l.activeRuntime()
	implementation := implementationKey(language, algorithm)
	stats := l.metrics.Snapshot()
	stats.ActiveLanguage = language
	stats.ActiveAlgorithm = algorithm
	stats.ActiveImplementation = implementation
	stats.Load = l.loadGen.State()
	stats.Runtime = ReadRuntimeStats()
	writeJSON(w, http.StatusOK, stats)
}

func (l *Lab) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	language, algorithm, _ := l.activeRuntime()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(l.metrics.PrometheusText(implementationKey(language, algorithm), ReadRuntimeStats(), l.loadGen.State())))
}

func implementationKey(language string, algorithm string) string {
	return language + ":" + algorithm
}

func sortedIDs(ids []int) []int {
	cp := append([]int(nil), ids...)
	sort.Ints(cp)
	return cp
}
