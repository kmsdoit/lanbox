// Package ipam은 할당 범위에서 비어 있는 LAN IP를 고른다.
package ipam

import (
	"fmt"
	"net/netip"
	"sort"
)

// Pool은 범위 안의 사용 가능 주소와 예약·사용 중 주소다.
type Pool struct {
	hosts []netip.Addr
	used  map[netip.Addr]bool
}

// New는 rangeCIDR에서 네트워크·브로드캐스트 주소를 뺀 호스트 목록을 만든다.
// reserved에 게이트웨이, shim, 호스트 주소를 넣는다. 범위 밖 주소는 무시한다.
func New(rangeCIDR string, reserved []string) (*Pool, error) {
	p, err := netip.ParsePrefix(rangeCIDR)
	if err != nil {
		return nil, fmt.Errorf("IP 범위가 올바른 CIDR이 아닙니다: %s", rangeCIDR)
	}
	p = p.Masked()
	if !p.Addr().Is4() {
		return nil, fmt.Errorf("IPv4 범위만 지원합니다: %s", rangeCIDR)
	}
	var hosts []netip.Addr
	addr := p.Addr()
	for {
		if !p.Contains(addr) {
			break
		}
		hosts = append(hosts, addr)
		next := addr.Next()
		if !next.IsValid() || next == addr || !addr.Less(next) {
			break
		}
		addr = next
	}
	if p.Bits() < 31 && len(hosts) >= 2 {
		hosts = hosts[1 : len(hosts)-1]
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("범위 %s 에 할당 가능한 주소가 없습니다", rangeCIDR)
	}
	pool := &Pool{hosts: hosts, used: map[netip.Addr]bool{}}
	for _, r := range reserved {
		if r == "" {
			continue
		}
		if err := pool.Use(r); err != nil {
			return nil, err
		}
	}
	return pool, nil
}

// Use는 주소를 사용 중으로 표시한다. 범위 밖이면 무시한다.
func (p *Pool) Use(ip string) error {
	if ip == "" {
		return nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("IP 주소가 올바르지 않습니다: %s", ip)
	}
	addr = addr.Unmap()
	for _, h := range p.hosts {
		if h == addr {
			p.used[addr] = true
			return nil
		}
	}
	return nil
}

// Free는 남은 주소 수다.
func (p *Pool) Free() int {
	n := 0
	for _, h := range p.hosts {
		if !p.used[h] {
			n++
		}
	}
	return n
}

// Allocate는 앞에서부터 비어 있는 주소를 n개 고른다.
func (p *Pool) Allocate(n int) ([]string, error) {
	if n < 1 {
		return nil, fmt.Errorf("생성 개수는 1 이상이어야 합니다")
	}
	var out []string
	for _, h := range p.hosts {
		if p.used[h] {
			continue
		}
		out = append(out, h.String())
		if len(out) == n {
			return out, nil
		}
	}
	return nil, fmt.Errorf("빈 IP가 %d개 필요합니다. 남은 주소는 %d개입니다", n, p.Free())
}

// AllocateFrom은 start부터 연속된 n개를 고른다. 중간에 사용 중이면 실패한다.
func (p *Pool) AllocateFrom(start string, n int) ([]string, error) {
	if n < 1 {
		return nil, fmt.Errorf("생성 개수는 1 이상이어야 합니다")
	}
	addr, err := netip.ParseAddr(start)
	if err != nil {
		return nil, fmt.Errorf("시작 IP가 올바르지 않습니다: %s", start)
	}
	addr = addr.Unmap()
	idx := -1
	for i, h := range p.hosts {
		if h == addr {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("시작 IP %s 가 할당 범위 안에 없습니다", start)
	}
	if idx+n > len(p.hosts) {
		return nil, fmt.Errorf("시작 IP %s 부터 %d개를 범위 안에 둘 수 없습니다", start, n)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		h := p.hosts[idx+i]
		if p.used[h] {
			return nil, fmt.Errorf("IP %s 는 이미 예약되었거나 사용 중입니다", h)
		}
		out = append(out, h.String())
	}
	return out, nil
}

// LastUsable은 범위의 마지막 할당 가능 주소다. shim 기본값에 쓴다.
func (p *Pool) LastUsable() string {
	if len(p.hosts) == 0 {
		return ""
	}
	return p.hosts[len(p.hosts)-1].String()
}

// Contains는 주소가 할당 가능 목록에 있는지만 본다. 사용 여부는 보지 않는다.
func (p *Pool) Contains(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, h := range p.hosts {
		if h == addr {
			return true
		}
	}
	return false
}

// Hosts는 테스트와 출력용으로 할당 가능 주소를 복사해 돌려준다.
func (p *Pool) Hosts() []string {
	out := make([]string, len(p.hosts))
	for i, h := range p.hosts {
		out[i] = h.String()
	}
	return out
}

// ValidateNetwork는 서브넷, 게이트웨이, 범위의 포함 관계를 확인한다.
func ValidateNetwork(subnet, gateway, ipRange string) error {
	sn, err := netip.ParsePrefix(subnet)
	if err != nil || !sn.Addr().Is4() {
		return fmt.Errorf("서브넷이 올바른 IPv4 CIDR이 아닙니다: %s", subnet)
	}
	sn = sn.Masked()
	gw, err := netip.ParseAddr(gateway)
	if err != nil || !gw.Is4() {
		return fmt.Errorf("게이트웨이가 올바른 IPv4 주소가 아닙니다: %s", gateway)
	}
	if !sn.Contains(gw) {
		return fmt.Errorf("게이트웨이 %s 가 서브넷 %s 안에 없습니다", gateway, sn)
	}
	rg, err := netip.ParsePrefix(ipRange)
	if err != nil || !rg.Addr().Is4() {
		return fmt.Errorf("IP 범위가 올바른 IPv4 CIDR이 아닙니다: %s", ipRange)
	}
	rg = rg.Masked()
	last := lastAddr(rg)
	if !sn.Contains(rg.Addr()) || !sn.Contains(last) {
		return fmt.Errorf("IP 범위 %s 가 서브넷 %s 안에 없습니다", rg, sn)
	}
	return nil
}

func lastAddr(p netip.Prefix) netip.Addr {
	p = p.Masked()
	addr := p.Addr()
	if p.Bits() >= addr.BitLen() {
		return addr
	}
	bits := addr.BitLen() - p.Bits()
	raw := addr.As4()
	n := uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
	n |= uint32(1<<bits - 1)
	var b [4]byte
	b[0] = byte(n >> 24)
	b[1] = byte(n >> 16)
	b[2] = byte(n >> 8)
	b[3] = byte(n)
	return netip.AddrFrom4(b)
}

// SuggestDriver는 무선이면 ipvlan, 유선이면 macvlan을 제안한다.
func SuggestDriver(wireless bool) string {
	if wireless {
		return "ipvlan"
	}
	return "macvlan"
}

// SortAddrs는 표시용으로 주소를 정렬한다.
func SortAddrs(ips []string) []string {
	out := append([]string(nil), ips...)
	sort.Strings(out)
	return out
}
