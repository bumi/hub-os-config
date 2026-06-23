// Package app is the orchestration core: a two-mode state machine
// (Setup/Normal) with a supervisor loop. It decides which mode the device
// should be in based on whether WiFi is configured and reachable, and drives a
// Controller to perform the side effects (hotspot + web server). The decision
// logic here is unit-tested with fakes; the host-touching Controller lives in
// controller.go.
package app

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/getAlby/hub-os-config/internal/config"
	"github.com/getAlby/hub-os-config/internal/connectivity"
	"github.com/getAlby/hub-os-config/internal/netmgr"
	"github.com/getAlby/hub-os-config/internal/web"
)

// Mode is the device's operating mode.
type Mode string

const (
	ModeSetup  Mode = "setup"
	ModeNormal Mode = "normal"
)

// Prober reports raw (pre-hysteresis) internet reachability.
type Prober interface {
	Online(ctx context.Context) bool
}

// Controller performs the side effects of entering a mode: bringing the AP and
// web server up or down on the right interface/port.
type Controller interface {
	EnterSetup(ctx context.Context) error
	EnterNormal(ctx context.Context) error
}

// StateStore writes a record of runtime state (for observability/debugging).
type StateStore interface {
	Save(config.State) error
}

// Deps are the App's collaborators.
type Deps struct {
	NM         netmgr.Manager
	Prober     Prober
	Monitor    *connectivity.Monitor
	Controller Controller
	State      StateStore
	Env        web.EnvStore // writes advanced Alby Hub .env settings
	Reboot     web.Rebooter

	// BootProbeAttempts/Delay bound how long Boot waits for connectivity before
	// concluding the device is offline (covers NM association + brief WAN settle).
	BootProbeAttempts int
	BootProbeDelay    time.Duration

	// ConnectTimeout bounds a WiFi credential test; FlushDelay lets the HTTP
	// response reach the client before the AP drops for the test.
	ConnectTimeout time.Duration
	FlushDelay     time.Duration
}

// App is the state machine.
type App struct {
	deps Deps

	mu          sync.Mutex
	mode        Mode
	online      bool
	lastAttempt *config.Attempt
}

// New builds an App, applying defaults for unset boot-probe settings.
func New(d Deps) *App {
	if d.BootProbeAttempts <= 0 {
		d.BootProbeAttempts = 6
	}
	if d.BootProbeDelay <= 0 {
		d.BootProbeDelay = 10 * time.Second
	}
	if d.ConnectTimeout <= 0 {
		d.ConnectTimeout = 45 * time.Second
	}
	if d.FlushDelay <= 0 {
		d.FlushDelay = time.Second
	}
	return &App{deps: d}
}

// decideInitialMode is the pure boot decision: Normal only when WiFi is both
// configured and reachable.
func decideInitialMode(configured, online bool) Mode {
	if configured && online {
		return ModeNormal
	}
	return ModeSetup
}

// Boot determines the initial mode and enters it.
func (a *App) Boot(ctx context.Context) (Mode, error) {
	configured, err := a.deps.NM.IsWiFiConfigured(ctx)
	if err != nil {
		// Don't leave the device without an AP if NetworkManager is briefly
		// unavailable at boot; assume nothing is configured and enter Setup.
		log.Printf("checking WiFi configuration failed, assuming none: %v", err)
		configured = false
	}

	online := false
	if configured {
		online = a.waitForConnectivity(ctx)
	}

	switch decideInitialMode(configured, online) {
	case ModeNormal:
		return ModeNormal, a.enterNormal(ctx)
	default:
		var attempt *config.Attempt
		if configured && !online {
			ssid, _ := a.deps.NM.CurrentSSID(ctx)
			attempt = &config.Attempt{SSID: ssid, Result: "failed", Reason: "could not reach the internet"}
		}
		return ModeSetup, a.enterSetup(ctx, attempt)
	}
}

// waitForConnectivity probes up to BootProbeAttempts times, routing results
// through the Monitor so a successful probe marks the device as having been
// online (enabling the runtime grace window later).
func (a *App) waitForConnectivity(ctx context.Context) bool {
	for i := 0; i < a.deps.BootProbeAttempts; i++ {
		if a.deps.Monitor.Observe(a.deps.Prober.Online(ctx)) {
			return true
		}
		if i < a.deps.BootProbeAttempts-1 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(a.deps.BootProbeDelay):
			}
		}
	}
	return false
}

// Supervise runs the monitoring loop until ctx is cancelled.
func (a *App) Supervise(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.superviseTick(ctx)
		}
	}
}

// superviseTick performs one monitoring step. Only Normal Mode is actively
// monitored; a sustained connectivity loss (past the grace window) reverts the
// device to Setup Mode live.
func (a *App) superviseTick(ctx context.Context) {
	if a.Mode() != ModeNormal {
		return
	}
	effective := a.deps.Monitor.Observe(a.deps.Prober.Online(ctx))
	a.setOnline(effective)
	if !effective {
		ssid, _ := a.deps.NM.CurrentSSID(ctx)
		_ = a.enterSetup(ctx, &config.Attempt{SSID: ssid, Result: "failed", Reason: "internet connectivity lost"})
	}
}

func (a *App) enterNormal(ctx context.Context) error {
	if err := a.deps.Controller.EnterNormal(ctx); err != nil {
		return err
	}
	a.set(ModeNormal, true, nil)
	a.persist()
	return nil
}

func (a *App) enterSetup(ctx context.Context, attempt *config.Attempt) error {
	if err := a.deps.Controller.EnterSetup(ctx); err != nil {
		return err
	}
	a.set(ModeSetup, false, attempt)
	a.persist()
	return nil
}

func (a *App) persist() {
	a.mu.Lock()
	st := config.State{LastMode: string(a.mode), LastAttempt: a.lastAttempt}
	a.mu.Unlock()
	_ = a.deps.State.Save(st)
}

// --- guarded state accessors ---

func (a *App) set(mode Mode, online bool, attempt *config.Attempt) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mode = mode
	a.online = online
	a.lastAttempt = attempt
}

func (a *App) setOnline(online bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.online = online
}

// clearAttempt marks a fresh attempt as in progress by discarding any prior
// failure, so the UI never mistakes a previous attempt's result for this one's.
func (a *App) clearAttempt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAttempt = nil
}

// Mode returns the current mode.
func (a *App) Mode() Mode {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mode
}
