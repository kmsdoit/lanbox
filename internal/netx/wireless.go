package netx

import (
	"os"
	"path/filepath"
)

// IsWireless는 sysfs에 wireless 또는 phy80211이 있으면 무선으로 본다.
func IsWireless(sysfsRoot, iface string) bool {
	if iface == "" {
		return false
	}
	if sysfsRoot == "" {
		sysfsRoot = "/sys/class/net"
	}
	base := filepath.Join(sysfsRoot, iface)
	if _, err := os.Stat(filepath.Join(base, "wireless")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(base, "phy80211")); err == nil {
		return true
	}
	return false
}
