package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamesAndDiff(t *testing.T) {
	names, group, err := Names("web", 3)
	if err != nil {
		t.Fatal(err)
	}
	if group != "web" || len(names) != 3 || names[0] != "web-1" || names[2] != "web-3" {
		t.Fatalf("%v %s", names, group)
	}
	one, group, err := Names("dev1", 1)
	if err != nil || group != "" || one[0] != "dev1" {
		t.Fatalf("%v %s %v", one, group, err)
	}

	desired := []Desired{
		{Name: "dev1", OS: "ubuntu:24.04", CPUs: "2", Memory: "2g", User: "lanbox", Mode: "light"},
		{Name: "web-1", Group: "web", OS: "rockylinux:9", CPUs: "1", Memory: "1g", User: "lanbox", Mode: "light"},
		{Name: "web-2", Group: "web", OS: "rockylinux:9", CPUs: "1", Memory: "1g", User: "lanbox", Mode: "light"},
	}
	actual := []Actual{
		{Name: "dev1", OS: "ubuntu:24.04", CPUs: "1", Memory: "2g", User: "lanbox", Mode: "light"},
		{Name: "web-1", Group: "web", OS: "ubuntu:24.04", CPUs: "1", Memory: "1g", User: "lanbox", Mode: "light"},
		{Name: "web-2", Group: "web", OS: "rockylinux:9", CPUs: "1", Memory: "1g", Packages: "vim", User: "lanbox", Mode: "light"},
		{Name: "web-3", Group: "web", OS: "rockylinux:9"},
		{Name: "other", OS: "alpine:latest"},
	}
	changes := Diff(desired, actual)
	got := map[string]Action{}
	for _, ch := range changes {
		got[ch.Name] = ch.Action
	}
	if got["dev1"] != ActionUpdate {
		t.Fatalf("dev1 %s", got["dev1"])
	}
	if got["web-1"] != ActionRecreate {
		t.Fatalf("web-1 %s", got["web-1"])
	}
	if got["web-2"] != ActionRecreate {
		t.Fatalf("web-2 %s", got["web-2"])
	}
	if got["web-3"] != ActionDelete {
		t.Fatalf("web-3 %s", got["web-3"])
	}
	if _, ok := got["other"]; ok {
		t.Fatal("unlisted instance should stay")
	}
}

func TestConfigRoundTripAndLabels(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Network: Network{Name: "lanbox_net", Driver: "ipvlan", Parent: "eth0", Subnet: "192.168.1.0/24", Gateway: "192.168.1.1", Range: "192.168.1.64/28", HostIP: "192.168.1.10"},
		Shim:    Shim{Enabled: true, IP: "192.168.1.78"},
		Reserve: []string{"192.168.1.78"},
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Network.Parent != "eth0" || loaded.Defaults.User != "lanbox" {
		t.Fatalf("%+v", loaded)
	}
	back, err := ConfigFromLabels(cfg.Network.Name, cfg.Labels())
	if err != nil {
		t.Fatal(err)
	}
	if back.Network.Range != cfg.Network.Range || back.Shim.IP != "192.168.1.78" {
		t.Fatalf("%+v", back)
	}
	if _, err := os.Stat(filepath.Join(dir, ConfigFile)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lab.yaml")
	body := []byte(`
instances:
  - name: dev1
    os: ubuntu:24.04
    cpus: 2
    memory: 2g
    gpus: all
    packages: [vim, git]
  - name: web
    os: rocky
    count: 3
    cpus: 1
    memory: 1g
    gpus: 1
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	lab, err := LoadLab(path)
	if err != nil {
		t.Fatal(err)
	}
	if lab.Instances[0].CPUs != "2" || lab.Instances[0].Memory != "2g" || lab.Instances[0].GPUs != "all" || lab.Instances[1].Count != 3 || lab.Instances[1].GPUs != "1" {
		t.Fatalf("%+v", lab.Instances)
	}
	expanded, err := Expand(lab.Instances[1], "lanbox", "light")
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 3 || expanded[2].Name != "web-3" || expanded[0].GPUs != "1" || expanded[2].GPUs != "1" {
		t.Fatalf("%+v", expanded)
	}
}

func TestGPUChangeDoesNotUpdate(t *testing.T) {
	desired := []Desired{{Name: "dev1", OS: "ubuntu:24.04", CPUs: "1", Memory: "1g", GPUs: "all", User: "lanbox", Mode: "light"}}
	actual := []Actual{{Name: "dev1", OS: "ubuntu:24.04", CPUs: "1", Memory: "1g", User: "lanbox", Mode: "light"}}
	if changes := Diff(desired, actual); len(changes) != 0 {
		t.Fatalf("gpus is create-time only, got %+v", changes)
	}
}
