//go:build linux

package netx

import (
	"fmt"
	"net"

	"github.com/kmsdoit/lanbox/internal/ipam"
	"github.com/vishvananda/netlink"
)

// LinkInfo는 기본 경로 인터페이스와 그 주소다.
type LinkInfo struct {
	Name            string
	Wireless        bool
	Subnet          string
	Gateway         string
	HostIP          string
	SuggestedDriver string
}

// Detect는 기본 IPv4 경로의 인터페이스, 주소, 게이트웨이를 읽는다.
func Detect() (LinkInfo, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return LinkInfo{}, fmt.Errorf("라우트 조회: %w", err)
	}
	var def *netlink.Route
	for i := range routes {
		r := &routes[i]
		if r.Dst == nil && r.Gw != nil && r.LinkIndex > 0 {
			def = r
			break
		}
	}
	if def == nil {
		return LinkInfo{}, fmt.Errorf("기본 IPv4 경로를 찾지 못했습니다")
	}
	link, err := netlink.LinkByIndex(def.LinkIndex)
	if err != nil {
		return LinkInfo{}, fmt.Errorf("인터페이스 조회: %w", err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return LinkInfo{}, fmt.Errorf("주소 조회: %w", err)
	}
	if len(addrs) == 0 || addrs[0].IPNet == nil {
		return LinkInfo{}, fmt.Errorf("%s에 IPv4 주소가 없습니다", link.Attrs().Name)
	}
	ip := addrs[0].IP.To4()
	if ip == nil {
		return LinkInfo{}, fmt.Errorf("%s에 IPv4 주소가 없습니다", link.Attrs().Name)
	}
	mask := addrs[0].IPNet.Mask
	ones, _ := mask.Size()
	network := ip.Mask(mask)
	info := LinkInfo{
		Name:     link.Attrs().Name,
		Wireless: IsWireless("", link.Attrs().Name),
		Subnet:   fmt.Sprintf("%s/%d", network.String(), ones),
		HostIP:   ip.String(),
	}
	if def.Gw != nil {
		info.Gateway = def.Gw.String()
	}
	info.SuggestedDriver = ipam.SuggestDriver(info.Wireless)
	if info.Gateway == "" {
		return info, fmt.Errorf("%s의 게이트웨이를 찾지 못했습니다", info.Name)
	}
	return info, nil
}

// InterfaceExists는 부모 인터페이스가 있는지 확인한다.
func InterfaceExists(name string) error {
	if _, err := net.InterfaceByName(name); err != nil {
		return fmt.Errorf("인터페이스 %s 를 찾지 못했습니다: %w", name, err)
	}
	return nil
}
