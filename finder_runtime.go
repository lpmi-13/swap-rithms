package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	languageGo         = "go"
	languagePython     = "python"
	languageTypeScript = "typescript"
)

type LanguageInfo struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

type LookupResult struct {
	IDs     []int
	Count   int
	Elapsed time.Duration
}

type FinderRuntime interface {
	Name() string
	Label() string
	Available() bool
	Error() string
	Run(scenario string, algorithm string, dataStructure string, request ScenarioRequest) (LookupResult, error)
	Find(algorithm string, since time.Time, includeIDs bool) (LookupResult, error)
	Close() error
}

type GoRuntime struct {
	scenarios map[string]map[string]ScenarioImplementation
}

func NewGoRuntime(algorithms any) *GoRuntime {
	runtime := &GoRuntime{scenarios: make(map[string]map[string]ScenarioImplementation)}
	switch typed := algorithms.(type) {
	case map[string]map[string]ScenarioImplementation:
		runtime.scenarios = typed
	case map[string]map[string]ScenarioAlgorithm:
		for scenario, scenarioAlgorithms := range typed {
			runtime.scenarios[scenario] = make(map[string]ScenarioImplementation, len(scenarioAlgorithms))
			for name, algorithm := range scenarioAlgorithms {
				runtime.scenarios[scenario][implementationName(name, defaultDataStructure)] = algorithmImplementation{
					ScenarioAlgorithm: algorithm,
					dataStructure:     defaultDataStructure,
				}
			}
		}
	case map[string]ProfileFinder:
		runtime.scenarios[scenarioLookup] = make(map[string]ScenarioImplementation, len(typed))
		for name, finder := range typed {
			runtime.scenarios[scenarioLookup][implementationName(name, defaultDataStructure)] = algorithmImplementation{
				ScenarioAlgorithm: lookupAlgorithm{finder},
				dataStructure:     defaultDataStructure,
			}
		}
	}
	return runtime
}

func (r *GoRuntime) Name() string {
	return languageGo
}

func (r *GoRuntime) Label() string {
	return "Go"
}

func (r *GoRuntime) Available() bool {
	return true
}

func (r *GoRuntime) Error() string {
	return ""
}

func (r *GoRuntime) Run(scenario string, algorithm string, dataStructure string, request ScenarioRequest) (LookupResult, error) {
	implementations := r.scenarios[scenario]
	if implementations == nil {
		return LookupResult{}, fmt.Errorf("unknown scenario %q", scenario)
	}
	runner := implementations[implementationName(algorithm, dataStructure)]
	if runner == nil {
		return LookupResult{}, fmt.Errorf("unknown implementation %q/%q", algorithm, dataStructure)
	}

	start := time.Now()
	ids := runner.Run(request.normalized())
	elapsed := time.Since(start)

	result := LookupResult{
		Count:   len(ids),
		Elapsed: elapsed,
	}
	if request.IncludeIDs {
		result.IDs = limitIDs(ids, request.IDLimit)
	}
	return result, nil
}

func (r *GoRuntime) Find(algorithm string, since time.Time, includeIDs bool) (LookupResult, error) {
	return r.Run(scenarioLookup, algorithm, defaultDataStructure, ScenarioRequest{Since: since, IncludeIDs: includeIDs})
}

func (r *GoRuntime) Close() error {
	return nil
}

type WorkerRuntime struct {
	name                string
	label               string
	executable          string
	executablePath      string
	args                []string
	sourceName          string
	source              string
	profileCount        int
	generatedAtUnixNano string

	mu       sync.Mutex
	tempDir  string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	scanner  *bufio.Scanner
	nextID   int64
	waitOnce sync.Once
	waitErr  error

	available bool
	errText   string
}

func NewWorkerRuntime(name string, label string, executable string, args []string, sourceName string, source string, dataset *Dataset) *WorkerRuntime {
	executablePath, err := exec.LookPath(executable)
	runtime := &WorkerRuntime{
		name:                name,
		label:               label,
		executable:          executable,
		executablePath:      executablePath,
		args:                args,
		sourceName:          sourceName,
		source:              source,
		profileCount:        len(dataset.Profiles),
		generatedAtUnixNano: strconv.FormatInt(dataset.GeneratedAt.UnixNano(), 10),
	}

	if err != nil {
		runtime.available = false
		runtime.errText = err.Error()
		log.Printf("%s runtime unavailable: %v", label, err)
		return runtime
	}

	runtime.available = true
	return runtime
}

func NewPythonRuntime(dataset *Dataset) *WorkerRuntime {
	return NewWorkerRuntime(
		languagePython,
		"Python",
		"python3",
		[]string{"-u", "{script}"},
		"finders.py",
		pythonFinderSource,
		dataset,
	)
}

func NewTypeScriptRuntime(dataset *Dataset) *WorkerRuntime {
	return NewWorkerRuntime(
		languageTypeScript,
		"TypeScript",
		"node",
		[]string{"--experimental-strip-types", "{script}"},
		"finders.ts",
		typescriptFinderSource,
		dataset,
	)
}

func (r *WorkerRuntime) Name() string {
	return r.name
}

func (r *WorkerRuntime) Label() string {
	return r.label
}

func (r *WorkerRuntime) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

func (r *WorkerRuntime) Error() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.errText
}

func (r *WorkerRuntime) startLocked() error {
	if r.cmd != nil {
		return nil
	}

	tempDir, err := os.MkdirTemp("", "swap-rithms-workers-*")
	if err != nil {
		return err
	}
	r.tempDir = tempDir

	scriptPath := filepath.Join(tempDir, r.sourceName)
	if err := os.WriteFile(scriptPath, []byte(r.source), 0o600); err != nil {
		_ = r.closeLocked()
		return err
	}

	args := make([]string, len(r.args))
	for i, arg := range r.args {
		if arg == "{script}" {
			args[i] = scriptPath
			continue
		}
		args[i] = arg
	}

	cmd := exec.Command(r.executablePath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = r.closeLocked()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = r.closeLocked()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = r.closeLocked()
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = r.closeLocked()
		return err
	}

	r.cmd = cmd
	r.stdin = stdin
	r.scanner = bufio.NewScanner(stdout)
	r.scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)

	go r.logStderr(stderr)

	if err := r.sendInitLocked(); err != nil {
		_ = r.closeLocked()
		return err
	}
	return nil
}

func (r *WorkerRuntime) logStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		log.Printf("%s worker: %s", r.name, scanner.Text())
	}
}

func (r *WorkerRuntime) sendInitLocked() error {
	id := r.nextRequestID()
	if err := r.writeRequest(workerRequest{
		ID:                  id,
		Type:                "init",
		ProfileCount:        r.profileCount,
		GeneratedAtUnixNano: r.generatedAtUnixNano,
	}); err != nil {
		return err
	}

	response, err := r.readResponse()
	if err != nil {
		return err
	}
	if response.ID != id {
		return fmt.Errorf("unexpected init response id %d", response.ID)
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	return nil
}

func (r *WorkerRuntime) nextRequestID() int64 {
	r.nextID++
	return r.nextID
}

func (r *WorkerRuntime) Run(scenario string, algorithm string, dataStructure string, request ScenarioRequest) (LookupResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.available {
		return LookupResult{}, fmt.Errorf("%s runtime unavailable: %s", r.label, r.errText)
	}
	if err := r.startLocked(); err != nil {
		r.available = false
		r.errText = err.Error()
		log.Printf("%s runtime unavailable: %v", r.label, err)
		return LookupResult{}, fmt.Errorf("%s runtime unavailable: %s", r.label, r.errText)
	}

	id := r.nextRequestID()
	request = request.normalized()
	if err := r.writeRequest(workerRequest{
		ID:            id,
		Type:          "run",
		Scenario:      scenario,
		Algorithm:     algorithm,
		DataStructure: dataStructure,
		SinceUnixNano: request.SinceUnixNano,
		IncludeIDs:    request.IncludeIDs,
		IDLimit:       request.IDLimit,
		Limit:         request.Limit,
		Query:         request.Query,
		ProfileID:     request.ProfileID,
	}); err != nil {
		return LookupResult{}, err
	}

	response, err := r.readResponse()
	if err != nil {
		return LookupResult{}, err
	}
	if response.ID != id {
		return LookupResult{}, fmt.Errorf("unexpected response id %d", response.ID)
	}
	if !response.OK {
		return LookupResult{}, errors.New(response.Error)
	}

	return LookupResult{
		IDs:     response.IDs,
		Count:   response.Count,
		Elapsed: time.Duration(response.ElapsedMicros) * time.Microsecond,
	}, nil
}

func (r *WorkerRuntime) Find(algorithm string, since time.Time, includeIDs bool) (LookupResult, error) {
	return r.Run(scenarioLookup, algorithm, defaultDataStructure, ScenarioRequest{Since: since, IncludeIDs: includeIDs})
}

func (r *WorkerRuntime) writeRequest(request workerRequest) error {
	return json.NewEncoder(r.stdin).Encode(request)
}

func (r *WorkerRuntime) readResponse() (workerResponse, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return workerResponse{}, err
		}
		return workerResponse{}, io.EOF
	}

	var response workerResponse
	if err := json.Unmarshal(r.scanner.Bytes(), &response); err != nil {
		return workerResponse{}, err
	}
	return response, nil
}

func (r *WorkerRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.closeLocked()
}

func (r *WorkerRuntime) closeLocked() error {
	if r.stdin != nil {
		_ = r.writeRequest(workerRequest{Type: "shutdown"})
		_ = r.stdin.Close()
		r.stdin = nil
	}

	if r.cmd != nil {
		done := make(chan error, 1)
		go func() {
			done <- r.wait()
		}()

		select {
		case err := <-done:
			if err != nil && r.available {
				log.Printf("%s worker exited with error: %v", r.name, err)
			}
		case <-time.After(2 * time.Second):
			_ = r.cmd.Process.Kill()
			<-done
		}
		r.cmd = nil
	}

	if r.tempDir != "" {
		err := os.RemoveAll(r.tempDir)
		r.tempDir = ""
		return err
	}
	return nil
}

func (r *WorkerRuntime) wait() error {
	r.waitOnce.Do(func() {
		r.waitErr = r.cmd.Wait()
	})
	return r.waitErr
}

type workerRequest struct {
	ID                  int64  `json:"id,omitempty"`
	Type                string `json:"type"`
	ProfileCount        int    `json:"profileCount,omitempty"`
	GeneratedAtUnixNano string `json:"generatedAtUnixNano,omitempty"`
	Algorithm           string `json:"algorithm,omitempty"`
	DataStructure       string `json:"dataStructure,omitempty"`
	Scenario            string `json:"scenario,omitempty"`
	SinceUnixNano       string `json:"sinceUnixNano,omitempty"`
	IncludeIDs          bool   `json:"includeIds"`
	IDLimit             int    `json:"idLimit,omitempty"`
	Limit               int    `json:"limit,omitempty"`
	Query               string `json:"query,omitempty"`
	ProfileID           int    `json:"profileId,omitempty"`
}

type workerResponse struct {
	ID            int64  `json:"id"`
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	Count         int    `json:"count"`
	ElapsedMicros int64  `json:"elapsedMicros"`
	IDs           []int  `json:"ids,omitempty"`
}

func limitIDs(ids []int, limit int) []int {
	if limit > 0 && len(ids) > limit {
		return ids[:limit]
	}
	return ids
}
