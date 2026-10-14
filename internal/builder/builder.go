// Package builder는 계열별 Dockerfile 템플릿을 렌더링하고 캐시 태그를 만든다.
package builder

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/distribution/reference"
)

//go:embed templates/*.tmpl entrypoint.sh bootstrap.sh
var files embed.FS

// Data는 템플릿에 넘기는 값이다.
type Data struct {
	Base         string
	User         string
	Extra        string
	Systemd      bool
	PasswordAuth string
}

var userPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// Render는 계열 템플릿으로 Dockerfile 본문을 만든다.
func Render(family string, data Data) (string, error) {
	if !userPattern.MatchString(data.User) {
		return "", fmt.Errorf("계정 이름이 올바르지 않습니다: %s", data.User)
	}
	if data.PasswordAuth != "yes" && data.PasswordAuth != "no" {
		return "", fmt.Errorf("PasswordAuth는 yes 또는 no 여야 합니다")
	}
	name := "templates/" + family + ".tmpl"
	raw, err := files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("계열 %s 템플릿이 없습니다", family)
	}
	tpl, err := template.New(family).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("템플릿 파싱(%s): %w", family, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Hash는 템플릿과 계정, 패키지, 비밀번호 로그인, 모드를 포함한 캐시 키다.
// 같은 OS라도 계정이나 추가 패키지가 다르면 다른 파생 이미지를 쓴다.
func Hash(family string, user string, packages []string, passwordAuth bool, mode string) (string, error) {
	raw, err := files.ReadFile("templates/" + family + ".tmpl")
	if err != nil {
		return "", fmt.Errorf("계열 %s 템플릿이 없습니다", family)
	}
	entry, err := files.ReadFile("entrypoint.sh")
	if err != nil {
		return "", err
	}
	boot, err := files.ReadFile("bootstrap.sh")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "family=%s\nuser=%s\npkgs=%s\npassword=%t\nmode=%s\n",
		family, user, strings.Join(packages, ","), passwordAuth, mode)
	h.Write(raw)
	h.Write(entry)
	h.Write(boot)
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// DerivedRef는 lanbox/<os>-<tag>:<해시> 형식의 파생 이미지 이름이다.
func DerivedRef(baseImage, hash string) (string, error) {
	named, err := reference.ParseNormalizedNamed(baseImage)
	if err != nil {
		return "", fmt.Errorf("기본 이미지 참조: %w", err)
	}
	named = reference.TagNameOnly(named)
	path := reference.Path(named)
	path = strings.TrimPrefix(path, "library/")
	tag := "latest"
	if tagged, ok := named.(reference.Tagged); ok {
		tag = tagged.Tag()
	}
	safe := sanitize(path + "-" + tag)
	ref := "lanbox/" + safe + ":" + hash
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return "", fmt.Errorf("파생 이미지 이름 생성 실패(%s): %w", ref, err)
	}
	return ref, nil
}

// SnapshotRef는 스냅샷 이미지 이름을 만든다.
func SnapshotRef(name string, when time.Time) (string, error) {
	safe := sanitize(name)
	ref := fmt.Sprintf("lanbox/snap-%s:%s", safe, when.UTC().Format("20060102t150405z"))
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return "", fmt.Errorf("스냅샷 이름 생성 실패(%s): %w", ref, err)
	}
	return ref, nil
}

func sanitize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			r = '-'
		}
		if r == '-' && prevDash {
			continue
		}
		prevDash = r == '-'
		b.WriteRune(r)
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "image"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

// ContextTar는 Dockerfile과 엔트리포인트 스크립트를 담은 빌드 컨텍스트다.
func ContextTar(dockerfile string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	entries := map[string]string{"Dockerfile": dockerfile}
	for _, name := range []string{"entrypoint.sh", "bootstrap.sh"} {
		b, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		entries[name] = string(b)
	}
	for name, body := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(body)),
		}
		if strings.HasSuffix(name, ".sh") {
			hdr.Mode = 0o755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(tw, body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}
