//go:build !linux

package netx

import "fmt"

// LinkInfo는 기본 경로 인터페이스와 그 주소다.
type LinkInfo struct {
	Name            string
	Wireless        bool
	Subnet          string
	Gateway         string
	HostIP          string
	SuggestedDriver string
}

// Detect는 Linux에서만 기본 경로와 부모 인터페이스를 찾는다.
func Detect() (LinkInfo, error) {
	return LinkInfo{}, fmt.Errorf("인터페이스 자동 감지는 Linux에서만 동작합니다. --parent, --subnet, --gateway, --driver를 지정하세요")
}

// InterfaceExists는 Linux에서만 인터페이스를 확인한다.
func InterfaceExists(name string) error {
	return fmt.Errorf("인터페이스 확인은 Linux에서만 동작합니다")
}
