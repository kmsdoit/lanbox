package ipam

import "testing"

func TestAllocateRange(t *testing.T) {
	pool, err := New("192.168.0.240/28", []string{"192.168.0.254", "192.168.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	hosts := pool.Hosts()
	if hosts[0] != "192.168.0.241" || hosts[len(hosts)-1] != "192.168.0.254" {
		t.Fatalf("hosts %v", hosts)
	}
	if pool.Free() != 13 { // 241-254 is 14, one reserved
		t.Fatalf("free %d", pool.Free())
	}
	got, err := pool.Allocate(2)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "192.168.0.241" || got[1] != "192.168.0.242" {
		t.Fatalf("got %v", got)
	}
	if _, err := pool.AllocateFrom("192.168.0.254", 1); err == nil {
		t.Fatal("reserved address should fail")
	}
	seq, err := pool.AllocateFrom("192.168.0.250", 2)
	if err != nil {
		t.Fatal(err)
	}
	if seq[0] != "192.168.0.250" || seq[1] != "192.168.0.251" {
		t.Fatalf("seq %v", seq)
	}
}

func TestExhausted(t *testing.T) {
	pool, err := New("192.168.0.240/30", nil)
	if err != nil {
		t.Fatal(err)
	}
	// /30 usable hosts are the two middle addresses.
	if pool.Free() != 2 {
		t.Fatalf("free %d hosts %v", pool.Free(), pool.Hosts())
	}
	if _, err := pool.Allocate(3); err == nil {
		t.Fatal("expected shortage")
	}
}

func TestValidateNetwork(t *testing.T) {
	if err := ValidateNetwork("192.168.0.0/24", "192.168.0.1", "192.168.0.240/28"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNetwork("192.168.0.0/24", "10.0.0.1", "192.168.0.240/28"); err == nil {
		t.Fatal("gateway outside subnet")
	}
	if err := ValidateNetwork("192.168.1.0/24", "192.168.1.1", "192.168.0.240/28"); err == nil {
		t.Fatal("range outside subnet")
	}
}

func TestSuggestDriver(t *testing.T) {
	if SuggestDriver(true) != "ipvlan" || SuggestDriver(false) != "macvlan" {
		t.Fatal("driver suggestion")
	}
}
