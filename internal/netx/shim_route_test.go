package netx

import (
	"net"
	"testing"
)

func TestPlanShimRoutesContainerOnly(t *testing.T) {
	plan, err := PlanShimRoutes("192.168.1.0/24", "192.168.1.64/28", "192.168.1.70")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Subnet.String() != "192.168.1.0/24" {
		t.Fatalf("subnet %s", plan.Subnet)
	}
	if plan.Container.String() != "192.168.1.64/28" {
		t.Fatalf("container %s", plan.Container)
	}
	if plan.Src.String() != "192.168.1.70" {
		t.Fatalf("src %s", plan.Src)
	}
	if !dropBroadShimRoute(plan.Subnet, plan.Subnet) {
		t.Fatal("same-prefix subnet route on the shim must be removed")
	}
	if dropBroadShimRoute(plan.Container, plan.Subnet) {
		t.Fatal("longer container prefix must stay")
	}
	if dropBroadShimRoute(nil, plan.Subnet) {
		t.Fatal("default route must not be selected for deletion")
	}
	if !plan.Container.Contains(net.ParseIP("192.168.1.65")) || !plan.Container.Contains(net.ParseIP("192.168.1.78")) {
		t.Fatal("container addresses must match the shim route")
	}
	if plan.Container.Contains(net.ParseIP("192.168.1.10")) || plan.Container.Contains(net.ParseIP("192.168.1.1")) {
		t.Fatal("host LAN and gateway must stay on the parent prefix")
	}
}

func TestPlanShimRoutesRejectsSamePrefix(t *testing.T) {
	if _, err := PlanShimRoutes("192.168.1.0/24", "192.168.1.0/24", "192.168.1.70"); err == nil {
		t.Fatal("same prefix must not be installed on the shim")
	}
	if _, err := PlanShimRoutes("192.168.1.0/24", "192.168.0.0/16", "192.168.1.70"); err == nil {
		t.Fatal("broader prefix must not be installed on the shim")
	}
}

func TestPlanShimRoutesRejectsRangeOutsideSubnet(t *testing.T) {
	if _, err := PlanShimRoutes("192.168.1.0/24", "10.0.0.0/28", "192.168.1.70"); err == nil {
		t.Fatal("range outside the subnet")
	}
	if _, err := PlanShimRoutes("192.168.1.0/24", "192.168.1.64/28", "10.0.0.8"); err == nil {
		t.Fatal("shim IP outside the subnet")
	}
}

func TestPlanShimRoutesMasksContainerPrefix(t *testing.T) {
	plan, err := PlanShimRoutes("192.168.1.0/24", "192.168.1.78/28", "192.168.1.70")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Container.String() != "192.168.1.64/28" {
		t.Fatalf("masked container %s", plan.Container)
	}
}
