package app

import (
	"context"
	"time"

	"github.com/getAlby/hub-os-config/internal/config"
	"github.com/getAlby/hub-os-config/internal/web"
)

// Save applies a validated configuration change. WiFi changes are verified
// before being committed (see testAndApply), so the call returns immediately:
// testing the password requires dropping the AP — and with it the captive
// portal — so the outcome is reported via status, not this call.
func (a *App) Save(req web.SaveRequest) {
	if req.WiFi != nil {
		a.clearAttempt() // mark this attempt pending so the UI ignores a prior failure
		go func() {
			if a.deps.FlushDelay > 0 {
				time.Sleep(a.deps.FlushDelay) // let the HTTP response flush
			}
			ctx, cancel := context.WithTimeout(context.Background(), a.deps.ConnectTimeout)
			defer cancel()
			a.testAndApply(ctx, req)
		}()
		return
	}
	// Advanced-only change: apply and reboot so Alby Hub re-reads its .env.
	if len(req.Advanced) > 0 {
		_ = a.deps.Env.Apply(req.Advanced)
	}
	_ = a.deps.Reboot.Reboot()
}

// testAndApply verifies the WiFi credentials before committing, then reboots.
// ConnectWiFi created an autoconnecting profile, so on the next boot
// NetworkManager reconnects to it on its own and Alby Hub re-reads any changed
// .env. On failure it discards the profile, restores the setup AP, and records
// the reason so the portal can prompt for the password again.
func (a *App) testAndApply(ctx context.Context, req web.SaveRequest) {
	ssid := req.WiFi.SSID
	_ = a.deps.NM.StopHotspot(ctx) // free the single radio to connect as a station
	if err := a.deps.NM.ConnectWiFi(ctx, ssid, req.WiFi.Password, req.WiFi.Hidden); err != nil {
		_ = a.deps.NM.DeleteConnection(ctx, ssid)
		a.failWiFi(ctx, ssid, "incorrect password or could not connect")
		return
	}

	// Password verified — commit and reboot for a clean start on the new network.
	if len(req.Advanced) > 0 {
		_ = a.deps.Env.Apply(req.Advanced)
	}
	a.set(ModeNormal, true, nil)
	a.persist()
	_ = a.deps.Reboot.Reboot()
}

func (a *App) failWiFi(ctx context.Context, ssid, reason string) {
	a.set(ModeSetup, false, &config.Attempt{SSID: ssid, Result: "failed", Reason: reason})
	a.persist()
	_ = a.deps.Controller.EnterSetup(ctx) // bring the AP + portal back up
}
