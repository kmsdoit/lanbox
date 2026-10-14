package osreg

import (
	"fmt"
	"sort"
	"strings"
)

// packageAliases는 계열마다 이름이 다른 흔한 패키지를 매핑한다.
// 키는 사용자가 넣을 수 있는 이름, 값은 계열별 실제 패키지 이름이다.
var packageAliases = map[string]map[string]string{
	"iproute2": {
		FamilyRHEL: "iproute",
	},
	"iproute": {
		FamilyDebian: "iproute2",
		FamilyAlpine: "iproute2",
		FamilySUSE:   "iproute2",
		FamilyArch:   "iproute2",
	},
	"iputils-ping": {
		FamilyRHEL:   "iputils",
		FamilyAlpine: "iputils",
		FamilySUSE:   "iputils",
		FamilyArch:   "iputils",
	},
	"iputils": {
		FamilyDebian: "iputils-ping",
	},
	"procps": {
		FamilyRHEL: "procps-ng",
	},
	"procps-ng": {
		FamilyDebian: "procps",
		FamilyAlpine: "procps",
		FamilySUSE:   "procps",
		FamilyArch:   "procps",
	},
}

// NormalizePackages는 추가 패키지 이름을 계열에 맞게 바꾸고 중복을 없앤다.
func NormalizePackages(family string, pkgs []string) ([]string, error) {
	if !KnownFamily(family) {
		return nil, fmt.Errorf("알 수 없는 계열 %q", family)
	}
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range pkgs {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !validPackageName(p) {
				return nil, fmt.Errorf("패키지 이름이 올바르지 않습니다: %s", p)
			}
			if mapped, ok := packageAliases[p][family]; ok {
				p = mapped
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func validPackageName(p string) bool {
	if p == "" || len(p) > 128 {
		return false
	}
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '+' || r == '_' || r == '.' || r == '-':
		default:
			return false
		}
	}
	return true
}
