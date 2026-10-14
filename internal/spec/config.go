package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kmsdoit/lanbox/internal/labels"
	"gopkg.in/yaml.v3"
)

const (
	DefaultDir  = "/etc/lanbox"
	ConfigFile  = "config.yaml"
	DefaultNet  = "lanbox_net"
	DefaultUser = "lanbox"
	DefaultMode = "light"
	DefaultKey  = "~/.ssh/id_ed25519.pub"
)

// Config는 init이 만드는 호스트 설정이다. 같은 내용은 네트워크 라벨에도 남긴다.
type Config struct {
	Network  Network  `yaml:"network"`
	Shim     Shim     `yaml:"shim"`
	Reserve  []string `yaml:"reserve,omitempty"`
	Defaults Defaults `yaml:"defaults"`
}

type Network struct {
	Name    string `yaml:"name"`
	Driver  string `yaml:"driver"`
	Parent  string `yaml:"parent"`
	Subnet  string `yaml:"subnet"`
	Gateway string `yaml:"gateway"`
	Range   string `yaml:"range"`
	HostIP  string `yaml:"host_ip,omitempty"`
}

type Shim struct {
	Enabled bool   `yaml:"enabled"`
	IP      string `yaml:"ip,omitempty"`
}

type Defaults struct {
	User   string `yaml:"user"`
	SSHKey string `yaml:"ssh_key"`
	Mode   string `yaml:"mode"`
}

// Dir는 --config-dir, LANBOX_CONFIG_DIR, /etc/lanbox 순서로 정한다.
func Dir(flag string) string {
	if strings.TrimSpace(flag) != "" {
		return flag
	}
	if v := strings.TrimSpace(os.Getenv("LANBOX_CONFIG_DIR")); v != "" {
		return v
	}
	return DefaultDir
}

func Path(dir string) string {
	return filepath.Join(dir, ConfigFile)
}

// Load는 설정 파일을 읽는다. 없으면 os.ErrNotExist를 감싸 반환한다.
func Load(dir string) (Config, error) {
	raw, err := os.ReadFile(Path(dir))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("config.yaml 파싱: %w", err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// Save는 설정 파일을 만든다.
func Save(dir string, cfg Config) error {
	cfg.applyDefaults()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("설정 디렉터리 생성: %w", err)
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := []byte("# lanbox init이 생성한 호스트 설정이다.\n")
	return os.WriteFile(Path(dir), append(header, raw...), 0o644)
}

func (c *Config) applyDefaults() {
	if c.Network.Name == "" {
		c.Network.Name = DefaultNet
	}
	if c.Defaults.User == "" {
		c.Defaults.User = DefaultUser
	}
	if c.Defaults.SSHKey == "" {
		c.Defaults.SSHKey = DefaultKey
	}
	if c.Defaults.Mode == "" {
		c.Defaults.Mode = DefaultMode
	}
}

// Labels는 네트워크에 붙일 라벨이다. 설정 파일이 없어도 이 값으로 복구한다.
func (c Config) Labels() map[string]string {
	c.applyDefaults()
	return map[string]string{
		labels.Managed: "true",
		labels.Role:    labels.RoleNetwork,
		labels.Parent:  c.Network.Parent,
		labels.Subnet:  c.Network.Subnet,
		labels.Gateway: c.Network.Gateway,
		labels.Range:   c.Network.Range,
		labels.Driver:  c.Network.Driver,
		labels.Shim:    boolLabel(c.Shim.Enabled),
		labels.ShimIP:  c.Shim.IP,
		labels.Reserve: strings.Join(c.Reserve, ","),
		labels.HostIP:  c.Network.HostIP,
		labels.User:    c.Defaults.User,
		labels.Name:    c.Network.Name,
	}
}

// ConfigFromLabels는 네트워크 라벨에서 설정을 복구한다.
func ConfigFromLabels(name string, m map[string]string) (Config, error) {
	if !labels.ManagedTrue(m) || m[labels.Role] != labels.RoleNetwork {
		return Config{}, fmt.Errorf("lanbox 네트워크 라벨이 아닙니다")
	}
	cfg := Config{
		Network: Network{
			Name:    name,
			Driver:  m[labels.Driver],
			Parent:  m[labels.Parent],
			Subnet:  m[labels.Subnet],
			Gateway: m[labels.Gateway],
			Range:   m[labels.Range],
			HostIP:  m[labels.HostIP],
		},
		Shim: Shim{
			Enabled: m[labels.Shim] == "true",
			IP:      m[labels.ShimIP],
		},
		Defaults: Defaults{User: m[labels.User]},
	}
	if m[labels.Reserve] != "" {
		for _, p := range strings.Split(m[labels.Reserve], ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.Reserve = append(cfg.Reserve, p)
			}
		}
	}
	if cfg.Network.Name == "" {
		cfg.Network.Name = name
	}
	cfg.applyDefaults()
	if cfg.Network.Subnet == "" || cfg.Network.Range == "" || cfg.Network.Parent == "" {
		return Config{}, fmt.Errorf("네트워크 라벨에 서브넷, 범위, 부모 인터페이스가 없습니다")
	}
	return cfg, nil
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
