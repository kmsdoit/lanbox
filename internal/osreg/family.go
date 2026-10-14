package osreg

import "fmt"

const (
	FamilyDebian = "debian"
	FamilyRHEL   = "rhel"
	FamilyAlpine = "alpine"
	FamilySUSE   = "suse"
	FamilyArch   = "arch"
)

// families는 계획서의 감지 순서다. 먼저 맞는 계열을 쓴다.
var families = []struct {
	name string
	ids  []string
}{
	{FamilyDebian, []string{"debian", "ubuntu"}},
	{FamilyRHEL, []string{"rhel", "fedora", "centos"}},
	{FamilyAlpine, []string{"alpine"}},
	{FamilySUSE, []string{"suse", "opensuse"}},
	{FamilyArch, []string{"arch"}},
}

// KnownFamily는 지원 계열인지 확인한다.
func KnownFamily(name string) bool {
	for _, f := range families {
		if f.name == name {
			return true
		}
	}
	return false
}

// Families는 지원 계열 이름을 반환한다.
func Families() []string {
	out := make([]string, len(families))
	for i, f := range families {
		out[i] = f.name
	}
	return out
}

// Detect는 os-release 토큰으로 패키지 매니저 계열을 정한다.
func Detect(rel Release) (string, error) {
	toks := rel.Tokens()
	for _, fam := range families {
		for _, id := range fam.ids {
			for _, tok := range toks {
				if tok == id {
					return fam.name, nil
				}
			}
		}
	}
	if len(toks) == 0 {
		return "", fmt.Errorf("os-release에서 ID를 찾지 못했습니다. --family로 지정하세요")
	}
	return "", fmt.Errorf("지원하지 않는 OS 계열(%s)입니다. --family로 지정하세요", stringsJoin(toks))
}

func stringsJoin(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}
