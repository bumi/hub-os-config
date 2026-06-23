package system

import (
	"os/exec"
	"time"
)

// Rebooter schedules a system reboot via systemd. It implements web.Rebooter.
//
// Reboot returns immediately and performs the reboot after a short delay, so an
// in-flight HTTP response (the "applying, rebooting…" page) can be delivered
// before the device goes down.
type Rebooter struct {
	delay time.Duration
	run   func(name string, args ...string) error
}

// NewRebooter returns a Rebooter that waits delay before rebooting.
func NewRebooter(delay time.Duration) *Rebooter {
	return &Rebooter{
		delay: delay,
		run: func(name string, args ...string) error {
			return exec.Command(name, args...).Run()
		},
	}
}

// Reboot schedules the reboot and returns without waiting.
func (r *Rebooter) Reboot() error {
	go func() {
		if r.delay > 0 {
			time.Sleep(r.delay)
		}
		_ = r.run("systemctl", "reboot")
	}()
	return nil
}
