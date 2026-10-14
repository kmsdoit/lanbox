package dockerx

import (
	"reflect"
	"testing"

	"github.com/kmsdoit/lanbox/internal/units"
	"github.com/moby/moby/api/types/container"
)

func TestGPUDeviceRequests(t *testing.T) {
	if got := gpuDeviceRequests(0); got != nil {
		t.Fatalf("count 0 should omit DeviceRequests, got %+v", got)
	}

	all := gpuDeviceRequests(-1)
	one := gpuDeviceRequests(1)
	again := gpuDeviceRequests(1)
	if !reflect.DeepEqual(one, again) {
		t.Fatalf("same count must be the same request, not a split: %+v %+v", one, again)
	}

	wantAll := []container.DeviceRequest{{
		Driver:       "nvidia",
		Count:        -1,
		Capabilities: [][]string{{"gpu"}},
	}}
	wantOne := []container.DeviceRequest{{
		Driver:       "nvidia",
		Count:        1,
		Capabilities: [][]string{{"gpu"}},
	}}
	if !reflect.DeepEqual(all, wantAll) {
		t.Fatalf("all: %+v", all)
	}
	if !reflect.DeepEqual(one, wantOne) {
		t.Fatalf("one: %+v", one)
	}

	none := instanceResources(CreateSpec{CPUs: 2, Memory: 2 << 30})
	if none.DeviceRequests != nil {
		t.Fatalf("omitted gpus leaked into HostConfig: %+v", none.DeviceRequests)
	}
	if none.NanoCPUs != units.NanoCPUs(2) || none.Memory != 2<<30 || none.MemorySwap != 2<<30 {
		t.Fatalf("cpu/memory changed: %+v", none)
	}

	with := instanceResources(CreateSpec{CPUs: 1, Memory: 1 << 30, GPUs: 1})
	if !reflect.DeepEqual(with.DeviceRequests, wantOne) {
		t.Fatalf("HostConfig DeviceRequests: %+v", with.DeviceRequests)
	}
	if with.NanoCPUs != units.NanoCPUs(1) || with.MemorySwap != with.Memory {
		t.Fatalf("cpu/memory with gpus: %+v", with)
	}
}
