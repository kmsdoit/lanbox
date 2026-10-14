package osreg

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/distribution/reference"
	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaultsFS embed.FS

const OSFile = "os.yaml"

var aliasName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,40}$`)

// Registry는 별칭 → 이미지 매핑이다. 사용자 파일이 기본값을 덮어쓴다.
type Registry struct {
	Aliases map[string]string `yaml:"aliases"`
	source  map[string]string // alias → "builtin" | "user"
}

type aliasFile struct {
	Aliases map[string]string `yaml:"aliases"`
}

// Load는 내장 별칭과 configDir/os.yaml을 합친다. 파일이 없어도 된다.
func Load(configDir string) (*Registry, error) {
	raw, err := defaultsFS.ReadFile("defaults.yaml")
	if err != nil {
		return nil, err
	}
	reg := &Registry{Aliases: map[string]string{}, source: map[string]string{}}
	if err := merge(reg, raw, "builtin"); err != nil {
		return nil, err
	}
	if configDir == "" {
		return reg, nil
	}
	path := filepath.Join(configDir, OSFile)
	user, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, fmt.Errorf("os.yaml 읽기: %w", err)
	}
	if err := merge(reg, user, "user"); err != nil {
		return nil, fmt.Errorf("os.yaml 파싱: %w", err)
	}
	return reg, nil
}

func merge(reg *Registry, raw []byte, src string) error {
	var f aliasFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return err
	}
	for k, v := range f.Aliases {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		reg.Aliases[k] = v
		reg.source[k] = src
	}
	return nil
}

// Source는 별칭이 builtin인지 user인지 반환한다.
func (r *Registry) Source(alias string) string {
	if r.source == nil {
		return ""
	}
	return r.source[alias]
}

// Resolve는 별칭 또는 이미지 참조를 정규 이미지 이름으로 바꾼다.
// 태그가 없으면 latest를 붙인다. 반환값은 familiar 이름(ubuntu:24.04)이다.
func (r *Registry) Resolve(osName string) (string, error) {
	osName = strings.TrimSpace(osName)
	if osName == "" {
		return "", fmt.Errorf("--os가 필요합니다")
	}
	image := osName
	if !strings.Contains(osName, ":") && !strings.Contains(osName, "/") {
		alias, ok := r.Aliases[osName]
		if !ok {
			return "", fmt.Errorf("알 수 없는 OS 별칭 %q 입니다. 이미지(ubuntu:24.04)를 지정하거나 lanbox os add로 등록하세요", osName)
		}
		image = alias
	}
	return Familiar(image)
}

// Familiar는 이미지 참조를 태그가 있는 짧은 이름으로 정규화한다.
func Familiar(image string) (string, error) {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", fmt.Errorf("이미지 참조가 올바르지 않습니다(%s): %w", image, err)
	}
	named = reference.TagNameOnly(named)
	return reference.FamiliarString(named), nil
}

// Add는 사용자 os.yaml에 별칭을 기록한다. 같은 이름이면 덮어쓴다.
func Add(configDir, alias, image string) (string, error) {
	alias = strings.TrimSpace(alias)
	if !aliasName.MatchString(alias) {
		return "", fmt.Errorf("별칭은 소문자로 시작해야 합니다: %s", alias)
	}
	familiar, err := Familiar(image)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("설정 디렉터리 생성: %w", err)
	}
	path := filepath.Join(configDir, OSFile)
	var f aliasFile
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &f); err != nil {
			return "", fmt.Errorf("os.yaml 파싱: %w", err)
		}
	}
	if f.Aliases == nil {
		f.Aliases = map[string]string{}
	}
	f.Aliases[alias] = familiar
	out, err := yaml.Marshal(f)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", fmt.Errorf("os.yaml 기록: %w", err)
	}
	return familiar, nil
}
