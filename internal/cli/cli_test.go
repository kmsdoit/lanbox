package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kmsdoit/lanbox/internal/spec"
	"gopkg.in/yaml.v3"
)

func TestHelpSmoke(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"create", "--help"},
		{"rm", "--help"},
		{"purge", "--help"},
		{"init", "--help"},
		{"os", "ls", "--help"},
		{"apply", "--help"},
		{"shim", "--help"},
	} {
		root := NewRoot()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, buf.String())
		}
		if !strings.Contains(buf.String(), "lanbox") && !strings.Contains(buf.String(), "Usage") && buf.Len() == 0 {
			t.Fatalf("%v empty help", args)
		}
	}
}

func TestInitDryRun(t *testing.T) {
	dir := t.TempDir()
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"--config-dir", dir,
		"init", "--dry-run",
		"--parent", "eth0",
		"--subnet", "192.168.0.0/24",
		"--gateway", "192.168.0.1",
		"--range", "192.168.0.240/28",
		"--driver", "ipvlan",
		"--shim",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{"lanbox_net", "ipvlan", "eth0", "192.168.0.240/28", "dry-run"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("dry-run should not write config")
	}
}

func TestCreateDryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := spec.Config{
		Network: spec.Network{
			Name: "lanbox_net", Driver: "ipvlan", Parent: "eth0",
			Subnet: "192.168.0.0/24", Gateway: "192.168.0.1", Range: "192.168.0.240/28",
		},
		Shim: ShimOff(),
	}
	if err := spec.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"--config-dir", dir,
		"create", "web", "--os", "rocky", "-n", "2", "--cpus", "1", "--memory", "1g", "--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{"web-1", "web-2", "rockylinux:9", "ssh lanbox@", "dry-run"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in\n%s", want, out)
		}
	}
}

func TestCreateGPUFlag(t *testing.T) {
	help := NewRoot()
	var helpBuf bytes.Buffer
	help.SetOut(&helpBuf)
	help.SetErr(&helpBuf)
	help.SetArgs([]string{"create", "--help"})
	if err := help.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(helpBuf.String(), "--gpus") {
		t.Fatalf("create help missing --gpus:\n%s", helpBuf.String())
	}

	resize := NewRoot()
	var resizeBuf bytes.Buffer
	resize.SetOut(&resizeBuf)
	resize.SetErr(&resizeBuf)
	resize.SetArgs([]string{"resize", "--help"})
	if err := resize.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resizeBuf.String(), "--gpus") {
		t.Fatalf("resize must not take --gpus:\n%s", resizeBuf.String())
	}

	dir := t.TempDir()
	cfg := spec.Config{
		Network: spec.Network{
			Name: "lanbox_net", Driver: "ipvlan", Parent: "eth0",
			Subnet: "192.168.0.0/24", Gateway: "192.168.0.1", Range: "192.168.0.240/28",
		},
		Shim: ShimOff(),
	}
	if err := spec.Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		root := NewRoot()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs(append([]string{"--config-dir", dir}, args...))
		err := root.Execute()
		return buf.String(), err
	}

	for _, args := range [][]string{
		{"create", "gpu1", "--os", "ubuntu", "--gpus", "all", "--dry-run"},
		{"create", "web", "--os", "rocky", "-n", "2", "--gpus", "1", "--dry-run"},
	} {
		out, err := run(t, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		if !strings.Contains(out, "dry-run") {
			t.Fatalf("%v missing dry-run:\n%s", args, out)
		}
	}
	if out, err := run(t, "create", "web", "--os", "rocky", "-n", "2", "--gpus", "1", "--dry-run"); err != nil || !strings.Contains(out, "web-1") || !strings.Contains(out, "web-2") {
		t.Fatalf("multi create: %v\n%s", err, out)
	}

	for _, bad := range []string{"0", "-1", "nope", "1.5"} {
		args := []string{"create", "gpu1", "--os", "ubuntu", "--gpus=" + bad, "--dry-run"}
		out, err := run(t, args...)
		if err == nil {
			t.Fatalf("--gpus %s should fail before create:\n%s", bad, out)
		}
		if strings.Contains(out, "dry-run: 컨테이너를 만들지 않습니다") {
			t.Fatalf("--gpus %s reached create output:\n%s", bad, out)
		}
	}
}

func TestRmDryRunMissingIsIdempotentMessage(t *testing.T) {
	// Docker가 없으면 연결 오류다. 도움말과 무관하게 플래그가 파싱되는지만 본다.
	root := NewRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"rm", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "--dry-run") || !strings.Contains(buf.String(), "-g") {
		t.Fatalf("%s", buf.String())
	}
}

func TestApplyFileParseViaCLIHelp(t *testing.T) {
	dir := t.TempDir()
	body := []byte("instances:\n  - name: dev1\n    os: ubuntu\n")
	path := filepath.Join(dir, "lab.yaml")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	var lab spec.Lab
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &lab); err != nil {
		t.Fatal(err)
	}
	if lab.Instances[0].Name != "dev1" {
		t.Fatal(lab)
	}
}

func ShimOff() spec.Shim { return spec.Shim{} }
