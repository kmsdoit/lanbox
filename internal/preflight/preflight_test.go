package preflight

import "testing"

func TestMemoryBlockAndWarn(t *testing.T) {
	block := Check(Input{
		MemTotal: 4 << 30, NewCount: 1, NewMemoryEach: 3 << 30, CPUs: 4, NewCPUsEach: 1,
		NeedIPs: 1, FreeIPs: 4, ArchKnown: true, ArchOK: true, Family: "debian", FamilyKnown: true,
		HasSSHKey: true, DiskFree: 20 << 30, DiskTotal: 100 << 30,
	})
	if !block.Blocked() {
		t.Fatalf("expected memory block: %+v", block.Items)
	}
	warn := Check(Input{
		MemTotal: 16 << 30, NewCount: 1, NewMemoryEach: 13 << 30, CPUs: 8, NewCPUsEach: 1,
		NeedIPs: 1, FreeIPs: 4, ArchKnown: true, ArchOK: true, Family: "debian", FamilyKnown: true,
		HasSSHKey: true, DiskFree: 50 << 30, DiskTotal: 100 << 30,
	})
	if warn.Blocked() {
		t.Fatalf("should warn not block: %+v", warn.Items)
	}
	if !hasLevel(warn, "memory", Warn) {
		t.Fatalf("missing memory warn: %+v", warn.Items)
	}
}

func TestCPUOvercommitAndUnlimited(t *testing.T) {
	r := Check(Input{
		MemTotal: 32 << 30, NewCount: 2, NewMemoryEach: 1 << 30, CPUs: 4, NewCPUsEach: 3,
		NeedIPs: 2, FreeIPs: 8, ArchKnown: true, ArchOK: true, Family: "rhel", FamilyKnown: true,
		HasSSHKey: true, DiskFree: 40 << 30, DiskTotal: 100 << 30,
	})
	if !hasLevel(r, "cpu", Warn) {
		t.Fatalf("%+v", r.Items)
	}
	r = Check(Input{
		MemTotal: 32 << 30, NewCount: 1, CPUs: 4,
		NeedIPs: 1, FreeIPs: 8, ArchKnown: true, ArchOK: true, Family: "alpine", FamilyKnown: true,
		HasSSHKey: true, DiskFree: 40 << 30, DiskTotal: 100 << 30,
	})
	if !hasLevel(r, "unlimited", Warn) {
		t.Fatalf("%+v", r.Items)
	}
}

func TestIPArchParentSSH(t *testing.T) {
	r := Check(Input{
		MemTotal: 16 << 30, NewCount: 3, NewMemoryEach: 1 << 30, CPUs: 8, NewCPUsEach: 1,
		NeedIPs: 3, FreeIPs: 1, ArchKnown: true, ArchOK: false, ArchDetail: "arm64 없음",
		FamilyKnown: false, DiskFree: 1 << 20, DiskTotal: 10 << 30, Wireless: true, Driver: "macvlan",
	})
	if !r.Blocked() {
		t.Fatal("expected blocks")
	}
	for _, name := range []string{"ip", "arch", "disk", "parent"} {
		if !hasLevel(r, name, Block) {
			t.Fatalf("missing block %s: %+v", name, r.Items)
		}
	}
	if !hasLevel(r, "ssh", Warn) {
		t.Fatalf("missing ssh warn: %+v", r.Items)
	}
}

func TestEOLAndARP(t *testing.T) {
	r := Check(Input{
		MemTotal: 16 << 30, NewCount: 1, NewMemoryEach: 1 << 30, CPUs: 4, NewCPUsEach: 1,
		NeedIPs: 1, FreeIPs: 4, ArchKnown: true, ArchOK: true, Family: "debian", FamilyKnown: true,
		EOL: "Ubuntu 18.04 EOL", HasSSHKey: true, DiskFree: 40 << 30, DiskTotal: 100 << 30,
		ARPConflicts: []string{"192.168.0.241"},
	})
	if !hasLevel(r, "family", Warn) || !hasLevel(r, "arp", Block) {
		t.Fatalf("%+v", r.Items)
	}
}

func hasLevel(r Report, name string, level Level) bool {
	for _, it := range r.Items {
		if it.Name == name && it.Level == level {
			return true
		}
	}
	return false
}
