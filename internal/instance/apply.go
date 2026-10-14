package instance

import (
	"context"
	"fmt"
	"io"

	"github.com/kmsdoit/lanbox/internal/dockerx"
	"github.com/kmsdoit/lanbox/internal/osreg"
	"github.com/kmsdoit/lanbox/internal/spec"
	"github.com/kmsdoit/lanbox/internal/units"
)

// Apply는 랩 스펙과 현재 상태를 비교해 차이만 반영한다.
// 스펙에 없는 다른 인스턴스는 지우지 않는다. 그룹 개수가 줄면 그 그룹의 남는 번호만 지운다.
func Apply(ctx context.Context, d *dockerx.Client, cfg spec.Config, reg *osreg.Registry, lab spec.Lab, dryRun bool, stderr io.Writer) ([]spec.Change, error) {
	existing, err := d.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	byName := map[string]dockerx.Instance{}
	var actual []spec.Actual
	for _, inst := range existing {
		byName[inst.Name] = inst
		actual = append(actual, spec.Actual{
			Name:     inst.Name,
			Group:    inst.Group,
			OS:       inst.OS,
			CPUs:     inst.CPUText,
			Memory:   inst.MemoryText,
			Packages: inst.Packages,
			IP:       inst.IP,
			User:     inst.User,
			Mode:     inst.Mode,
		})
	}
	var desired []spec.Desired
	for _, inst := range lab.Instances {
		image, err := reg.Resolve(inst.OS)
		if err != nil {
			return nil, err
		}
		expanded, err := spec.Expand(inst, cfg.Defaults.User, cfg.Defaults.Mode)
		if err != nil {
			return nil, err
		}
		for _, dsr := range expanded {
			dsr.OS = image
			if cur, ok := byName[dsr.Name]; ok && cur.Family != "" && cur.OS == image {
				norm, nerr := osreg.NormalizePackages(cur.Family, dsr.Packages)
				if nerr != nil {
					return nil, nerr
				}
				dsr.Packages = norm
			}
			desired = append(desired, dsr)
		}
	}
	changes := spec.Diff(desired, actual)
	if dryRun {
		return changes, nil
	}
	for _, ch := range changes {
		switch ch.Action {
		case spec.ActionDelete:
			picked, _, err := Select(ctx, d, RemoveRequest{Names: []string{ch.Name}})
			if err != nil {
				return changes, err
			}
			if _, err := Remove(ctx, d, picked); err != nil {
				return changes, err
			}
		case spec.ActionRecreate:
			fmt.Fprintf(stderr, "%s 를 다시 만듭니다. 쓰기 레이어는 삭제됩니다. (%s)\n", ch.Name, ch.Reason)
			picked, _, err := Select(ctx, d, RemoveRequest{Names: []string{ch.Name}})
			if err != nil {
				return changes, err
			}
			if _, err := Remove(ctx, d, picked); err != nil {
				return changes, err
			}
			if err := applyCreate(ctx, d, cfg, reg, ch.Desired, stderr); err != nil {
				return changes, err
			}
		case spec.ActionUpdate:
			cur := byName[ch.Name]
			cpus := cur.CPUs
			mem := cur.Memory
			if ch.Desired.CPUs != "" {
				cpus, err = units.ParseCPUs(ch.Desired.CPUs)
				if err != nil {
					return changes, err
				}
			}
			if ch.Desired.Memory != "" {
				mem, err = units.ParseMemory(ch.Desired.Memory)
				if err != nil {
					return changes, err
				}
			}
			if err := d.UpdateResources(ctx, cur.ID, cpus, mem); err != nil {
				return changes, err
			}
		case spec.ActionCreate:
			if err := applyCreate(ctx, d, cfg, reg, ch.Desired, stderr); err != nil {
				return changes, err
			}
		}
	}
	return changes, nil
}

func applyCreate(ctx context.Context, d *dockerx.Client, cfg spec.Config, reg *osreg.Registry, dsr spec.Desired, stderr io.Writer) error {
	_, _, err := Create(ctx, d, cfg, reg, CreateRequest{
		BaseName:   dsr.Name,
		Count:      1,
		OS:         dsr.OS,
		User:       dsr.User,
		Packages:   dsr.Packages,
		CPUs:       dsr.CPUs,
		Memory:     dsr.Memory,
		GPUs:       dsr.GPUs,
		StartIP:    dsr.IP,
		Hostname:   dsr.Hostname,
		Mode:       dsr.Mode,
		HomeVolume: dsr.HomeVolume,
	}, stderr)
	return err
}
