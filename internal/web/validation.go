package web

import (
	"fmt"
	"strings"
)

// validateWiFi checks the SSID and password. An empty password means an open
// network (allowed); a non-empty one must be a valid WPA PSK length.
func validateWiFi(ssid, password string) error {
	if n := len(ssid); n < 1 || n > 32 {
		return fmt.Errorf("SSID must be 1-32 characters")
	}
	if password != "" {
		if n := len(password); n < 8 || n > 63 {
			return fmt.Errorf("WiFi password must be 8-63 characters")
		}
	}
	return nil
}

// validateAdvanced checks the managed Alby Hub .env values that are present.
// Values may be any single-line text (they aren't required to be URLs), but a
// line break would inject extra lines into the .env file, so it is rejected.
func validateAdvanced(adv map[string]string) error {
	for k, v := range adv {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("%s must be a single line", k)
		}
	}
	if v, ok := adv["LN_BACKEND_TYPE"]; ok && v != "LDK" && v != "BARK" {
		return fmt.Errorf("LN_BACKEND_TYPE must be LDK or BARK")
	}
	return nil
}
