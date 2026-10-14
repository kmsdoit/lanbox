//go:build !linux

package netx

import "fmt"

const ShimName = "lanbox-shim"

// ShimUp은 Linux에서 ipvlan/macvlan 심을 만든다.
func ShimUp(parent, driver, ip, subnet, ipRange string) error {
	return fmt.Errorf("shim은 Linux에서만 만들 수 있습니다")
}

// ShimDown은 shim 인터페이스를 지운다.
func ShimDown() error {
	return fmt.Errorf("shim은 Linux에서만 지울 수 있습니다")
}

// ShimExists는 shim이 있는지 본다.
func ShimExists() (bool, error) {
	return false, nil
}
