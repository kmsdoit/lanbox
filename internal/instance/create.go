// Package instance는 생성, 삭제, 선언형 적용의 실행 순서다.
package instance

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/kmsdoit/lanbox/internal/builder"
	"github.com/kmsdoit/lanbox/internal/dockerx"
	"github.com/kmsdoit/lanbox/internal/ipam"
	"github.com/kmsdoit/lanbox/internal/labels"
	"github.com/kmsdoit/lanbox/internal/netx"
	"github.com/kmsdoit/lanbox/internal/osreg"
	"github.com/kmsdoit/lanbox/internal/preflight"
	"github.com/kmsdoit/lanbox/internal/spec"
	"github.com/kmsdoit/lanbox/internal/units"
	"net/netip"
	"time"
)

// CreateRequest는 create와 apply가 공유하는 입력이다.
type CreateRequest struct {
	BaseName   string
	Count      int
	OS         string
	Family     string
	User       string
	PublicKey  string
	Password   string
	Packages   []string
	CPUs       string
	Memory     string
	GPUs       string
	StartIP    string
	Hostname   string
	Mode       string
	HomeVolume bool
	From       string
	DryRun     bool
}

// Created는 만들어진 인스턴스다.
type Created struct {
	Name  string `json:"name"`
	IP    string `json:"ip"`
	User  string `json:"user"`
	Image string `json:"image"`
	SSH   string `json:"ssh"`
}

// Create는 검증 뒤에 인스턴스를 만든다. 중간에 실패하면 이번 실행에서 만든 것만 롤백한다.
func Create(ctx context.Context, d *dockerx.Client, cfg spec.Config, reg *osreg.Registry, req CreateRequest, stderr io.Writer) ([]Created, preflight.Report, error) {
	if req.Count < 1 {
		req.Count = 1
	}
	if req.User == "" {
		req.User = cfg.Defaults.User
	}
	if req.Mode == "" {
		req.Mode = cfg.Defaults.Mode
	}
	if req.Mode != "light" && req.Mode != "systemd" {
		return nil, preflight.Report{}, fmt.Errorf("--mode는 light 또는 systemd 여야 합니다")
	}
	if req.Family != "" && !osreg.KnownFamily(req.Family) {
		return nil, preflight.Report{}, fmt.Errorf("지원하지 않는 --family %s (가능: %s)", req.Family, strings.Join(osreg.Families(), ", "))
	}
	names, group, err := spec.Names(req.BaseName, req.Count)
	if err != nil {
		return nil, preflight.Report{}, err
	}
	if req.OS == "" && req.From == "" {
		return nil, preflight.Report{}, fmt.Errorf("--os 또는 --from이 필요합니다")
	}

	var image string
	if req.From != "" {
		image = req.From
	} else {
		image, err = reg.Resolve(req.OS)
		if err != nil {
			return nil, preflight.Report{}, err
		}
	}
	osLabel := image
	if req.OS != "" && req.From == "" {
		osLabel = image
	}

	cpuEach, err := units.ParseCPUs(req.CPUs)
	if err != nil {
		return nil, preflight.Report{}, err
	}
	memEach, err := units.ParseMemory(req.Memory)
	if err != nil {
		return nil, preflight.Report{}, err
	}
	gpuCount, err := units.ParseGPUs(req.GPUs)
	if err != nil {
		return nil, preflight.Report{}, err
	}

	var existing []dockerx.Instance
	if d != nil {
		existing, err = d.ListInstances(ctx)
		if err != nil {
			if !req.DryRun {
				return nil, preflight.Report{}, err
			}
			fmt.Fprintln(stderr, "경고: Docker에서 기존 인스턴스를 읽지 못했습니다. 없는 것으로 보고 계획을 출력합니다.")
			existing = nil
		}
	} else if !req.DryRun {
		return nil, preflight.Report{}, fmt.Errorf("Docker 데몬에 연결해야 합니다")
	}

	for _, name := range names {
		for _, ex := range existing {
			if ex.Name == name {
				return nil, preflight.Report{}, fmt.Errorf("인스턴스 %s 가 이미 있습니다", name)
			}
		}
	}

	reserved := append([]string{}, cfg.Reserve...)
	reserved = append(reserved, cfg.Network.Gateway, cfg.Shim.IP, cfg.Network.HostIP)
	pool, err := ipam.New(cfg.Network.Range, reserved)
	if err != nil {
		return nil, preflight.Report{}, err
	}
	for _, ex := range existing {
		_ = pool.Use(ex.IP)
	}
	var ips []string
	if req.StartIP != "" {
		ips, err = pool.AllocateFrom(req.StartIP, len(names))
	} else {
		ips, err = pool.Allocate(len(names))
	}
	if err != nil {
		return nil, preflight.Report{}, err
	}

	family := req.Family
	var release osreg.Release
	baseMissing := true
	derivedMissing := true
	archKnown, archOK := false, false
	archDetail := ""
	if req.From != "" && d != nil {
		info, ierr := d.InspectImage(ctx, req.From)
		if ierr != nil {
			return nil, preflight.Report{}, ierr
		}
		if !info.Present && !req.DryRun {
			return nil, preflight.Report{}, fmt.Errorf("스냅샷 이미지가 없습니다: %s", req.From)
		}
		baseMissing = !info.Present
		derivedMissing = false
		if info.Present {
			archKnown = true
			archOK = info.Architecture == "" || info.Architecture == goArch()
			if !archOK {
				archDetail = fmt.Sprintf("스냅샷 아키텍처 %s", info.Architecture)
			}
			if family == "" {
				family = info.Labels[labels.Family]
			}
			if osLabel == req.From || req.OS == "" {
				if v := info.Labels[labels.OS]; v != "" {
					osLabel = v
				}
			}
		}
	} else if d != nil && req.From == "" {
		ok, known, detail, aerr := d.SupportsArch(ctx, image)
		if aerr != nil {
			return nil, preflight.Report{}, aerr
		}
		archKnown, archOK, archDetail = known, ok, detail
		info, ierr := d.InspectImage(ctx, image)
		if ierr != nil {
			return nil, preflight.Report{}, ierr
		}
		baseMissing = !info.Present
		if family != "" {
			norm, _ := osreg.NormalizePackages(family, req.Packages)
			hash, herr := builder.Hash(family, req.User, norm, req.Password != "", req.Mode)
			if herr == nil {
				ref, rerr := builder.DerivedRef(image, hash)
				if rerr == nil {
					dimg, derr := d.InspectImage(ctx, ref)
					if derr == nil {
						derivedMissing = !dimg.Present
					}
				}
			}
		}
	}

	host, herr := preflight.HostResources("")
	if herr != nil && stderr != nil {
		fmt.Fprintf(stderr, "경고: 호스트 자원을 읽지 못했습니다: %v\n", herr)
	}
	if d != nil && !req.DryRun {
		if info, ierr := d.Info(ctx); ierr == nil && info.Root != "" {
			if again, aerr := preflight.HostResources(info.Root); aerr == nil {
				host = again
			}
		}
	}

	var conflicts []string
	arpSkipped := ""
	if cfg.Network.Parent != "" {
		for _, ip := range ips {
			addr, perr := netip.ParseAddr(ip)
			if perr != nil {
				continue
			}
			used, aerr := netx.ARPProbe(cfg.Network.Parent, addr, 1200*time.Millisecond)
			if aerr != nil {
				arpSkipped = "ARP 충돌 검사를 건너뛰었습니다: " + aerr.Error()
				break
			}
			if used {
				conflicts = append(conflicts, ip)
			}
		}
	}

	in := preflight.Input{
		MemTotal:       host.MemTotal,
		DiskFree:       host.DiskFree,
		DiskTotal:      host.DiskTotal,
		CPUs:           host.CPUs,
		NewCount:       len(names),
		NewMemoryEach:  memEach,
		NewCPUsEach:    cpuEach,
		NeedIPs:        len(names),
		FreeIPs:        pool.Free(),
		ArchKnown:      archKnown,
		ArchOK:         archOK,
		ArchDetail:     archDetail,
		Family:         family,
		FamilyKnown:    family != "",
		HasSSHKey:      req.PublicKey != "",
		HasPassword:    req.Password != "",
		BaseMissing:    baseMissing && req.From == "",
		DerivedMissing: derivedMissing && req.From == "",
		Wireless:       netx.IsWireless("", cfg.Network.Parent),
		Driver:         cfg.Network.Driver,
		ARPConflicts:   conflicts,
		ARPSkipped:     arpSkipped,
	}
	for _, ex := range existing {
		in.Existing = append(in.Existing, preflight.Existing{Memory: ex.Memory, CPUs: ex.CPUs})
	}
	report := preflight.Check(in)
	if report.Blocked() {
		return nil, report, fmt.Errorf("%s", report.Error())
	}

	if req.DryRun {
		return dryCreated(names, ips, group, req, osLabel), report, nil
	}
	if d == nil {
		return nil, report, fmt.Errorf("Docker 데몬에 연결해야 합니다")
	}

	runtimeImage := image
	pkgs := req.Packages
	if req.From == "" {
		if baseMissing {
			fmt.Fprintf(stderr, "이미지 받는 중: %s\n", image)
			if err := d.Pull(ctx, image, stderr); err != nil {
				return nil, report, err
			}
		}
		info, err := d.InspectImage(ctx, image)
		if err != nil {
			return nil, report, err
		}
		if info.Present && info.Architecture != "" && info.Architecture != goArch() {
			return nil, report, fmt.Errorf("이미지 아키텍처가 %s 이고 호스트는 %s 입니다", info.Architecture, goArch())
		}
		if family == "" {
			raw, err := d.ReadOSRelease(ctx, image)
			if err != nil {
				return nil, report, err
			}
			release, err = osreg.Parse(strings.NewReader(raw))
			if err != nil {
				return nil, report, err
			}
			family, err = osreg.Detect(release)
			if err != nil {
				return nil, report, err
			}
			if msg := osreg.EOLWarning(release); msg != "" {
				fmt.Fprintf(stderr, "경고: %s\n", msg)
			}
		}
		if req.Mode == "systemd" && family == osreg.FamilyAlpine {
			return nil, report, fmt.Errorf("alpine 계열은 systemd 모드를 지원하지 않습니다. light 모드를 사용하세요")
		}
		norm, err := osreg.NormalizePackages(family, req.Packages)
		if err != nil {
			return nil, report, err
		}
		pkgs = norm
		hash, err := builder.Hash(family, req.User, pkgs, req.Password != "", req.Mode)
		if err != nil {
			return nil, report, err
		}
		ref, err := builder.DerivedRef(image, hash)
		if err != nil {
			return nil, report, err
		}
		dimg, err := d.InspectImage(ctx, ref)
		if err != nil {
			return nil, report, err
		}
		if dimg.Present {
			fmt.Fprintf(stderr, "파생 이미지 캐시 사용: %s\n", ref)
		} else {
			fmt.Fprintf(stderr, "파생 이미지 빌드: %s (계열 %s)\n", ref, family)
			df, err := builder.Render(family, builder.Data{
				Base:         image,
				User:         req.User,
				Extra:        strings.Join(pkgs, " "),
				Systemd:      req.Mode == "systemd",
				PasswordAuth: yesNo(req.Password != ""),
			})
			if err != nil {
				return nil, report, err
			}
			tar, err := builder.ContextTar(df)
			if err != nil {
				return nil, report, err
			}
			imgLabels := map[string]string{
				labels.Managed:      "true",
				labels.Role:         labels.RoleImage,
				labels.OS:           osLabel,
				labels.Family:       family,
				labels.User:         req.User,
				labels.Base:         image,
				labels.Template:     hash,
				labels.Packages:     strings.Join(pkgs, ","),
				labels.Mode:         req.Mode,
				labels.PasswordAuth: yesNo(req.Password != ""),
			}
			if err := d.Build(ctx, tar, ref, imgLabels, stderr); err != nil {
				return nil, report, fmt.Errorf("%s 계열 이미지 빌드 실패 (패키지: %s): %w", family, empty(strings.Join(pkgs, ","), "없음"), err)
			}
		}
		runtimeImage = ref
	} else if req.From != "" && len(req.Packages) > 0 {
		fmt.Fprintln(stderr, "경고: --from 스냅샷에는 --packages를 다시 설치하지 않습니다.")
	}

	rb := &dockerx.Rollback{}
	var made []Created
	for i, name := range names {
		hostName := hostnameOf(name, req.Hostname, i+1, req.Count)
		volName := ""
		if req.HomeVolume {
			volName = "lanbox-home-" + name
			created, verr := d.EnsureVolume(ctx, volName, map[string]string{
				labels.Managed: "true",
				labels.Role:    labels.RoleVolume,
				labels.Name:    name,
			})
			if verr != nil {
				_ = rb.Undo(ctx, d)
				return nil, report, verr
			}
			if created {
				rb.AddVolume(volName)
			}
		}
		env := []string{"LANBOX_USER=" + req.User}
		if req.PublicKey != "" {
			env = append(env, "LANBOX_SSH_PUBKEY="+req.PublicKey)
		}
		if req.Password != "" {
			env = append(env, "LANBOX_PASSWORD="+req.Password)
		}
		// GPU는 컨테이너 HostConfig다. 이미지 해시와 이미지 라벨에는 넣지 않는다.
		// -n 이어도 나누지 않고 각 컨테이너에 같은 DeviceRequest를 넣는다.
		id, cerr := d.CreateInstance(ctx, dockerx.CreateSpec{
			Name:       name,
			Image:      runtimeImage,
			Hostname:   hostName,
			Network:    cfg.Network.Name,
			IP:         ips[i],
			CPUs:       cpuEach,
			Memory:     memEach,
			GPUs:       gpuCount,
			User:       req.User,
			Mode:       req.Mode,
			HomeVolume: volName,
			DNS:        dnsOf(cfg),
			Env:        env,
			Labels: map[string]string{
				labels.Managed:      "true",
				labels.Role:         labels.RoleInstance,
				labels.Name:         name,
				labels.Group:        group,
				labels.OS:           osLabel,
				labels.Family:       family,
				labels.User:         req.User,
				labels.IP:           ips[i],
				labels.CPUs:         req.CPUs,
				labels.Memory:       req.Memory,
				labels.GPUs:         req.GPUs,
				labels.Mode:         req.Mode,
				labels.Packages:     strings.Join(pkgs, ","),
				labels.Base:         image,
				labels.PasswordAuth: yesNo(req.Password != ""),
			},
		})
		if cerr != nil {
			_ = rb.Undo(ctx, d)
			return nil, report, fmt.Errorf("%s 생성 실패, 이번 실행분을 롤백했습니다: %w", name, cerr)
		}
		rb.AddContainer(id)
		made = append(made, Created{Name: name, IP: ips[i], User: req.User, Image: osLabel, SSH: sshCommand(req.User, ips[i])})
	}
	return made, report, nil
}

func dryCreated(names, ips []string, group string, req CreateRequest, osLabel string) []Created {
	var out []Created
	for i, name := range names {
		out = append(out, Created{Name: name, IP: ips[i], User: req.User, Image: osLabel, SSH: sshCommand(req.User, ips[i])})
	}
	_ = group
	return out
}

func sshCommand(user, ip string) string {
	return "ssh " + user + "@" + ip
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func empty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func hostnameOf(name, override string, index, count int) string {
	h := override
	if h == "" {
		h = name
	} else if count > 1 {
		h = fmt.Sprintf("%s-%d", override, index)
	}
	var b strings.Builder
	for _, r := range strings.ToLower(h) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "lanbox"
	}
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}

func dnsOf(cfg spec.Config) []string {
	if cfg.Network.Gateway == "" {
		return nil
	}
	return []string{cfg.Network.Gateway}
}

func goArch() string {
	return arch()
}
