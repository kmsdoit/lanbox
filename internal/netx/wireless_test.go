package netx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsWireless(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "wlan0", "wireless"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "eth0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsWireless(root, "wlan0") {
		t.Fatal("expected wireless")
	}
	if IsWireless(root, "eth0") {
		t.Fatal("eth0 is wired")
	}
}
