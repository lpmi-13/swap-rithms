package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LoadRequest struct {
	Rate            int    `json:"rate"`
	DurationSeconds int    `json:"durationSeconds"`
	WindowSeconds   int    `json:"windowSeconds"`
	IncludeIDs      bool   `json:"includeIds"`
	Query           string `json:"query,omitempty"`
}

type LoadGeneratorState struct {
	Running         bool       `json:"running"`
	Rate            int        `json:"rate"`
	DurationSeconds int        `json:"durationSeconds"`
	WindowSeconds   int        `json:"windowSeconds"`
	IncludeIDs      bool       `json:"includeIds"`
	Query           string     `json:"query,omitempty"`
	StartedAt       time.Time  `json:"startedAt,omitempty"`
	StopsAt         *time.Time `json:"stopsAt,omitempty"`
	Completed       uint64     `json:"completed"`
	Errors          uint64     `json:"errors"`
	InFlight        int64      `json:"inFlight"`
}

type LoadGenerator struct {
	lab *Lab

	mu          sync.Mutex
	cancel      context.CancelFunc
	state       LoadGeneratorState
	rateChanged chan struct{}

	completed atomic.Uint64
	errors    atomic.Uint64
	inFlight  atomic.Int64
}

func NewLoadGenerator(lab *Lab) *LoadGenerator {
	return &LoadGenerator{
		lab:         lab,
		rateChanged: make(chan struct{}, 1),
	}
}

func (g *LoadGenerator) Start(req LoadRequest) error {
	if err := validateLoadRate(req.Rate); err != nil {
		return err
	}
	if req.DurationSeconds < 0 {
		return errors.New("duration must be zero for infinite runs or greater than zero")
	}
	if req.DurationSeconds > 600 {
		return errors.New("duration must be 600 seconds or lower")
	}
	scenario, def := g.scenarioForLoad()
	if def == nil {
		return errors.New("active scenario is unavailable")
	}
	if req.WindowSeconds <= 0 {
		req.WindowSeconds = def.RequestDefault
	}
	if req.WindowSeconds < def.RequestMin || req.WindowSeconds > def.RequestMax {
		return fmt.Errorf("%s must be between %d and %d", strings.ToLower(def.RequestLabel), def.RequestMin, def.RequestMax)
	}
	if scenario == scenarioTextSearch && strings.TrimSpace(req.Query) == "" {
		req.Query = def.QueryDefault
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cancel != nil {
		g.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now().UTC()
	g.cancel = cancel
	g.completed.Store(0)
	g.errors.Store(0)
	g.inFlight.Store(0)
	state := LoadGeneratorState{
		Running:         true,
		Rate:            req.Rate,
		DurationSeconds: req.DurationSeconds,
		WindowSeconds:   req.WindowSeconds,
		IncludeIDs:      req.IncludeIDs,
		Query:           req.Query,
		StartedAt:       now,
	}
	if req.DurationSeconds > 0 {
		stopsAt := now.Add(time.Duration(req.DurationSeconds) * time.Second)
		state.StopsAt = &stopsAt
	}
	g.state = state

	go g.run(ctx, req)
	return nil
}

func (g *LoadGenerator) UpdateRate(rate int) error {
	if err := validateLoadRate(rate); err != nil {
		return err
	}

	g.mu.Lock()
	if g.cancel == nil || !g.state.Running {
		g.mu.Unlock()
		return errors.New("load is not running")
	}
	g.state.Rate = rate
	g.mu.Unlock()

	g.notifyRateChanged()
	return nil
}

func (g *LoadGenerator) Stop() {
	g.mu.Lock()
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	g.state.Running = false
	g.state.Completed = g.completed.Load()
	g.state.Errors = g.errors.Load()
	g.state.InFlight = g.inFlight.Load()
	g.mu.Unlock()
}

func (g *LoadGenerator) State() LoadGeneratorState {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.state
	state.Completed = g.completed.Load()
	state.Errors = g.errors.Load()
	state.InFlight = g.inFlight.Load()
	if state.Running && state.StopsAt != nil && time.Now().UTC().After(*state.StopsAt) {
		state.Running = false
	}
	return state
}

func (g *LoadGenerator) run(ctx context.Context, req LoadRequest) {
	var stop <-chan time.Time
	var timer *time.Timer
	if req.DurationSeconds > 0 {
		timer = time.NewTimer(time.Duration(req.DurationSeconds) * time.Second)
		stop = timer.C
		defer timer.Stop()
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for {
		delay := time.NewTimer(g.currentPeriod())
		select {
		case <-ctx.Done():
			delay.Stop()
			return
		case <-stop:
			delay.Stop()
			g.markStopped()
			return
		case <-g.rateChanged:
			delay.Stop()
			continue
		case <-delay.C:
			g.inFlight.Add(1)
			go g.sendOne(ctx, client, req)
		}
	}
}

func (g *LoadGenerator) notifyRateChanged() {
	select {
	case g.rateChanged <- struct{}{}:
	default:
	}
}

func (g *LoadGenerator) currentPeriod() time.Duration {
	g.mu.Lock()
	rate := g.state.Rate
	g.mu.Unlock()
	return ratePeriod(rate)
}

func validateLoadRate(rate int) error {
	if rate <= 0 {
		return errors.New("rate must be greater than zero")
	}
	if rate > 10_000 {
		return errors.New("rate must be 10000 req/s or lower")
	}
	return nil
}

func ratePeriod(rate int) time.Duration {
	period := time.Second / time.Duration(rate)
	if period <= 0 {
		return time.Nanosecond
	}
	return period
}

func (g *LoadGenerator) sendOne(ctx context.Context, client *http.Client, req LoadRequest) {
	defer g.inFlight.Add(-1)

	target, err := g.loadURL(req)
	if err != nil {
		g.errors.Add(1)
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		g.errors.Add(1)
		return
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		g.errors.Add(1)
		return
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		g.errors.Add(1)
		return
	}
	g.completed.Add(1)
}

func (g *LoadGenerator) scenarioForLoad() (string, *ScenarioDefinition) {
	g.lab.mu.RLock()
	defer g.lab.mu.RUnlock()
	scenario := g.lab.activeScenario
	if scenario == "" {
		scenario = scenarioLookup
	}
	if g.lab.scenarios == nil {
		return scenario, &ScenarioDefinition{
			Name:           scenarioLookup,
			Endpoint:       "/profiles/recent",
			RequestLabel:   "Recent window seconds",
			RequestMin:     1,
			RequestMax:     86400,
			RequestDefault: 300,
		}
	}
	return scenario, g.lab.scenarios[scenario]
}

func (g *LoadGenerator) loadURL(req LoadRequest) (string, error) {
	scenario, def := g.scenarioForLoad()
	if def == nil {
		return "", errors.New("active scenario is unavailable")
	}

	parsedBase, err := url.Parse(g.lab.selfURL)
	if err != nil {
		return "", err
	}

	value := req.WindowSeconds
	if value <= 0 {
		value = def.RequestDefault
	}
	value = max(def.RequestMin, min(value, def.RequestMax))

	target := url.URL{
		Scheme: parsedBase.Scheme,
		Host:   parsedBase.Host,
		Path:   def.Endpoint,
	}
	query := target.Query()
	query.Set("ids", fmt.Sprintf("%t", req.IncludeIDs))
	switch scenario {
	case scenarioLookup:
		query.Set("window", fmt.Sprintf("%ds", value))
	case scenarioTopK:
		query.Set("k", fmt.Sprintf("%d", value))
	case scenarioSorting:
		query.Set("limit", fmt.Sprintf("%d", value))
	case scenarioCaching:
		hotRange := max(1, min(value, len(g.lab.dataset.Profiles)))
		sequence := int(g.completed.Load()+uint64(max(0, int(g.inFlight.Load())))) + 1
		id := 1 + (sequence % hotRange)
		if sequence%10 == 0 {
			id = 1 + ((sequence * 7919) % max(1, len(g.lab.dataset.Profiles)))
		}
		query.Set("id", fmt.Sprintf("%d", id))
	case scenarioTextSearch:
		query.Set("limit", fmt.Sprintf("%d", value))
		query.Set("q", strings.TrimSpace(req.Query))
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func (g *LoadGenerator) markStopped() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancel = nil
	g.state.Running = false
	g.state.Completed = g.completed.Load()
	g.state.Errors = g.errors.Load()
	g.state.InFlight = g.inFlight.Load()
}
