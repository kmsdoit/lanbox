package spec

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Lab은 apply에 쓰는 선언형 스펙이다.
type Lab struct {
	Instances []LabInstance `yaml:"instances"`
}

// LabInstance는 인스턴스 하나 또는 count개의 묶음이다.
type LabInstance struct {
	Name       string   `yaml:"name"`
	OS         string   `yaml:"os"`
	Count      int      `yaml:"count,omitempty"`
	CPUs       string   `yaml:"cpus,omitempty"`
	Memory     string   `yaml:"memory,omitempty"`
	GPUs       string   `yaml:"gpus,omitempty"`
	Packages   []string `yaml:"packages,omitempty"`
	IP         string   `yaml:"ip,omitempty"`
	User       string   `yaml:"user,omitempty"`
	Mode       string   `yaml:"mode,omitempty"`
	Hostname   string   `yaml:"hostname,omitempty"`
	HomeVolume bool     `yaml:"home_volume,omitempty"`
}

// UnmarshalYAML는 cpus: 2 처럼 숫자로 적어도 문자열로 받는다.
func (i *LabInstance) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Name       string    `yaml:"name"`
		OS         string    `yaml:"os"`
		Count      int       `yaml:"count"`
		CPUs       yaml.Node `yaml:"cpus"`
		Memory     yaml.Node `yaml:"memory"`
		GPUs       yaml.Node `yaml:"gpus"`
		Packages   []string  `yaml:"packages"`
		IP         string    `yaml:"ip"`
		User       string    `yaml:"user"`
		Mode       string    `yaml:"mode"`
		Hostname   string    `yaml:"hostname"`
		HomeVolume bool      `yaml:"home_volume"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	cpus, err := nodeString(&raw.CPUs)
	if err != nil {
		return fmt.Errorf("cpus: %w", err)
	}
	mem, err := nodeString(&raw.Memory)
	if err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	gpus, err := nodeString(&raw.GPUs)
	if err != nil {
		return fmt.Errorf("gpus: %w", err)
	}
	*i = LabInstance{
		Name:       strings.TrimSpace(raw.Name),
		OS:         strings.TrimSpace(raw.OS),
		Count:      raw.Count,
		CPUs:       cpus,
		Memory:     mem,
		GPUs:       gpus,
		Packages:   raw.Packages,
		IP:         strings.TrimSpace(raw.IP),
		User:       strings.TrimSpace(raw.User),
		Mode:       strings.TrimSpace(raw.Mode),
		Hostname:   strings.TrimSpace(raw.Hostname),
		HomeVolume: raw.HomeVolume,
	}
	if i.Count == 0 {
		i.Count = 1
	}
	return nil
}

func nodeString(n *yaml.Node) (string, error) {
	if n == nil || n.Tag == "" && n.Value == "" && n.Kind == 0 {
		return "", nil
	}
	if n.Kind == 0 || n.Tag == "!!null" || n.Value == "" && n.Kind != yaml.ScalarNode {
		if n.Kind != yaml.ScalarNode {
			return "", nil
		}
	}
	if n.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("스칼라 값이어야 합니다")
	}
	if n.Tag == "!!null" || n.Value == "" || n.Value == "null" {
		return "", nil
	}
	if n.Tag == "!!float" || n.Tag == "!!int" {
		if n.Tag == "!!float" {
			f, err := strconv.ParseFloat(n.Value, 64)
			if err != nil {
				return "", err
			}
			return strconv.FormatFloat(f, 'f', -1, 64), nil
		}
	}
	return n.Value, nil
}

// LoadLab은 랩 스펙 파일을 읽는다.
func LoadLab(path string) (Lab, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Lab{}, fmt.Errorf("스펙 파일 읽기: %w", err)
	}
	var lab Lab
	if err := yaml.Unmarshal(raw, &lab); err != nil {
		return Lab{}, fmt.Errorf("스펙 파싱: %w", err)
	}
	if len(lab.Instances) == 0 {
		return Lab{}, fmt.Errorf("스펙에 instances가 없습니다")
	}
	seen := map[string]struct{}{}
	for i, inst := range lab.Instances {
		if inst.Name == "" {
			return Lab{}, fmt.Errorf("instances[%d]에 name이 없습니다", i)
		}
		if inst.OS == "" {
			return Lab{}, fmt.Errorf("%s에 os가 없습니다", inst.Name)
		}
		if inst.Count < 1 {
			return Lab{}, fmt.Errorf("%s의 count는 1 이상이어야 합니다", inst.Name)
		}
		if _, ok := seen[inst.Name]; ok {
			return Lab{}, fmt.Errorf("이름이 중복되었습니다: %s", inst.Name)
		}
		seen[inst.Name] = struct{}{}
	}
	return lab, nil
}
