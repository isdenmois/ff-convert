// Package pool provides a bounded worker pool used to run conversions in
// parallel.
package pool

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

// EnvConcurrency is the environment variable overriding the pool size.
const EnvConcurrency = "FF_CONVERT_CONCURRENCY"

// Pool runs jobs with at most Limit running at once.
type Pool struct {
	sem chan struct{}
	wg  sync.WaitGroup

	mu      sync.Mutex
	running int
}

// New creates a pool with the given concurrency limit (minimum 1).
func New(limit int) *Pool {
	if limit < 1 {
		limit = 1
	}
	return &Pool{sem: make(chan struct{}, limit)}
}

// DefaultLimit returns the default concurrency: $FF_CONVERT_CONCURRENCY or
// half the CPU count (minimum 1).
func DefaultLimit() int {
	if v := os.Getenv(EnvConcurrency); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	return n
}

// NewDefault creates a pool sized by DefaultLimit.
func NewDefault() *Pool { return New(DefaultLimit()) }

// Limit returns the configured concurrency limit.
func (p *Pool) Limit() int { return cap(p.sem) }

// Go submits a job. The call returns immediately; the job blocks internally
// until a slot is free. Use Wait to wait for all submitted jobs.
func (p *Pool) Go(job func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.sem <- struct{}{}
		p.mu.Lock()
		p.running++
		p.mu.Unlock()
		defer func() {
			<-p.sem
			p.mu.Lock()
			p.running--
			p.mu.Unlock()
		}()
		job()
	}()
}

// Wait blocks until all submitted jobs have finished.
func (p *Pool) Wait() { p.wg.Wait() }

// Running returns the number of currently executing jobs.
func (p *Pool) Running() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}
