package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Lab struct {
	dataset       *Dataset
	selfURL       string
	scenarios     map[string]*ScenarioDefinition
	scenarioOrder []string
	runtimes      map[string]FinderRuntime
	languages     []string

	mu              sync.RWMutex
	activeScenario  string
	activeLanguage  string
	activeAlgorithm string

	metrics *Metrics
	loadGen *LoadGenerator
}

func NewLab(dataset *Dataset, selfURL string) *Lab {
	scenarios, scenarioOrder := NewScenarioRegistry(dataset)
	goAlgorithms := make(map[string]map[string]ScenarioAlgorithm, len(scenarios))
	for name, scenario := range scenarios {
		goAlgorithms[name] = scenario.Algorithms
	}

	lab := &Lab{
		dataset:         dataset,
		selfURL:         selfURL,
		scenarios:       scenarios,
		scenarioOrder:   scenarioOrder,
		runtimes:        make(map[string]FinderRuntime, 3),
		languages:       []string{languageGo, languagePython, languageTypeScript},
		activeScenario:  scenarioLookup,
		activeLanguage:  languageGo,
		activeAlgorithm: scenarios[scenarioLookup].DefaultAlgorithm,
		metrics:         NewMetrics(),
	}
	lab.runtimes[languageGo] = NewGoRuntime(goAlgorithms)
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
	mux.HandleFunc("/profiles/top", l.handleTopProfiles)
	mux.HandleFunc("/profiles/sorted", l.handleSortedProfiles)
	mux.HandleFunc("/profiles/search", l.handleSearchProfiles)
	mux.HandleFunc("/profiles/cache", l.handleCachedProfile)
	mux.HandleFunc("/api/state", l.handleState)
	mux.HandleFunc("/api/algorithm", l.handleAlgorithm)
	mux.HandleFunc("/api/load/start", l.handleLoadStart)
	mux.HandleFunc("/api/load/rate", l.handleLoadRate)
	mux.HandleFunc("/api/load/stop", l.handleLoadStop)
	mux.HandleFunc("/api/stats", l.handleStats)
	mux.HandleFunc("/metrics", l.handlePrometheusMetrics)
}

func (l *Lab) activeRuntime() (string, string, string, FinderRuntime) {
	l.mu.RLock()
	scenario := l.activeScenario
	language := l.activeLanguage
	algorithm := l.activeAlgorithm
	runtime := l.runtimes[language]
	l.mu.RUnlock()
	return scenario, language, algorithm, runtime
}

func (l *Lab) runtimeForScenario(scenario string) (string, string, FinderRuntime, error) {
	l.mu.RLock()
	language := l.activeLanguage
	algorithm := l.activeAlgorithm
	if l.activeScenario != scenario {
		if def := l.scenarios[scenario]; def != nil {
			algorithm = def.DefaultAlgorithm
		}
	}
	runtime := l.runtimes[language]
	l.mu.RUnlock()
	if runtime == nil {
		return "", "", nil, fmt.Errorf("active runtime unavailable")
	}
	return language, algorithm, runtime, nil
}

func (l *Lab) setImplementation(values ...string) error {
	scenario := ""
	language := ""
	algorithm := ""
	switch len(values) {
	case 2:
		language = values[0]
		algorithm = values[1]
	case 3:
		scenario = values[0]
		language = values[1]
		algorithm = values[2]
	default:
		return fmt.Errorf("setImplementation expects language/algorithm or scenario/language/algorithm")
	}
	scenario = strings.TrimSpace(scenario)
	language = strings.TrimSpace(language)
	algorithm = strings.TrimSpace(algorithm)
	l.mu.Lock()
	defer l.mu.Unlock()

	if scenario == "" {
		scenario = l.activeScenario
	}
	if language == "" {
		language = l.activeLanguage
	}

	def := l.scenarios[scenario]
	if def == nil {
		return fmt.Errorf("unknown scenario %q", scenario)
	}
	if algorithm == "" {
		if scenario == l.activeScenario {
			algorithm = l.activeAlgorithm
		} else {
			algorithm = def.DefaultAlgorithm
		}
	}

	runtime := l.runtimes[language]
	if runtime == nil {
		return fmt.Errorf("unknown language %q", language)
	}
	if !runtime.Available() {
		return fmt.Errorf("%s runtime unavailable: %s", runtime.Label(), runtime.Error())
	}
	if _, ok := def.Algorithms[algorithm]; !ok {
		return fmt.Errorf("unknown algorithm %q", algorithm)
	}

	l.activeScenario = scenario
	l.activeLanguage = language
	l.activeAlgorithm = algorithm
	return nil
}

func (l *Lab) algorithmInfos() []FinderInfo {
	l.mu.RLock()
	scenario := l.activeScenario
	l.mu.RUnlock()
	return l.algorithmInfosForScenario(scenario)
}

func (l *Lab) algorithmInfosForScenario(scenario string) []FinderInfo {
	def := l.scenarios[scenario]
	if def == nil {
		return nil
	}
	return algorithmInfosFor(def)
}

func (l *Lab) scenarioInfos() []ScenarioInfo {
	infos := make([]ScenarioInfo, 0, len(l.scenarioOrder))
	for _, name := range l.scenarioOrder {
		infos = append(infos, scenarioInfo(l.scenarios[name]))
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
	request := ScenarioRequest{Since: since, IncludeIDs: includeIDs}
	l.runScenario(w, scenarioLookup, request, map[string]any{"since": since})
}

func (l *Lab) handleTopProfiles(w http.ResponseWriter, r *http.Request) {
	request, meta, err := l.parseLimitRequest(r, scenarioTopK, "k")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l.runScenario(w, scenarioTopK, request, meta)
}

func (l *Lab) handleSortedProfiles(w http.ResponseWriter, r *http.Request) {
	request, meta, err := l.parseLimitRequest(r, scenarioSorting, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l.runScenario(w, scenarioSorting, request, meta)
}

func (l *Lab) handleSearchProfiles(w http.ResponseWriter, r *http.Request) {
	request, meta, err := l.parseLimitRequest(r, scenarioTextSearch, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	def := l.scenarios[scenarioTextSearch]
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" {
		query = def.QueryDefault
	}
	request.Query = query
	meta["query"] = query
	l.runScenario(w, scenarioTextSearch, request, meta)
}

func (l *Lab) handleCachedProfile(w http.ResponseWriter, r *http.Request) {
	def := l.scenarios[scenarioCaching]
	id, err := parseIntDefault(r.URL.Query().Get("id"), 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be a whole number")
		return
	}
	id = max(1, min(id, len(l.dataset.Profiles)))
	includeIDs := parseBoolDefault(r.URL.Query().Get("ids"), true)
	request := ScenarioRequest{
		IncludeIDs: includeIDs,
		Limit:      clampScenarioLimit(id, def),
		ProfileID:  id,
	}
	l.runScenario(w, scenarioCaching, request, map[string]any{"profileId": id})
}

func (l *Lab) parseLimitRequest(r *http.Request, scenario string, key string) (ScenarioRequest, map[string]any, error) {
	def := l.scenarios[scenario]
	limit, err := parseIntDefault(r.URL.Query().Get(key), def.RequestDefault)
	if err != nil {
		return ScenarioRequest{}, nil, fmt.Errorf("%s must be a whole number", key)
	}
	limit = clampScenarioLimit(limit, def)
	includeIDs := parseBoolDefault(r.URL.Query().Get("ids"), true)
	return ScenarioRequest{Limit: limit, IncludeIDs: includeIDs}, map[string]any{key: limit}, nil
}

func (l *Lab) runScenario(w http.ResponseWriter, scenario string, request ScenarioRequest, meta map[string]any) {
	def := l.scenarios[scenario]
	if def == nil {
		writeError(w, http.StatusBadRequest, "unknown scenario")
		return
	}
	if request.Limit > 0 {
		request.Limit = clampScenarioLimit(request.Limit, def)
	}
	language, algorithm, runtime, err := l.runtimeForScenario(scenario)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := runtime.Run(scenario, algorithm, request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	implementation := implementationKey(scenario, language, algorithm)
	l.metrics.Observe(implementation, result.Elapsed)

	response := ScenarioRunResponse{
		Scenario:       scenario,
		Language:       language,
		Algorithm:      algorithm,
		Implementation: implementation,
		Count:          result.Count,
		ElapsedMicros:  result.Elapsed.Microseconds(),
		Request:        meta,
	}
	if request.IncludeIDs {
		response.IDs = result.IDs
	}
	if !request.Since.IsZero() {
		response.Since = request.Since
	}
	writeJSON(w, http.StatusOK, response)
}

type ScenarioRunResponse struct {
	Scenario       string         `json:"scenario"`
	Language       string         `json:"language"`
	Algorithm      string         `json:"algorithm"`
	Implementation string         `json:"implementation"`
	Since          time.Time      `json:"since,omitempty"`
	Count          int            `json:"count"`
	ElapsedMicros  int64          `json:"elapsedMicros"`
	IDs            []int          `json:"ids,omitempty"`
	Request        map[string]any `json:"request,omitempty"`
}

type RecentProfilesResponse = ScenarioRunResponse

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

func parseIntDefault(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func (l *Lab) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, l.stateResponse())
}

func (l *Lab) stateResponse() StateResponse {
	scenario, language, algorithm, _ := l.activeRuntime()
	return StateResponse{
		ActiveScenario:       scenario,
		ActiveLanguage:       language,
		ActiveAlgorithm:      algorithm,
		ActiveImplementation: implementationKey(scenario, language, algorithm),
		Scenarios:            l.scenarioInfos(),
		Languages:            l.languageInfos(),
		Algorithms:           l.algorithmInfos(),
		ProfileCount:         len(l.dataset.Profiles),
		GeneratedAt:          l.dataset.GeneratedAt,
		Load:                 l.loadGen.State(),
	}
}

type StateResponse struct {
	ActiveScenario       string             `json:"activeScenario"`
	ActiveLanguage       string             `json:"activeLanguage"`
	ActiveAlgorithm      string             `json:"activeAlgorithm"`
	ActiveImplementation string             `json:"activeImplementation"`
	Scenarios            []ScenarioInfo     `json:"scenarios"`
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
		Scenario string `json:"scenario"`
		Name     string `json:"name"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := l.setImplementation(req.Scenario, req.Language, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scenario, language, algorithm, _ := l.activeRuntime()
	l.metrics.MarkAlgorithmSwitch(implementationKey(scenario, language, algorithm))
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

	scenario, language, algorithm, _ := l.activeRuntime()
	implementation := implementationKey(scenario, language, algorithm)
	stats := l.metrics.Snapshot()
	stats.ActiveScenario = scenario
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

	scenario, language, algorithm, _ := l.activeRuntime()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(l.metrics.PrometheusText(implementationKey(scenario, language, algorithm), ReadRuntimeStats(), l.loadGen.State())))
}

func implementationKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func sortedIDs(ids []int) []int {
	cp := append([]int(nil), ids...)
	sort.Ints(cp)
	return cp
}
