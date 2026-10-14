package builder

import (
	"strings"
	"testing"
)

func TestRenderFamilies(t *testing.T) {
	for _, family := range []string{"debian", "rhel", "alpine", "suse", "arch"} {
		body, err := Render(family, Data{Base: "example:1", User: "lanbox", PasswordAuth: "no"})
		if err != nil {
			t.Fatal(family, err)
		}
		for _, want := range []string{"FROM example:1", "lanbox", "PermitRootLogin no", "PasswordAuthentication no", "sshd"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %s\n%s", family, want, body)
			}
		}
		if family == "alpine" && strings.Contains(body, "/sbin/init") {
			t.Fatal("alpine template should stay on the light entrypoint")
		}
	}
	sys, err := Render("debian", Data{Base: "ubuntu:24.04", User: "lanbox", PasswordAuth: "yes", Systemd: true, Extra: "vim"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sys, "systemd") || !strings.Contains(sys, "/sbin/init") || !strings.Contains(sys, "vim") {
		t.Fatalf("systemd template:\n%s", sys)
	}
}

func TestHashStableAndDerivedRef(t *testing.T) {
	a, err := Hash("debian", "lanbox", []string{"vim"}, false, "light")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Hash("debian", "lanbox", []string{"vim"}, false, "light")
	if err != nil {
		t.Fatal(err)
	}
	if a != b || len(a) != 16 {
		t.Fatalf("hash %s %s", a, b)
	}
	c, err := Hash("debian", "lanbox", []string{"curl"}, false, "light")
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("packages should change the cache key")
	}
	ref, err := DerivedRef("ubuntu:24.04", a)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "lanbox/ubuntu-24.04:"+a {
		t.Fatalf("ref %s", ref)
	}
	ref, err = DerivedRef("opensuse/leap:15.6", a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ref, "lanbox/opensuse-leap-15.6:") {
		t.Fatalf("ref %s", ref)
	}
}

func TestRejectBadUser(t *testing.T) {
	if _, err := Render("debian", Data{Base: "ubuntu:24.04", User: "bad user", PasswordAuth: "no"}); err == nil {
		t.Fatal("expected invalid user")
	}
}
