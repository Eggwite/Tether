package utils

import (
	"sort"
	"sync"
	"time"
)

// LatencyRing is a fixed-size ring buffer that tracks duration samples and can report p99.
type LatencyRing struct {
	mu   sync.Mutex
	vals []time.Duration
	idx  int
	full bool
}

// Record adds a duration sample to the ring, allocating storage lazily.
func (r *LatencyRing) Record(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.vals == nil {
		r.vals = make([]time.Duration, 100)
	}
	r.vals[r.idx] = d
	r.idx = (r.idx + 1) % len(r.vals)
	if r.idx == 0 {
		r.full = true
	}
}

// snapshot returns a copy of the samples currently stored in the ring.
func (r *LatencyRing) snapshot() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.vals == nil {
		return nil
	}
	if r.full {
		out := make([]time.Duration, len(r.vals))
		copy(out, r.vals)
		return out
	}
	out := make([]time.Duration, r.idx)
	copy(out, r.vals[:r.idx])
	return out
}

// P99 returns the 99th percentile sample (sorted ascending) from the ring.
func (r *LatencyRing) P99() time.Duration {
	snap := r.snapshot()
	if len(snap) == 0 {
		return 0
	}
	sort.Slice(snap, func(i, j int) bool { return snap[i] < snap[j] })
	idx := int(float64(len(snap)-1) * 0.99)
	return snap[idx]
}
