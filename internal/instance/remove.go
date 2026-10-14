package instance

import (
	"context"
	"fmt"

	"github.com/kmsdoit/lanbox/internal/dockerx"
	"github.com/kmsdoit/lanbox/internal/labels"
)

// RemoveRequest는 이름 또는 그룹으로 지울 대상을 고른다.
type RemoveRequest struct {
	Names []string
	Group string
}

// RemoveResult는 실제로 지운 것과 없는 이름, 남긴 홈 볼륨이다.
type RemoveResult struct {
	Removed []string `json:"removed"`
	Missing []string `json:"missing"`
	Volumes []string `json:"volumes_kept"`
}

// Select는 삭제 대상을 고른다. 관리하지 않는 컨테이너 이름은 오류다.
func Select(ctx context.Context, d *dockerx.Client, req RemoveRequest) ([]dockerx.Instance, []string, error) {
	all, err := d.ListInstances(ctx)
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]dockerx.Instance{}
	for _, inst := range all {
		byName[inst.Name] = inst
	}
	var picked []dockerx.Instance
	seen := map[string]struct{}{}
	add := func(inst dockerx.Instance) {
		if _, ok := seen[inst.Name]; ok {
			return
		}
		seen[inst.Name] = struct{}{}
		picked = append(picked, inst)
	}
	if req.Group != "" {
		for _, inst := range all {
			if inst.Group == req.Group {
				add(inst)
			}
		}
	}
	var missing []string
	for _, name := range req.Names {
		if inst, ok := byName[name]; ok {
			add(inst)
			continue
		}
		got, exists, err := d.InspectAny(ctx, name)
		if err != nil {
			return nil, nil, err
		}
		if exists && !labels.ManagedTrue(got) {
			return nil, nil, fmt.Errorf("%s 는 lanbox가 관리하지 않는 컨테이너입니다", name)
		}
		if exists && labels.ManagedTrue(got) && got[labels.Role] != labels.RoleInstance {
			return nil, nil, fmt.Errorf("%s 는 인스턴스가 아닙니다", name)
		}
		missing = append(missing, name)
	}
	return picked, missing, nil
}

// Remove는 고른 인스턴스 컨테이너를 지운다. 홈 볼륨은 남긴다.
func Remove(ctx context.Context, d *dockerx.Client, picked []dockerx.Instance) (RemoveResult, error) {
	var res RemoveResult
	vols, _ := d.ListVolumes(ctx)
	volSet := map[string]struct{}{}
	for _, v := range vols {
		volSet[v] = struct{}{}
	}
	for _, inst := range picked {
		if err := d.RemoveContainer(ctx, inst.ID); err != nil {
			return res, err
		}
		res.Removed = append(res.Removed, inst.Name)
		vol := "lanbox-home-" + inst.Name
		if _, ok := volSet[vol]; ok {
			res.Volumes = append(res.Volumes, vol)
		}
	}
	return res, nil
}
