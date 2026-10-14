package osreg

import (
	"fmt"
	"strconv"
	"strings"
)

// EOLWarning은 저장소가 닫혔을 가능성이 큰 배포판에 경고 문구를 돌려준다.
// 기준일은 2026-10이다. 모르면 빈 문자열이다.
func EOLWarning(rel Release) string {
	id := strings.ToLower(rel.ID)
	ver := rel.VersionID
	switch id {
	case "ubuntu":
		if ver != "" && versionLess(ver, "22.04") {
			return fmt.Sprintf("Ubuntu %s 는 지원 종료(EOL)로 패키지 설치가 실패할 수 있습니다", ver)
		}
	case "debian":
		if ver != "" && versionLess(ver, "13") {
			return fmt.Sprintf("Debian %s 는 지원 종료(EOL)로 패키지 설치가 실패할 수 있습니다", ver)
		}
	case "centos":
		return "CentOS는 지원이 종료되어 패키지 설치가 실패할 수 있습니다"
	case "alpine":
		if ver != "" && versionLess(ver, "3.21") {
			return fmt.Sprintf("Alpine %s 는 지원 종료(EOL)로 패키지 설치가 실패할 수 있습니다", ver)
		}
	case "fedora":
		// Fedora는 약 1년 주기. 2026-10 기준 42 미만은 종료된 것으로 본다.
		if ver != "" && versionLess(ver, "42") {
			return fmt.Sprintf("Fedora %s 는 지원 종료(EOL)로 패키지 설치가 실패할 수 있습니다", ver)
		}
	}
	return ""
}

func versionLess(got, min string) bool {
	gp := splitVer(got)
	mp := splitVer(min)
	n := len(gp)
	if len(mp) > n {
		n = len(mp)
	}
	for i := 0; i < n; i++ {
		gv, mv := 0, 0
		if i < len(gp) {
			gv = gp[i]
		}
		if i < len(mp) {
			mv = mp[i]
		}
		if gv < mv {
			return true
		}
		if gv > mv {
			return false
		}
	}
	return false
}

func splitVer(s string) []int {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	var out []int
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
