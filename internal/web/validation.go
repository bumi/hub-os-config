package web

import (
	"fmt"
	"net/url"
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
func validateAdvanced(adv map[string]string) error {
	if v, ok := adv["LN_BACKEND_TYPE"]; ok {
		if v != "LDK" && v != "BARK" {
			return fmt.Errorf("LN_BACKEND_TYPE must be LDK or BARK")
		}
	}
	for _, key := range []string{"RELAY", "LDK_ESPLORA_SERVER"} {
		if v, ok := adv[key]; ok && !isValidURL(v) {
			return fmt.Errorf("%s must be a valid URL", key)
		}
	}
	return nil
}

func isValidURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}
