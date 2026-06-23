package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/getAlby/hub-os-config/internal/app"
	"github.com/getAlby/hub-os-config/internal/captive"
	"github.com/getAlby/hub-os-config/internal/config"
	"github.com/getAlby/hub-os-config/internal/connectivity"
	"github.com/getAlby/hub-os-config/internal/netmgr"
	"github.com/getAlby/hub-os-config/internal/system"
	"github.com/getAlby/hub-os-config/internal/web"
)

const (
	configPath      = "/etc/hub-os-config/config.toml"
	hotspotConnName = "hub-os-config-ap"
	wifiInterface   = "wlan0"
	superviseEvery  = 30 * time.Second
	rebootDelay     = 2 * time.Second
)

func runService() error {
	log.Printf("hub-os-config %s starting", version)

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	nm := netmgr.New(netmgr.APConfig{
		Interface:   wifiInterface,
		HotspotName: hotspotConnName,
		SSID:        cfg.AccessPoint.SSID,
		Channel:     cfg.AccessPoint.Channel,
		GatewayIP:   cfg.AccessPoint.GatewayIP,
	})
	checker := connectivity.New(cfg.Connectivity.PrimaryURL, cfg.Connectivity.FallbackURL, cfg.ProbeTimeout())
	monitor := connectivity.NewMonitor(cfg.RetryWindow())
	env := system.NewEnvStore(cfg.Paths.AlbyHubEnv, web.ManagedEnvKeys)
	reboot := system.NewRebooter(rebootDelay)
	state := app.NewFileStateStore(cfg.Paths.StateFile)

	var application *app.App
	ctrl := app.NewController(app.ControllerConfig{
		NM:         nm,
		Env:        env,
		Status:     func() web.AppStatus { return application.WebStatus() },
		Save:       func(r web.SaveRequest) { application.Save(r) },
		GatewayIP:  cfg.AccessPoint.GatewayIP,
		NormalPort: cfg.Web.NormalPort,
		DropInPath: captive.DefaultDropInPath,
	})
	application = app.New(app.Deps{
		NM:         nm,
		Prober:     checker,
		Monitor:    monitor,
		Controller: ctrl,
		State:      state,
		Env:        env,
		Reboot:     reboot,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mode, err := application.Boot(ctx)
	if err != nil {
		return err
	}
	log.Printf("entered %s mode", mode)

	application.Supervise(ctx, superviseEvery)
	log.Printf("shutting down")
	return nil
}
