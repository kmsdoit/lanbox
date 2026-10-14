//go:build linux

package preflight

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

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

// HostResources는 /proc/meminfo와 디스크 여유를 읽는다.
func HostResources(dockerRoot string) (Resources, error) {
	mem, err := memTotal()
	if err != nil {
		return Resources{}, err
	}
	path := dockerRoot
	if path == "" {
		path = "/var/lib/docker"
	}
	free, total, usedPath, err := disk(path)
	if err != nil {
		free, total, usedPath, err = disk("/")
		if err != nil {
			return Resources{}, err
		}
	}
	return Resources{
		MemTotal:  mem,
		CPUs:      runtime.NumCPU(),
		DiskFree:  free,
		DiskTotal: total,
		DiskPath:  usedPath,
	}, nil
}

func memTotal() (int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("meminfo 읽기: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return kb * 1024, nil
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("MemTotal 항목이 없습니다")
}

func disk(path string) (free, total int64, used string, err error) {
	var st unix.Statfs_t
	if err = unix.Statfs(path, &st); err != nil {
		return 0, 0, path, err
	}
	bsize := int64(st.Bsize)
	return int64(st.Bavail) * bsize, int64(st.Blocks) * bsize, path, nil
}
