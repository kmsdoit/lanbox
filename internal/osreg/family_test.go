package osreg

import (
	"strings"
	"testing"
)

func TestParseAndDetect(t *testing.T) {
	cases := []struct {
		body   string
		family string
	}{
		{"ID=ubuntu\nID_LIKE=debian\nVERSION_ID=24.04\n", FamilyDebian},
		{"ID=debian\nVERSION_ID=12\n", FamilyDebian},
		{"ID=\"rocky\"\nID_LIKE=\"rhel centos fedora\"\nVERSION_ID=\"9.3\"\n", FamilyRHEL},
		{"ID=fedora\nVERSION_ID=40\n", FamilyRHEL},
		{"ID=amzn\nID_LIKE=\"fedora\"\n", FamilyRHEL},
		{"ID=alpine\nVERSION_ID=3.21.0\n", FamilyAlpine},
		{"ID=\"opensuse-leap\"\nID_LIKE=\"opensuse suse\"\n", FamilySUSE},
		{"ID=arch\n", FamilyArch},
	}
	for _, tc := range cases {
		rel, err := Parse(strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		got, err := Detect(rel)
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		if got != tc.family {
			t.Fatalf("body %q family %s, want %s", tc.body, got, tc.family)
		}
	}
}

func TestDetectUnknown(t *testing.T) {
	rel, err := Parse(strings.NewReader("ID=windows\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Detect(rel); err == nil {
		t.Fatal("expected error")
	}
}

func TestEOL(t *testing.T) {
	if EOLWarning(Release{ID: "ubuntu", VersionID: "20.04"}) == "" {
		t.Fatal("ubuntu 20.04 should warn")
	}
	if EOLWarning(Release{ID: "ubuntu", VersionID: "24.04"}) != "" {
		t.Fatal("ubuntu 24.04 should not warn")
	}
	if EOLWarning(Release{ID: "debian", VersionID: "12"}) == "" {
		t.Fatal("debian 12 should warn as of 2026-10")
	}
}

func TestResolveAlias(t *testing.T) {
	reg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := reg.Resolve("rocky")
	if err != nil {
		t.Fatal(err)
	}
	if got != "rockylinux:9" {
		t.Fatalf("got %s", got)
	}
	got, err = reg.Resolve("ubuntu:22.04")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ubuntu:22.04" {
		t.Fatalf("got %s", got)
	}
	if _, err := reg.Resolve("no-such"); err == nil {
		t.Fatal("expected unknown alias error")
	}
}

func TestNormalizePackages(t *testing.T) {
	got, err := NormalizePackages(FamilyRHEL, []string{"iproute2", "vim", "iproute2"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "iproute,vim" {
		t.Fatalf("got %v", got)
	}
	if _, err := NormalizePackages(FamilyDebian, []string{"vim;rm"}); err == nil {
		t.Fatal("expected invalid package")
	}
}

func TestAddAlias(t *testing.T) {
	dir := t.TempDir()
	image, err := Add(dir, "lab", "myrepo/custom:1.0")
	if err != nil {
		t.Fatal(err)
	}
	if image != "myrepo/custom:1.0" {
		t.Fatalf("image %s", image)
	}
	reg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reg.Resolve("lab")
	if err != nil {
		t.Fatal(err)
	}
	if got != "myrepo/custom:1.0" {
		t.Fatalf("got %s", got)
	}
	ubuntu, err := reg.Resolve("ubuntu")
	if err != nil || ubuntu != "ubuntu:24.04" {
		t.Fatalf("ubuntu %s %v", ubuntu, err)
	}
}
