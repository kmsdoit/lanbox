package osreg

import (
	"bufio"
	"io"
	"strings"
)

// Release는 /etc/os-release에서 계열 판정에 쓰는 필드다.
type Release struct {
	ID         string
	IDLike     string
	Name       string
	VersionID  string
	PrettyName string
}

// Parse는 os-release 형식(KEY=VALUE, 따옴표, 주석)을 읽는다.
func Parse(r io.Reader) (Release, error) {
	var rel Release
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = unquote(strings.TrimSpace(val))
		switch strings.TrimSpace(key) {
		case "ID":
			rel.ID = val
		case "ID_LIKE":
			rel.IDLike = val
		case "NAME":
			rel.Name = val
		case "VERSION_ID":
			rel.VersionID = val
		case "PRETTY_NAME":
			rel.PrettyName = val
		}
	}
	return rel, sc.Err()
}

func unquote(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)
	s = strings.TrimSuffix(strings.TrimPrefix(s, `'`), `'`)
	return strings.TrimSpace(s)
}

// Tokens는 ID와 ID_LIKE를 소문자 토큰으로 펼친다.
func (rel Release) Tokens() []string {
	raw := rel.ID + " " + strings.NewReplacer(",", " ").Replace(rel.IDLike)
	fields := strings.Fields(strings.ToLower(raw))
	seen := map[string]struct{}{}
	var out []string
	for _, f := range fields {
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}
