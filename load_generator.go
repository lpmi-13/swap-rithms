package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type LoadRequest struct {
	Rate            int  `json:"rate"`
	DurationSeconds int  `json:"durationSeconds"`
	WindowSeconds   int  `json:"windowSeconds"`
	IncludeIDs      bool `json:"includeIds"`
}

type LoadGeneratorState struct {
	Running         bool       `json:"running"`
	Rate            int        `json:"rate"`
	DurationSeconds int        `json:"durationSeconds"`
	WindowSeconds   int        `json:"windowSeconds"`
	IncludeIDs      bool       `json:"includeIds"`
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
	if req.WindowSeconds <= 0 {
		req.WindowSeconds = 300
	}
	if req.WindowSeconds > 24*60*60 {
		return errors.New("window must be 24 hours or lower")
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

	target := url.URL{
		Path: "/profiles/recent",
	}
	parsedBase, err := url.Parse(g.lab.selfURL)
	if err != nil {
		g.errors.Add(1)
		return
	}
	target.Scheme = parsedBase.Scheme
	target.Host = parsedBase.Host
	query := target.Query()
	query.Set("window", fmt.Sprintf("%ds", req.WindowSeconds))
	query.Set("ids", fmt.Sprintf("%t", req.IncludeIDs))
	target.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
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

func (g *LoadGenerator) markStopped() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancel = nil
	g.state.Running = false
	g.state.Completed = g.completed.Load()
	g.state.Errors = g.errors.Load()
	g.state.InFlight = g.inFlight.Load()
}
