//go:build linux

package netx

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const ShimName = "lanbox-shim"

// ShimUp은 호스트가 ipvlan/macvlan 컨테이너에 도달하도록 심 인터페이스를 만든다.
// 이미 있으면 주소와 경로만 맞춘다.
//
// 주소는 부모 서브넷 마스크를 쓰되 noprefixroute로 둔다. 커널이 붙이는
// 서브넷 연결 경로는 메트릭이 없어 부모의 같은 프리픽스보다 우선한다.
// 그 경로는 지우고, 컨테이너 범위만 더 긴 프리픽스로 shim에 둔다.
func ShimUp(parent, driver, ip, subnet, ipRange string) error {
	if parent == "" || ip == "" || subnet == "" {
		return fmt.Errorf("shim에는 parent, ip, subnet이 필요합니다")
	}
	plan, err := PlanShimRoutes(subnet, ipRange, ip)
	if err != nil {
		return err
	}
	parentLink, err := netlink.LinkByName(parent)
	if err != nil {
		return fmt.Errorf("부모 인터페이스 %s: %w", parent, err)
	}
	link, err := ensureShimLink(parentLink, driver)
	if err != nil {
		return err
	}
	if err := ensureShimAddr(link, plan.Src, plan.Subnet.Mask); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("shim 활성화: %w", err)
	}
	if err := requireNoPrefixRoute(link, plan.Src); err != nil {
		return err
	}
	if err := deleteBroadShimRoutes(link, plan.Subnet); err != nil {
		return err
	}
	if err := installContainerRoute(link, plan); err != nil {
		return err
	}
	if err := deleteBroadShimRoutes(link, plan.Subnet); err != nil {
		return err
	}
	if left, err := broadShimRoute(link, plan.Subnet); err != nil {
		return err
	} else if left != "" {
		return fmt.Errorf("shim에 넓은 경로가 남아 있습니다: %s", left)
	}
	return nil
}

func ensureShimLink(parentLink netlink.Link, driver string) (netlink.Link, error) {
	link, err := netlink.LinkByName(ShimName)
	if err == nil {
		return link, nil
	}
	switch driver {
	case "macvlan":
		link = &netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{Name: ShimName, ParentIndex: parentLink.Attrs().Index},
			Mode:      netlink.MACVLAN_MODE_BRIDGE,
		}
	default:
		link = &netlink.IPVlan{
			LinkAttrs: netlink.LinkAttrs{Name: ShimName, ParentIndex: parentLink.Attrs().Index},
			Mode:      netlink.IPVLAN_MODE_L2,
		}
	}
	if err := netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("shim 생성: %w", err)
	}
	link, err = netlink.LinkByName(ShimName)
	if err != nil {
		return nil, err
	}
	return link, nil
}

// ensureShimAddr는 shim 주소에 noprefixroute를 둔다.
// 이미 있는 주소에 replace를 하면 이 플래그가 갱신되지 않아, 커널 연결 경로가 다시 생긴다.
// 플래그가 없으면 주소를 지웠다가 다시 넣는다.
func ensureShimAddr(link netlink.Link, ip net.IP, mask net.IPMask) error {
	want := &netlink.Addr{
		IPNet: &net.IPNet{IP: ip, Mask: mask},
		Flags: unix.IFA_F_NOPREFIXROUTE,
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	for _, a := range addrs {
		if !a.IP.Equal(ip) {
			continue
		}
		if a.Flags&unix.IFA_F_NOPREFIXROUTE != 0 {
			return nil
		}
		existing := a
		if err := netlink.AddrDel(link, &existing); err != nil {
			return fmt.Errorf("shim 주소 교체: %w", err)
		}
		break
	}
	if err := netlink.AddrAdd(link, want); err != nil {
		return fmt.Errorf("shim 주소 추가: %w", err)
	}
	return nil
}

func requireNoPrefixRoute(link netlink.Link, ip net.IP) error {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	for _, a := range addrs {
		if !a.IP.Equal(ip) {
			continue
		}
		if a.Flags&unix.IFA_F_NOPREFIXROUTE == 0 {
			return fmt.Errorf("shim 주소 %s 에 noprefixroute가 없습니다", ip)
		}
		return nil
	}
	return fmt.Errorf("shim 주소 %s 가 없습니다", ip)
}

func deleteBroadShimRoutes(link netlink.Link, subnet *net.IPNet) error {
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("shim 경로 조회: %w", err)
	}
	idx := link.Attrs().Index
	for _, r := range routes {
		if r.LinkIndex != idx || !dropBroadShimRoute(r.Dst, subnet) {
			continue
		}
		if err := netlink.RouteDel(&r); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("shim 넓은 경로 삭제 %s: %w", r.Dst, err)
		}
	}
	return nil
}

func broadShimRoute(link netlink.Link, subnet *net.IPNet) (string, error) {
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return "", fmt.Errorf("shim 경로 조회: %w", err)
	}
	idx := link.Attrs().Index
	for _, r := range routes {
		if r.LinkIndex == idx && dropBroadShimRoute(r.Dst, subnet) && r.Dst != nil {
			return r.Dst.String(), nil
		}
	}
	return "", nil
}

func installContainerRoute(link netlink.Link, plan ShimRoutePlan) error {
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       plan.Container,
		Src:       plan.Src,
		Scope:     netlink.SCOPE_LINK,
		Protocol:  unix.RTPROT_BOOT,
		Type:      unix.RTN_UNICAST,
		Table:     unix.RT_TABLE_MAIN,
	}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("컨테이너 경로 %s: %w", plan.Container, err)
	}
	return nil
}

// ShimDown은 shim 인터페이스를 지운다. 없으면 성공이다.
func ShimDown() error {
	link, err := netlink.LinkByName(ShimName)
	if err != nil {
		return nil
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("shim 삭제: %w", err)
	}
	return nil
}

// ShimExists는 shim 인터페이스가 있는지 본다.
func ShimExists() (bool, error) {
	_, err := netlink.LinkByName(ShimName)
	if err != nil {
		return false, nil
	}
	return true, nil
}
