package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RuntimeStats struct {
	Goroutines int     `json:"goroutines"`
	HeapBytes  uint64  `json:"heapBytes"`
	HeapMB     float64 `json:"heapMb"`
	AllocBytes uint64  `json:"allocBytes"`
	CPUPercent float64 `json:"cpuPercent"`
}

var cpuSampler = &processCPUSampler{lastWall: time.Now(), lastCPUJiffies: -1}

func ReadRuntimeStats() RuntimeStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return RuntimeStats{
		Goroutines: runtime.NumGoroutine(),
		HeapBytes:  mem.HeapInuse,
		HeapMB:     float64(mem.HeapInuse) / 1024 / 1024,
		AllocBytes: mem.Alloc,
		CPUPercent: cpuSampler.Sample(),
	}
}

type processCPUSampler struct {
	mu             sync.Mutex
	lastWall       time.Time
	lastCPUJiffies int64
}

func (s *processCPUSampler) Sample() float64 {
	now := time.Now()
	jiffies, ok := readProcessJiffies()
	if !ok {
		return -1
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastCPUJiffies < 0 {
		s.lastCPUJiffies = jiffies
		s.lastWall = now
		return 0
	}

	wall := now.Sub(s.lastWall).Seconds()
	cpu := float64(jiffies-s.lastCPUJiffies) / 100.0
	s.lastCPUJiffies = jiffies
	s.lastWall = now

	if wall <= 0 {
		return 0
	}
	return cpu / wall * 100
}

func readProcessJiffies() (int64, bool) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}

	text := string(data)
	endComm := strings.LastIndex(text, ")")
	if endComm < 0 || endComm+2 >= len(text) {
		return 0, false
	}
	fields := strings.Fields(text[endComm+2:])
	if len(fields) < 15 {
		return 0, false
	}

	utime, err := strconv.ParseInt(fields[11], 10, 64)
	if err != nil {
		return 0, false
	}
	stime, err := strconv.ParseInt(fields[12], 10, 64)
	if err != nil {
		return 0, false
	}

	return utime + stime, true
}
