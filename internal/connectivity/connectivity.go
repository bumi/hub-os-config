// Package connectivity decides whether the device has working internet.
//
// Checker performs a single probe round (primary URL, then a fallback).
// Monitor layers hysteresis on top of a sequence of probe results so that a
// device which was online tolerates transient outages for a grace window
// before being declared offline (and reverting to Setup Mode).
package connectivity

import (
	"context"
	"net/http"
	"time"
)

// Checker performs online/offline probes against two URLs.
type Checker struct {
	client      *http.Client
	primaryURL  string
	fallbackURL string
}

// New returns a Checker that probes primaryURL, falling back to fallbackURL,
// each with the given per-request timeout.
func New(primaryURL, fallbackURL string, timeout time.Duration) *Checker {
	return &Checker{
		client:      &http.Client{Timeout: timeout},
		primaryURL:  primaryURL,
		fallbackURL: fallbackURL,
	}
}

// Online reports whether either probe URL responds with a 2xx status.
func (c *Checker) Online(ctx context.Context) bool {
	if c.probe(ctx, c.primaryURL) {
		return true
	}
	return c.probe(ctx, c.fallbackURL)
}

func (c *Checker) probe(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// Monitor applies hysteresis to a stream of probe results. Until the device
// has been online at least once, a failure is reported as offline immediately.
// After that, failures are tolerated for retryWindow before being reported as
// offline.
type Monitor struct {
	retryWindow time.Duration
	now         func() time.Time

	everOnline bool
	inGrace    bool
	graceStart time.Time
}

// NewWithClock returns a Monitor using the supplied clock (for tests).
func NewWithClock(retryWindow time.Duration, now func() time.Time) *Monitor {
	return &Monitor{retryWindow: retryWindow, now: now}
}

// NewMonitor returns a Monitor using the wall clock.
func NewMonitor(retryWindow time.Duration) *Monitor {
	return NewWithClock(retryWindow, time.Now)
}

// Observe records a probe result and returns the effective connectivity state
// after applying hysteresis.
func (m *Monitor) Observe(online bool) bool {
	if online {
		m.everOnline = true
		m.inGrace = false
		return true
	}
	if !m.everOnline {
		return false
	}
	if !m.inGrace {
		m.inGrace = true
		m.graceStart = m.now()
	}
	return m.now().Sub(m.graceStart) < m.retryWindow
}
