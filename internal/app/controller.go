package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/getAlby/hub-os-config/internal/captive"
	"github.com/getAlby/hub-os-config/internal/netmgr"
	"github.com/getAlby/hub-os-config/internal/web"
)

const (
	hotspotAttempts     = 3
	hotspotRetryDelay   = 2 * time.Second
	serveBindAttempts   = 5
	serveBindRetryDelay = 2 * time.Second
)

// ControllerConfig wires the real Controller to the host.
type ControllerConfig struct {
	NM         netmgr.Manager
	Env        web.EnvStore
	Status     func() web.AppStatus
	Save       func(web.SaveRequest)
	GatewayIP  string
	NormalPort int
	DropInPath string // dnsmasq-shared.d captive drop-in
}

// realController brings the access point and web server up or down for each
// mode. It is host-touching glue, validated on hardware rather than in unit
// tests.
type realController struct {
	cfg ControllerConfig

	mu  sync.Mutex
	srv *http.Server
}

// NewController returns a Controller that drives NetworkManager and an HTTP
// server.
func NewController(cfg ControllerConfig) Controller {
	return &realController{cfg: cfg}
}

// EnterSetup brings up the open AP with captive-portal redirects and serves the
// UI on the gateway.
func (c *realController) EnterSetup(ctx context.Context) error {
	if err := captive.WriteDNSRedirect(c.cfg.DropInPath, c.cfg.GatewayIP); err != nil {
		log.Printf("warning: writing captive DNS drop-in: %v", err)
	}
	// The radio may not be ready immediately after boot or after dropping a
	// station connection, so retry bringing the AP up.
	if err := retry(ctx, hotspotAttempts, hotspotRetryDelay, func() error {
		return c.cfg.NM.StartHotspot(ctx)
	}); err != nil {
		return err
	}
	addr := net.JoinHostPort(c.cfg.GatewayIP, "80")
	portal := "http://" + c.cfg.GatewayIP + "/"
	return c.serve(addr, portal, true)
}

// EnterNormal tears down the AP and serves the UI on the LAN port.
func (c *realController) EnterNormal(ctx context.Context) error {
	if err := c.cfg.NM.StopHotspot(ctx); err != nil {
		log.Printf("warning: stopping hotspot: %v", err)
	}
	addr := ":" + strconv.Itoa(c.cfg.NormalPort)
	return c.serve(addr, "", false)
}

func (c *realController) serve(addr, portalURL string, captivePortal bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.srv != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = c.srv.Shutdown(shutCtx)
		cancel()
		c.srv = nil
	}

	server := web.NewServer(web.Deps{
		NM:        c.cfg.NM,
		Env:       c.cfg.Env,
		Status:    c.cfg.Status,
		Save:      c.cfg.Save,
		PortalURL: portalURL,
		Captive:   captivePortal,
	})
	c.srv = &http.Server{Addr: addr, Handler: server.Handler()}

	go func(s *http.Server) {
		log.Printf("serving config UI on %s (captive=%v)", addr, captivePortal)
		// Retry the bind: right after the AP comes up the gateway IP may not be
		// assigned to the interface yet. ErrServerClosed means a mode switch shut
		// this server down — stop then.
		for attempt := 1; ; attempt++ {
			err := s.ListenAndServe()
			if err == nil || err == http.ErrServerClosed {
				return
			}
			log.Printf("web server on %s failed (attempt %d/%d): %v", addr, attempt, serveBindAttempts, err)
			if attempt >= serveBindAttempts {
				log.Printf("giving up serving on %s", addr)
				return
			}
			time.Sleep(serveBindRetryDelay)
		}
	}(c.srv)
	return nil
}

// retry calls fn until it succeeds, up to attempts times, sleeping delay
// between tries (aborting early if ctx is cancelled).
func retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return err
			case <-time.After(delay):
			}
		}
	}
	return err
}
