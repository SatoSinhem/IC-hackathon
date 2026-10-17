package auth

import (
	"sync"
	"time"
)

const maxLimiterBuckets = 10000

type bucket struct {
	count int
	start time.Time
}

// limiter is a fixed-window failure counter kept in memory, so it is per API instance and resets on restart.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
	window  time.Duration
	now     func() time.Time
}

func newLimiter(window time.Duration, now func() time.Time) *limiter {
	return &limiter{buckets: make(map[string]bucket), window: window, now: now}
}

func (l *limiter) blocked(key string, limit int) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		return 0, false
	}
	now := l.now()
	if now.Sub(b.start) >= l.window {
		delete(l.buckets, key)
		return 0, false
	}
	if b.count >= limit {
		return b.start.Add(l.window).Sub(now), true
	}
	return 0, false
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		if !ok && len(l.buckets) >= maxLimiterBuckets {
			l.evict(now)
		}
		l.buckets[key] = bucket{count: 1, start: now}
		return
	}
	b.count++
	l.buckets[key] = b
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// evict drops expired buckets and, if the map is still full, the oldest one, so a flood of fake keys cannot disable limiting.
func (l *limiter) evict(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for k, b := range l.buckets {
		if now.Sub(b.start) >= l.window {
			delete(l.buckets, k)
			continue
		}
		if oldestKey == "" || b.start.Before(oldest) {
			oldestKey, oldest = k, b.start
		}
	}
	if len(l.buckets) >= maxLimiterBuckets && oldestKey != "" {
		delete(l.buckets, oldestKey)
	}
}
