package app

import "github.com/getAlby/hub-os-config/internal/web"

// WebStatus exposes the current operational status to the web UI. It is safe
// for concurrent use with the supervisor loop.
func (a *App) WebStatus() web.AppStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return web.AppStatus{
		Mode:        string(a.mode),
		Online:      a.online,
		LastAttempt: a.lastAttempt,
	}
}
