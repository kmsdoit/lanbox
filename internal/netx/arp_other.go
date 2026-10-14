//go:build !linux

package netx

import (
	"fmt"
	"net/netip"
	"time"
)

// ARPProbe는 주소가 LAN에서 쓰이는지 확인한다. Linux가 아니면 건너뛴다.
func ARPProbe(parent string, ip netip.Addr, timeout time.Duration) (bool, error) {
	return false, fmt.Errorf("ARP 확인은 Linux에서 root 권한이 필요합니다")
}
