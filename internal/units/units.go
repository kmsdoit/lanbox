package units

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/docker/go-units"
)

// ParseMemory는 2g, 512m 같은 값을 바이트로 바꾼다. 빈 문자열은 제한 없음(0)이다.
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" || s == "-" {
		return 0, nil
	}
	n, err := units.RAMInBytes(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("메모리 값이 올바르지 않습니다: %s", s)
	}
	return n, nil
}

// ParseGPUs는 --gpus 값을 DeviceRequest Count로 바꾼다.
// 빈 문자열은 요청 없음(0)이다. all은 Docker의 all(-1)이다.
// 0, 음수, all이 아닌 문자열은 오류다.
func ParseGPUs(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if s == "all" {
		return -1, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("GPU 값이 올바르지 않습니다: %s (all 또는 양의 정수)", s)
	}
	return n, nil
}

// ParseCPUs는 CPU 개수(소수 허용)를 읽는다. 빈 문자열은 제한 없음(0)이다.
func ParseCPUs(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" || s == "-" {
		return 0, nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("CPU 값이 올바르지 않습니다: %s", s)
	}
	return n, nil
}

// NanoCPUs는 docker NanoCPUs 단위로 바꾼다.
func NanoCPUs(cpus float64) int64 {
	if cpus <= 0 {
		return 0
	}
	return int64(math.Round(cpus * 1e9))
}

// FormatMemory는 바이트를 2g, 512m 형태로 보여 준다. 0은 "-"다.
func FormatMemory(n int64) string {
	if n <= 0 {
		return "-"
	}
	switch {
	case n%(1<<30) == 0:
		return fmt.Sprintf("%dg", n>>30)
	case n%(1<<20) == 0:
		return fmt.Sprintf("%dm", n>>20)
	case n%(1<<10) == 0:
		return fmt.Sprintf("%dk", n>>10)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// FormatCPUs는 CPU 개수를 문자열로 보여 준다. 0은 "-"다.
func FormatCPUs(n float64) string {
	if n <= 0 {
		return "-"
	}
	if n == math.Trunc(n) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

// CPUsFromNano는 NanoCPUs를 개수로 되돌린다.
func CPUsFromNano(n int64) float64 {
	if n <= 0 {
		return 0
	}
	return float64(n) / 1e9
}
