//go:build darwin

package preflight

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// Resources는 호스트에서 읽은 자원이다.
type Resources struct {
	MemTotal  int64
	CPUs      int
	DiskFree  int64
	DiskTotal int64
	DiskPath  string
}

// HostResources는 macOS에서 메모리와 디스크를 읽는다.
// 대상 서버는 Linux지만, 로컬 검증과 dry-run에 쓴다.
func HostResources(dockerRoot string) (Resources, error) {
	mem, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return Resources{}, err
	}
	path := dockerRoot
	if path == "" {
		path = "/"
	}
	free, total, used, err := disk(path)
	if err != nil {
		free, total, used, err = disk("/")
		if err != nil {
			return Resources{}, err
		}
	}
	return Resources{
		MemTotal:  int64(mem),
		CPUs:      runtime.NumCPU(),
		DiskFree:  free,
		DiskTotal: total,
		DiskPath:  used,
	}, nil
}

func disk(path string) (free, total int64, used string, err error) {
	var st unix.Statfs_t
	if err = unix.Statfs(path, &st); err != nil {
		return 0, 0, path, err
	}
	bsize := int64(st.Bsize)
	return int64(st.Bavail) * bsize, int64(st.Blocks) * bsize, path, nil
}
