package connectivity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOnlineTrueWhenPrimarySucceeds(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer primary.Close()

	c := New(primary.URL, "http://fallback.invalid", 2*time.Second)
	if !c.Online(context.Background()) {
		t.Fatal("expected online when primary returns 200")
	}
}

func TestOnlineFallsBackWhenPrimaryFails(t *testing.T) {
	// Primary refuses connections (started then closed).
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent) // 204, like generate_204
	}))
	defer fallback.Close()

	c := New(deadURL, fallback.URL, 2*time.Second)
	if !c.Online(context.Background()) {
		t.Fatal("expected online via fallback when primary is down")
	}
}

func TestOnlineFalseWhenBothFail(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	c := New(deadURL, "http://also.invalid.localhost", 1*time.Second)
	if c.Online(context.Background()) {
		t.Fatal("expected offline when both probes fail")
	}
}

func TestOnlineFalseWhenFallbackReturns200NotNoContent(t *testing.T) {
	// A captive portal intercepts the fallback and returns 200 (a login page)
	// instead of 204 — must be treated as offline.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 200, not 204
	}))
	defer portal.Close()

	c := New(deadURL, portal.URL, 2*time.Second)
	if c.Online(context.Background()) {
		t.Fatal("expected offline when the fallback returns 200 instead of 204")
	}
}

func TestOnlineFalseOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, srv.URL, 2*time.Second)
	if c.Online(context.Background()) {
		t.Fatal("expected offline when both probes return 5xx")
	}
}

// --- Monitor hysteresis ---

func TestMonitorNeverOnlineReportsOfflineImmediately(t *testing.T) {
	m := NewWithClock(30*time.Minute, func() time.Time { return time.Unix(0, 0) })
	if m.Observe(false) {
		t.Fatal("never-online monitor should report offline immediately")
	}
}

func TestMonitorReportsOnline(t *testing.T) {
	m := NewWithClock(30*time.Minute, func() time.Time { return time.Unix(0, 0) })
	if !m.Observe(true) {
		t.Fatal("expected online")
	}
}

func TestMonitorToleratesFailureWithinWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	m := NewWithClock(30*time.Minute, func() time.Time { return now })

	m.Observe(true) // establish online
	now = now.Add(10 * time.Minute)
	if !m.Observe(false) {
		t.Fatal("expected still-up within the 30-minute grace window")
	}
}

func TestMonitorReportsOfflineAfterWindowExceeded(t *testing.T) {
	now := time.Unix(1000, 0)
	m := NewWithClock(30*time.Minute, func() time.Time { return now })

	m.Observe(true) // online
	now = now.Add(time.Minute)
	m.Observe(false) // first failure -> grace begins
	now = now.Add(31 * time.Minute)
	if m.Observe(false) {
		t.Fatal("expected offline after grace window exceeded")
	}
}

func TestMonitorRecoveryClearsGrace(t *testing.T) {
	now := time.Unix(1000, 0)
	m := NewWithClock(30*time.Minute, func() time.Time { return now })

	m.Observe(true)
	now = now.Add(time.Minute)
	m.Observe(false) // grace begins at +1m
	now = now.Add(5 * time.Minute)
	if !m.Observe(true) {
		t.Fatal("expected online on recovery")
	}
	// Grace must have been reset: a fresh failure starts a new window.
	now = now.Add(time.Minute)
	if !m.Observe(false) {
		t.Fatal("expected still-up: grace should have reset after recovery")
	}
}
