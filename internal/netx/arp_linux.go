//go:build linux

package netx

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/mdlayher/arp"
)

// ARPProbe는 parent에서 ARP 요청을 보내고 응답이 있으면 true다.
// 타임아웃이면 사용 중이 아니다. 권한이 없으면 오류를 반환한다.
func ARPProbe(parent string, ip netip.Addr, timeout time.Duration) (bool, error) {
	if !ip.Is4() {
		return false, fmt.Errorf("ARP는 IPv4만 확인합니다: %s", ip)
	}
	ifi, err := net.InterfaceByName(parent)
	if err != nil {
		return false, fmt.Errorf("ARP 인터페이스 %s: %w", parent, err)
	}
	c, err := arp.Dial(ifi)
	if err != nil {
		return false, fmt.Errorf("ARP 소켓을 열지 못했습니다 (root 또는 CAP_NET_RAW 필요): %w", err)
	}
	defer c.Close()
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
		return false, err
	}
	hw, err := c.Resolve(ip)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return false, nil
		}
		return false, err
	}
	return hw != nil, nil
}
