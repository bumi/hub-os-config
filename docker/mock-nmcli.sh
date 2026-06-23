#!/bin/sh
# Mock nmcli for local Docker testing — the real WiFi/AP logic does not run on
# a Mac/container. It answers just the queries hub-os-config makes; every action
# (connection add/up/down/delete) is a no-op that succeeds, EXCEPT activating
# the demo network "FailNet", which fails so the wrong-password retry flow can
# be exercised.

case "$*" in
  *"NAME,TYPE connection show"*)
    # No saved Wi-Fi connection -> the app boots into Setup Mode.
    echo "Wired connection 1:802-3-ethernet"
    exit 0 ;;
  *"SSID,SIGNAL,SECURITY dev wifi list"*)
    printf '%s\n' \
      "Honey Home:92:WPA2" \
      "AlbyGuest:70:WPA2" \
      "FRITZ!Box 7590:61:WPA2" \
      "CoffeeShop_Free:40:" \
      "FailNet:30:WPA2"
    exit 0 ;;
  *"ACTIVE,SSID dev wifi list"*)
    echo "no:Honey Home"
    exit 0 ;;
esac

# Connecting to "FailNet" fails, to demo the wrong-password / retry path.
if [ "$1" = "device" ] && [ "$2" = "wifi" ] && [ "$3" = "connect" ] && [ "$4" = "FailNet" ]; then
  echo "Error: Connection activation failed: Secrets were required but not provided." >&2
  exit 4
fi

# All other actions succeed silently.
exit 0
