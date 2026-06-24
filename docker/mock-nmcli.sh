#!/bin/sh
# Mock nmcli for local Docker testing — the real WiFi/AP logic does not run on a
# Mac/container. It answers just the queries hub-os-config makes; actions are
# no-ops that succeed, EXCEPT activating "FailNet" (to demo the wrong-password
# retry flow).
#
# It is STATEFUL: a successful `device wifi connect <ssid>` records the SSID to a
# file, so that after a container restart `connection show` reports a saved WiFi
# connection — exactly as a real device does (NetworkManager persists an
# autoconnecting profile). That lets the reboot -> reconnect -> Normal Mode flow
# be tested locally. (The file lives in the container's writable layer, so it
# survives `docker restart` but not `docker rm`.)

STATE=/var/lib/mock-nmcli/saved-ssid

case "$*" in
  *"NAME,TYPE connection show"*)
    echo "Wired connection 1:802-3-ethernet"
    if [ -s "$STATE" ]; then
      printf '%s:802-11-wireless\n' "$(cat "$STATE")"
    fi
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
    if [ -s "$STATE" ]; then
      printf 'yes:%s\n' "$(cat "$STATE")"
    else
      echo "no:Honey Home"
    fi
    exit 0 ;;
esac

# Joining a network: remember it (so a restart reconnects), except "FailNet".
if [ "$1" = "device" ] && [ "$2" = "wifi" ] && [ "$3" = "connect" ]; then
  if [ "$4" = "FailNet" ]; then
    echo "Error: Connection activation failed: Secrets were required but not provided." >&2
    exit 4
  fi
  mkdir -p "$(dirname "$STATE")"
  printf '%s' "$4" >"$STATE"
  exit 0
fi

# All other actions succeed silently.
exit 0
