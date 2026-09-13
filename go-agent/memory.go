package main

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// MemoryStats is the run-level peak memory observed while a run was in flight, sampled
// alongside it rather than derived from any single request: what matters for planning hardware
// headroom is the worst moment of a run, not its average. macOS-only (ps/vm_stat); on any other
// OS, or when the server PID is unknown, sampling is skipped and this stays nil rather than
// reporting zeros as if they were real data.
type MemoryStats struct {
	PeakServerRSSMB int64 `json:"peak_server_rss_mb,omitempty"`
	PeakWiredMB     int64 `json:"peak_wired_mb,omitempty"`
	MinFreeMB       int64 `json:"min_free_mb,omitempty"`
}

type memorySampler struct {
	stop   chan struct{}
	done   chan struct{}
	result MemoryStats
}

// startMemorySampler polls the given server process's RSS and system-wide wired/free memory
// at interval until Stop is called, tracking the peak (or, for free memory, the minimum) seen.
// Returns nil when sampling isn't possible, so callers can treat a nil sampler and a nil Stop
// result identically to "no data" rather than special-casing platform support everywhere.
func startMemorySampler(pid int, interval time.Duration) *memorySampler {
	if pid <= 0 || runtime.GOOS != "darwin" {
		return nil
	}
	s := &memorySampler{stop: make(chan struct{}), done: make(chan struct{})}
	go s.run(pid, interval)
	return s
}

func (s *memorySampler) run(pid int, interval time.Duration) {
	defer close(s.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.sampleOnce(pid)
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.sampleOnce(pid)
		}
	}
}

func (s *memorySampler) sampleOnce(pid int) {
	if rss, err := readRSSMB(pid); err == nil && rss > s.result.PeakServerRSSMB {
		s.result.PeakServerRSSMB = rss
	}
	if wired, free, err := readVMStatMB(); err == nil {
		if wired > s.result.PeakWiredMB {
			s.result.PeakWiredMB = wired
		}
		if s.result.MinFreeMB == 0 || free < s.result.MinFreeMB {
			s.result.MinFreeMB = free
		}
	}
}

// Stop ends sampling and returns the peaks observed. Safe to call at most once.
func (s *memorySampler) Stop() MemoryStats {
	close(s.stop)
	<-s.done
	return s.result
}

func readRSSMB(pid int) (int64, error) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, err
	}
	return kb / 1024, nil
}

// readVMStatMB parses `vm_stat`'s own page size rather than assuming 16384 (Apple Silicon) or
// 4096 (Intel/other) — the header states the real value for the running machine.
func readVMStatMB() (wiredMB, freeMB int64, err error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, 0, err
	}
	pageSize := int64(4096)
	var freePages, wiredPages int64
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "of" && i+1 < len(fields) {
					if v, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil {
						pageSize = v
					}
				}
			}
		}
		if strings.HasPrefix(line, "Pages free:") {
			freePages = parseVMStatValue(line)
		}
		if strings.HasPrefix(line, "Pages wired down:") {
			wiredPages = parseVMStatValue(line)
		}
	}
	return wiredPages * pageSize / (1 << 20), freePages * pageSize / (1 << 20), nil
}

func parseVMStatValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseInt(strings.TrimSuffix(fields[len(fields)-1], "."), 10, 64)
	return v
}
