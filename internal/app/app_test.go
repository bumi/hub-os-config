package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/getAlby/hub-os-config/internal/config"
	"github.com/getAlby/hub-os-config/internal/connectivity"
	"github.com/getAlby/hub-os-config/internal/netmgr"
	"github.com/getAlby/hub-os-config/internal/web"
)

// --- fakes ---

type appNM struct {
	configured   bool
	ssid         string
	connectErr   error
	deleteCalled bool
	stopCalled   bool
}

func (f *appNM) ScanNetworks(context.Context) ([]netmgr.Network, error) { return nil, nil }
func (f *appNM) IsWiFiConfigured(context.Context) (bool, error)         { return f.configured, nil }
func (f *appNM) SaveAndConnect(context.Context, string, string) error   { return nil }
func (f *appNM) Connect(context.Context, string) error                  { return f.connectErr }
func (f *appNM) DeleteConnection(context.Context, string) error         { f.deleteCalled = true; return nil }
func (f *appNM) StartHotspot(context.Context) error                     { return nil }
func (f *appNM) StopHotspot(context.Context) error                      { f.stopCalled = true; return nil }
func (f *appNM) CurrentSSID(context.Context) (string, error)            { return f.ssid, nil }

type fakeEnv struct{ applied map[string]string }

func (f *fakeEnv) Get() (map[string]string, error) { return nil, nil }
func (f *fakeEnv) Apply(u map[string]string) error { f.applied = u; return nil }

type fakeReboot struct{ called bool }

func (f *fakeReboot) Reboot() error { f.called = true; return nil }

func newApplyApp(nm *appNM) (*App, *fakeEnv, *fakeReboot, *fakeController, *recordingState) {
	env := &fakeEnv{}
	rb := &fakeReboot{}
	ctrl := &fakeController{}
	rs := &recordingState{}
	mon := connectivity.NewWithClock(30*time.Minute, clockAt(time.Unix(0, 0)))
	a := New(Deps{
		NM: nm, Prober: &fakeProber{online: true}, Monitor: mon,
		Controller: ctrl, State: rs, Env: env, Reboot: rb,
	})
	return a, env, rb, ctrl, rs
}

type fakeController struct {
	setupCalls  int
	normalCalls int
}

func (f *fakeController) EnterSetup(context.Context) error  { f.setupCalls++; return nil }
func (f *fakeController) EnterNormal(context.Context) error { f.normalCalls++; return nil }

type fakeProber struct{ online bool }

func (f *fakeProber) Online(context.Context) bool { return f.online }

func newApp(nm netmgr.Manager, prober Prober, ctrl Controller, mon *connectivity.Monitor) (*App, *recordingState) {
	rs := &recordingState{}
	a := New(Deps{
		NM:                nm,
		Prober:            prober,
		Monitor:           mon,
		Controller:        ctrl,
		State:             rs,
		BootProbeAttempts: 1,
		BootProbeDelay:    0,
	})
	return a, rs
}

type recordingState struct {
	lastSavedMode string
	lastAttempt   *config.Attempt
	saves         int
}

func (r *recordingState) Load() (config.State, error) { return config.State{}, nil }
func (r *recordingState) Save(s config.State) error {
	r.saves++
	r.lastSavedMode = s.LastMode
	r.lastAttempt = s.LastAttempt
	return nil
}

func clockAt(t time.Time) func() time.Time { return func() time.Time { return t } }

// --- decideInitialMode ---

func TestDecideInitialMode(t *testing.T) {
	cases := []struct {
		configured, online bool
		want               Mode
	}{
		{false, false, ModeSetup},
		{false, true, ModeSetup},
		{true, false, ModeSetup},
		{true, true, ModeNormal},
	}
	for _, c := range cases {
		if got := decideInitialMode(c.configured, c.online); got != c.want {
			t.Errorf("decideInitialMode(%v,%v) = %v; want %v", c.configured, c.online, got, c.want)
		}
	}
}

// --- Boot ---

func TestBootNoWiFiEntersSetup(t *testing.T) {
	ctrl := &fakeController{}
	mon := connectivity.NewWithClock(30*time.Minute, clockAt(time.Unix(0, 0)))
	a, rs := newApp(&appNM{configured: false}, &fakeProber{online: false}, ctrl, mon)

	mode, err := a.Boot(context.Background())
	if err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if mode != ModeSetup || ctrl.setupCalls != 1 || ctrl.normalCalls != 0 {
		t.Fatalf("expected Setup entry; mode=%v setup=%d normal=%d", mode, ctrl.setupCalls, ctrl.normalCalls)
	}
	if rs.lastSavedMode != "setup" {
		t.Errorf("state mode = %q; want setup", rs.lastSavedMode)
	}
	if rs.lastAttempt != nil {
		t.Errorf("fresh setup should record no failed attempt, got %+v", rs.lastAttempt)
	}
}

func TestBootConfiguredAndOnlineEntersNormal(t *testing.T) {
	ctrl := &fakeController{}
	mon := connectivity.NewWithClock(30*time.Minute, clockAt(time.Unix(0, 0)))
	a, rs := newApp(&appNM{configured: true, ssid: "HomeWiFi"}, &fakeProber{online: true}, ctrl, mon)

	mode, err := a.Boot(context.Background())
	if err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if mode != ModeNormal || ctrl.normalCalls != 1 {
		t.Fatalf("expected Normal entry; mode=%v normal=%d", mode, ctrl.normalCalls)
	}
	if rs.lastSavedMode != "normal" || rs.lastAttempt != nil {
		t.Errorf("expected normal mode and no failed attempt; got mode=%q attempt=%+v", rs.lastSavedMode, rs.lastAttempt)
	}
}

func TestBootConfiguredButOfflineEntersSetupWithFailure(t *testing.T) {
	ctrl := &fakeController{}
	mon := connectivity.NewWithClock(30*time.Minute, clockAt(time.Unix(0, 0)))
	a, rs := newApp(&appNM{configured: true, ssid: "HomeWiFi"}, &fakeProber{online: false}, ctrl, mon)

	mode, err := a.Boot(context.Background())
	if err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if mode != ModeSetup || ctrl.setupCalls != 1 {
		t.Fatalf("expected Setup entry; mode=%v setup=%d", mode, ctrl.setupCalls)
	}
	if rs.lastAttempt == nil || rs.lastAttempt.Result != "failed" || rs.lastAttempt.SSID != "HomeWiFi" {
		t.Errorf("expected recorded failure for HomeWiFi, got %+v", rs.lastAttempt)
	}
}

// --- supervise tick ---

func TestSuperviseTickRevertsToSetupAfterGraceExceeded(t *testing.T) {
	now := time.Unix(1000, 0)
	mon := connectivity.NewWithClock(30*time.Minute, func() time.Time { return now })
	ctrl := &fakeController{}
	a, _ := newApp(&appNM{configured: true, ssid: "HomeWiFi"}, &fakeProber{online: false}, ctrl, mon)

	a.set(ModeNormal, false, nil)
	mon.Observe(true) // establish online
	now = now.Add(time.Minute)
	a.superviseTick(context.Background()) // first failure -> grace
	now = now.Add(31 * time.Minute)
	a.superviseTick(context.Background()) // grace exceeded -> Setup

	if a.Mode() != ModeSetup {
		t.Fatalf("expected revert to Setup, mode = %v", a.Mode())
	}
	if ctrl.setupCalls != 1 {
		t.Errorf("expected exactly one EnterSetup, got %d", ctrl.setupCalls)
	}
}

func TestSuperviseTickStaysNormalWithinGrace(t *testing.T) {
	now := time.Unix(1000, 0)
	mon := connectivity.NewWithClock(30*time.Minute, func() time.Time { return now })
	ctrl := &fakeController{}
	a, _ := newApp(&appNM{configured: true}, &fakeProber{online: false}, ctrl, mon)

	a.set(ModeNormal, false, nil)
	mon.Observe(true)
	now = now.Add(5 * time.Minute)
	a.superviseTick(context.Background())

	if a.Mode() != ModeNormal {
		t.Fatalf("expected to stay Normal within grace, mode = %v", a.Mode())
	}
	if ctrl.setupCalls != 0 {
		t.Errorf("should not enter Setup within grace, got %d calls", ctrl.setupCalls)
	}
}

func TestSuperviseTickInSetupIsNoOp(t *testing.T) {
	mon := connectivity.NewWithClock(30*time.Minute, clockAt(time.Unix(0, 0)))
	ctrl := &fakeController{}
	a, _ := newApp(&appNM{}, &fakeProber{online: false}, ctrl, mon)

	a.set(ModeSetup, false, nil)
	a.superviseTick(context.Background())

	if a.Mode() != ModeSetup || ctrl.setupCalls != 0 || ctrl.normalCalls != 0 {
		t.Errorf("Setup-mode tick should be a no-op; mode=%v setup=%d normal=%d",
			a.Mode(), ctrl.setupCalls, ctrl.normalCalls)
	}
}

// --- test-before-commit (Save / testAndApply) ---

func wifiReq(ssid, pw string, adv map[string]string) web.SaveRequest {
	return web.SaveRequest{WiFi: &web.WiFiCreds{SSID: ssid, Password: pw}, Advanced: adv}
}

func TestApplyWiFiSuccessGoesLiveWithoutReboot(t *testing.T) {
	nm := &appNM{ssid: "HomeWiFi"} // connectErr nil -> association succeeds
	a, env, rb, ctrl, rs := newApplyApp(nm)

	a.testAndApply(context.Background(), wifiReq("HomeWiFi", "correctpass", nil))

	if a.Mode() != ModeNormal {
		t.Fatalf("mode = %v; want normal", a.Mode())
	}
	if !nm.stopCalled {
		t.Error("expected the AP to be dropped to test as a station")
	}
	if ctrl.normalCalls != 1 {
		t.Errorf("expected to go live (EnterNormal once), got %d", ctrl.normalCalls)
	}
	if rb.called {
		t.Error("must not reboot when only WiFi changed")
	}
	if env.applied != nil {
		t.Error("no advanced settings to apply")
	}
	if rs.lastAttempt != nil {
		t.Errorf("success should clear last attempt, got %+v", rs.lastAttempt)
	}
}

func TestApplyWiFiSuccessWithAdvancedReboots(t *testing.T) {
	nm := &appNM{ssid: "HomeWiFi"}
	a, env, rb, _, _ := newApplyApp(nm)

	a.testAndApply(context.Background(), wifiReq("HomeWiFi", "correctpass", map[string]string{"LN_BACKEND_TYPE": "BARK"}))

	if env.applied["LN_BACKEND_TYPE"] != "BARK" {
		t.Errorf("advanced not applied on success: %+v", env.applied)
	}
	if !rb.called {
		t.Error("expected a reboot when advanced settings changed")
	}
}

func TestApplyWiFiWrongPasswordRestoresAPAndRecordsFailure(t *testing.T) {
	nm := &appNM{ssid: "HomeWiFi", connectErr: errors.New("802.1X authentication failed")}
	a, env, rb, ctrl, rs := newApplyApp(nm)

	a.testAndApply(context.Background(), wifiReq("HomeWiFi", "wrongpass", map[string]string{"LN_BACKEND_TYPE": "BARK"}))

	if a.Mode() != ModeSetup {
		t.Fatalf("mode = %v; want setup (restored)", a.Mode())
	}
	if ctrl.setupCalls != 1 {
		t.Errorf("expected the AP/portal to be restored (EnterSetup once), got %d", ctrl.setupCalls)
	}
	if !nm.deleteCalled {
		t.Error("expected the bad profile to be deleted")
	}
	if rb.called {
		t.Error("must not reboot on a failed password")
	}
	if env.applied != nil {
		t.Error("advanced settings must not be applied when the test fails")
	}
	if rs.lastAttempt == nil || rs.lastAttempt.Result != "failed" || rs.lastAttempt.SSID != "HomeWiFi" {
		t.Errorf("expected a recorded failure for HomeWiFi, got %+v", rs.lastAttempt)
	}
}
