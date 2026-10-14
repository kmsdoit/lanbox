package netx

import (
	"fmt"
	"net"
)

// ShimRoutePlan은 shim이 메인 테이블에 둬야 할 경로다.
// 서브넷과 같거나 더 넓은 경로는 넣지 않는다. 그 경로는 메트릭이 없으면
// 부모 인터페이스의 같은 프리픽스보다 우선해 호스트 LAN 응답이 shim으로 샌다.
// 컨테이너 범위만 더 긴 프리픽스로 둔다. longest prefix match라 부모 /24는 그대로다.
type ShimRoutePlan struct {
	Subnet    *net.IPNet
	Container *net.IPNet
	Src       net.IP
}

// PlanShimRoutes는 부모 서브넷, 컨테이너 범위, shim 주소로 경로 계획을 만든다.
// 컨테이너 범위가 서브넷보다 길지 않으면 오류다. 같은 프리픽스를 shim에 두지 않기 위해서다.
func PlanShimRoutes(subnet, ipRange, shimIP string) (ShimRoutePlan, error) {
	if subnet == "" || ipRange == "" || shimIP == "" {
		return ShimRoutePlan{}, fmt.Errorf("shim 경로에는 subnet, range, ip가 필요합니다")
	}
	_, sn, err := net.ParseCIDR(subnet)
	if err != nil || sn.IP.To4() == nil {
		return ShimRoutePlan{}, fmt.Errorf("shim 서브넷: %s", subnet)
	}
	_, rg, err := net.ParseCIDR(ipRange)
	if err != nil || rg.IP.To4() == nil {
		return ShimRoutePlan{}, fmt.Errorf("shim 컨테이너 범위: %s", ipRange)
	}
	src := net.ParseIP(shimIP).To4()
	if src == nil {
		return ShimRoutePlan{}, fmt.Errorf("shim IP가 올바르지 않습니다: %s", shimIP)
	}
	snBits := prefixBits(sn)
	rgBits := prefixBits(rg)
	if snBits < 0 || rgBits < 0 || rgBits <= snBits {
		return ShimRoutePlan{}, fmt.Errorf("컨테이너 범위 %s 는 서브넷 %s 보다 긴 프리픽스여야 합니다", rg, sn)
	}
	if !sn.Contains(rg.IP) || !sn.Contains(prefixLast(rg)) {
		return ShimRoutePlan{}, fmt.Errorf("컨테이너 범위 %s 가 서브넷 %s 안에 없습니다", rg, sn)
	}
	if !sn.Contains(src) {
		return ShimRoutePlan{}, fmt.Errorf("shim IP %s 가 서브넷 %s 안에 없습니다", src, sn)
	}
	return ShimRoutePlan{Subnet: sn, Container: rg, Src: src}, nil
}

// dropBroadShimRoute는 shim 메인 테이블에서 지울 넓은 경로인지 본다.
// 목적지가 없거나(기본 경로) 서브넷보다 긴 프리픽스는 지우지 않는다.
func dropBroadShimRoute(dst, subnet *net.IPNet) bool {
	if dst == nil || subnet == nil {
		return false
	}
	d := prefixBits(dst)
	s := prefixBits(subnet)
	if d < 0 || s < 0 {
		return false
	}
	return d <= s
}

func prefixBits(n *net.IPNet) int {
	ones, bits := n.Mask.Size()
	if bits != 32 || ones < 0 {
		return -1
	}
	return ones
}

func prefixLast(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	if ip == nil || len(n.Mask) != net.IPv4len {
		return nil
	}
	out := make(net.IP, net.IPv4len)
	for i := range out {
		out[i] = ip[i] | ^n.Mask[i]
	}
	return out
}
