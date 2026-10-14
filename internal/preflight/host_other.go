//go:build !linux && !darwin

package preflight

import "runtime"

// Resources는 호스트에서 읽은 자원이다.
type Resources struct {
	MemTotal  int64
	CPUs      int
	DiskFree  int64
	DiskTotal int64
	DiskPath  string
}

// HostResources는 이 OS에서 메모리와 디스크를 읽지 않는다.
func HostResources(dockerRoot string) (Resources, error) {
	return Resources{CPUs: runtime.NumCPU(), DiskPath: dockerRoot}, nil
}
