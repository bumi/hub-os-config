package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/getAlby/hub-os-config/internal/updater"
)

// updateURL is the hard-coded location of the replacement binary.
const updateURL = "https://github.com/getAlby/hub-os-config/releases/latest/download/hub-os-config"

const updateTimeout = 5 * time.Minute

// update downloads the binary from updateURL and replaces the running
// executable in place. The new version takes effect on the next service start.
func update() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	fmt.Printf("downloading update from %s ...\n", updateURL)
	if err := updater.Update(ctx, &http.Client{Timeout: updateTimeout}, updateURL, self); err != nil {
		return err
	}
	fmt.Printf("updated %s\nrestart to apply: systemctl restart hub-os-config (or reboot)\n", self)
	return nil
}
