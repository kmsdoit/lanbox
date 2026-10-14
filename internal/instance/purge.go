package instance

import (
	"context"

	"github.com/kmsdoit/lanbox/internal/dockerx"
	"github.com/kmsdoit/lanbox/internal/labels"
	"github.com/kmsdoit/lanbox/internal/netx"
)

// PurgePlan은 purge가 지울 리소스 목록이다.
type PurgePlan struct {
	Containers []string `json:"containers"`
	Volumes    []string `json:"volumes"`
	Networks   []string `json:"networks"`
	Images     []string `json:"images"`
	Shim       bool     `json:"shim"`
}

// Empty는 지울 것이 없는지 본다.
func (p PurgePlan) Empty() bool {
	return len(p.Containers)+len(p.Volumes)+len(p.Networks)+len(p.Images) == 0 && !p.Shim
}

// PlanPurge는 라벨이 있는 리소스만 모은다. images가 false면 이미지 목록은 비운다.
func PlanPurge(ctx context.Context, d *dockerx.Client, images bool) (PurgePlan, error) {
	var plan PurgePlan
	inst, err := d.ListManagedContainers(ctx)
	if err != nil {
		return plan, err
	}
	for _, i := range inst {
		plan.Containers = append(plan.Containers, i.Name)
	}
	vols, err := d.ListVolumes(ctx)
	if err != nil {
		return plan, err
	}
	plan.Volumes = vols
	nets, err := d.ListNetworks(ctx)
	if err != nil {
		return plan, err
	}
	for _, n := range nets {
		plan.Networks = append(plan.Networks, n.Name)
	}
	if images {
		imgs, err := d.ListImages(ctx)
		if err != nil {
			return plan, err
		}
		for _, img := range imgs {
			if img.Labels[labels.Role] != labels.RoleImage && img.Labels[labels.Role] != labels.RoleSnapshot {
				continue
			}
			if len(img.Tags) > 0 {
				plan.Images = append(plan.Images, img.Tags...)
			} else {
				plan.Images = append(plan.Images, img.ID)
			}
		}
	}
	ok, _ := netx.ShimExists()
	plan.Shim = ok
	return plan, nil
}

// ApplyPurge는 컨테이너, 볼륨, 네트워크, 심, 선택적 이미지 순으로 지운다.
func ApplyPurge(ctx context.Context, d *dockerx.Client, images bool) (PurgePlan, error) {
	plan, err := PlanPurge(ctx, d, images)
	if err != nil {
		return plan, err
	}
	inst, err := d.ListManagedContainers(ctx)
	if err != nil {
		return plan, err
	}
	for _, i := range inst {
		if err := d.RemoveContainer(ctx, i.ID); err != nil {
			return plan, err
		}
	}
	for _, name := range plan.Volumes {
		if err := d.RemoveVolume(ctx, name); err != nil {
			return plan, err
		}
	}
	nets, err := d.ListNetworks(ctx)
	if err != nil {
		return plan, err
	}
	for _, n := range nets {
		if err := d.RemoveNetwork(ctx, n.ID); err != nil {
			return plan, err
		}
	}
	if plan.Shim {
		if err := netx.ShimDown(); err != nil {
			return plan, err
		}
	}
	if images {
		imgs, err := d.ListImages(ctx)
		if err != nil {
			return plan, err
		}
		for _, img := range imgs {
			ref := img.ID
			if len(img.Tags) > 0 {
				ref = img.Tags[0]
			}
			if err := d.RemoveImage(ctx, ref); err != nil {
				return plan, err
			}
		}
	}
	return plan, nil
}
