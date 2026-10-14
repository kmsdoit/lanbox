// Package dockerx는 moby 클라이언트를 lanbox 라벨 규칙으로 감싼다.
package dockerx

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"runtime"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/kmsdoit/lanbox/internal/labels"
	"github.com/kmsdoit/lanbox/internal/units"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Client는 Docker 엔진 연결이다.
type Client struct {
	cli *client.Client
}

// New는 환경 변수(DOCKER_HOST 등)로 연결하고 API 버전을 협상한다.
func New(ctx context.Context) (*Client, error) {
	cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("Docker 클라이언트 생성: %w", err)
	}
	c := &Client{cli: cli}
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("Docker 데몬에 연결하지 못했습니다: %w", err)
	}
	return c, nil
}

// Close는 연결을 닫는다.
func (c *Client) Close() error {
	if c == nil || c.cli == nil {
		return nil
	}
	return c.cli.Close()
}

func managedFilters(role string) client.Filters {
	f := make(client.Filters).Add("label", labels.Managed+"=true")
	if role != "" {
		f = f.Add("label", labels.Role+"="+role)
	}
	return f
}

func hostPlatform() ocispec.Platform {
	return ocispec.Platform{OS: "linux", Architecture: runtime.GOARCH}
}

// Instance는 lanbox 컨테이너 하나다.
type Instance struct {
	ID           string
	Name         string
	Image        string
	OS           string
	Family       string
	User         string
	IP           string
	Status       string
	Group        string
	Mode         string
	Packages     string
	CPUs         float64
	Memory       int64
	CPUText      string
	MemoryText   string
	PasswordAuth string
}

// ListManagedContainers는 lanbox 라벨이 있는 컨테이너를 모두 찾는다.
func (c *Client) ListManagedContainers(ctx context.Context) ([]Instance, error) {
	return c.listContainers(ctx, managedFilters(""))
}

// ListInstances는 role=instance 인 컨테이너를 라벨로 찾는다.
func (c *Client) ListInstances(ctx context.Context) ([]Instance, error) {
	return c.listContainers(ctx, managedFilters(labels.RoleInstance))
}

func (c *Client) listContainers(ctx context.Context, filters client.Filters) ([]Instance, error) {
	res, err := c.cli.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("컨테이너 목록: %w", err)
	}
	var out []Instance
	for _, item := range res.Items {
		inst := instanceFromSummary(item)
		insp, err := c.cli.ContainerInspect(ctx, item.ID, client.ContainerInspectOptions{})
		if err == nil && insp.Container.HostConfig != nil {
			inst.CPUs = units.CPUsFromNano(insp.Container.HostConfig.NanoCPUs)
			inst.Memory = insp.Container.HostConfig.Memory
			inst.CPUText = units.FormatCPUs(inst.CPUs)
			inst.MemoryText = units.FormatMemory(inst.Memory)
			if inst.IP == "" {
				inst.IP = ipFromInspect(insp.Container.NetworkSettings)
			}
		}
		out = append(out, inst)
	}
	return out, nil
}

func instanceFromSummary(item container.Summary) Instance {
	name := ""
	if len(item.Names) > 0 {
		name = strings.TrimPrefix(item.Names[0], "/")
	}
	m := item.Labels
	inst := Instance{
		ID:           item.ID,
		Name:         name,
		Image:        item.Image,
		OS:           m[labels.OS],
		Family:       m[labels.Family],
		User:         m[labels.User],
		IP:           m[labels.IP],
		Status:       string(item.State),
		Group:        m[labels.Group],
		Mode:         m[labels.Mode],
		Packages:     m[labels.Packages],
		PasswordAuth: m[labels.PasswordAuth],
	}
	if inst.IP == "" && item.NetworkSettings != nil {
		inst.IP = ipFromEndpoints(item.NetworkSettings.Networks)
	}
	if inst.User == "" {
		inst.User = "lanbox"
	}
	return inst
}

func ipFromInspect(ns *container.NetworkSettings) string {
	if ns == nil {
		return ""
	}
	return ipFromEndpoints(ns.Networks)
}

func ipFromEndpoints(nets map[string]*network.EndpointSettings) string {
	for _, ep := range nets {
		if ep == nil {
			continue
		}
		if ep.IPAddress.IsValid() {
			return ep.IPAddress.String()
		}
		if ep.IPAMConfig != nil && ep.IPAMConfig.IPv4Address.IsValid() {
			return ep.IPAMConfig.IPv4Address.String()
		}
	}
	return ""
}

// Lookup는 이름으로 인스턴스를 찾는다. 없으면 nil.
func (c *Client) Lookup(ctx context.Context, name string) (*Instance, error) {
	all, err := c.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Name == name {
			return &all[i], nil
		}
	}
	return nil, nil
}

// InspectAny는 관리 여부와 상관없이 컨테이너를 조회한다.
func (c *Client) InspectAny(ctx context.Context, name string) (map[string]string, bool, error) {
	res, err := c.cli.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if res.Container.Config == nil {
		return nil, true, nil
	}
	return res.Container.Config.Labels, true, nil
}

// RemoveContainer는 컨테이너를 강제로 지운다. 없어도 성공이다.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	_, err := c.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("컨테이너 삭제(%s): %w", id, err)
	}
	return nil
}

// CreateSpec는 인스턴스 컨테이너를 만들 때 필요한 값이다.
type CreateSpec struct {
	Name       string
	Image      string
	Hostname   string
	Network    string
	IP         string
	CPUs       float64
	Memory     int64
	GPUs       int // 0이면 요청 없음. -1은 all. 양수는 개수.
	Labels     map[string]string
	Env        []string
	User       string
	Mode       string
	HomeVolume string
	DNS        []string
}

// instanceResources는 CPU, 메모리, GPU 요청을 HostConfig.Resources로 만든다.
// GPU를 생략하면 DeviceRequests를 넣지 않는다.
func instanceResources(spec CreateSpec) container.Resources {
	res := container.Resources{
		NanoCPUs: units.NanoCPUs(spec.CPUs),
		Memory:   spec.Memory,
	}
	if spec.Memory > 0 {
		res.MemorySwap = spec.Memory
	}
	if reqs := gpuDeviceRequests(spec.GPUs); len(reqs) > 0 {
		res.DeviceRequests = reqs
	}
	return res
}

// gpuDeviceRequests는 docker run --gpus와 같은 요청이다.
// count 0은 nil, -1은 all, 양수는 그 개수다. 장치를 나누지 않는다.
func gpuDeviceRequests(count int) []container.DeviceRequest {
	if count == 0 {
		return nil
	}
	return []container.DeviceRequest{{
		Driver:       "nvidia",
		Count:        count,
		Capabilities: [][]string{{"gpu"}},
	}}
}

// CreateInstance는 컨테이너를 만들고 시작한다.
func (c *Client) CreateInstance(ctx context.Context, spec CreateSpec) (string, error) {
	init := true
	host := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		Resources:     instanceResources(spec),
		NetworkMode:   container.NetworkMode(spec.Network),
	}
	if spec.Mode == "systemd" {
		host.Privileged = true
		host.CgroupnsMode = container.CgroupnsModeHost
		host.Tmpfs = map[string]string{
			"/run":      "",
			"/run/lock": "",
			"/tmp":      "exec",
		}
		host.Mounts = append(host.Mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: "/sys/fs/cgroup",
			Target: "/sys/fs/cgroup",
		})
	} else {
		host.Init = &init
	}
	if spec.HomeVolume != "" {
		host.Mounts = append(host.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: spec.HomeVolume,
			Target: "/home",
		})
	}
	for _, d := range spec.DNS {
		addr, err := parseAddr(d)
		if err != nil {
			return "", err
		}
		host.DNS = append(host.DNS, addr)
	}
	ip, err := parseAddr(spec.IP)
	if err != nil {
		return "", fmt.Errorf("컨테이너 IP: %w", err)
	}
	cfg := &container.Config{
		Image:    spec.Image,
		Hostname: spec.Hostname,
		Labels:   spec.Labels,
		Env:      spec.Env,
		User:     "",
	}
	res, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name:       spec.Name,
		Config:     cfg,
		HostConfig: host,
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				spec.Network: {
					IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: ip},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("컨테이너 생성(%s): %w", spec.Name, err)
	}
	if _, err := c.cli.ContainerStart(ctx, res.ID, client.ContainerStartOptions{}); err != nil {
		_ = c.RemoveContainer(ctx, res.ID)
		return "", fmt.Errorf("컨테이너 시작(%s): %w", spec.Name, err)
	}
	return res.ID, nil
}

// Start, Stop, Restart는 상태만 바꾼다.
func (c *Client) Start(ctx context.Context, id string) error {
	_, err := c.cli.ContainerStart(ctx, id, client.ContainerStartOptions{})
	return err
}

func (c *Client) Stop(ctx context.Context, id string) error {
	sec := 10
	_, err := c.cli.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &sec})
	return err
}

func (c *Client) Restart(ctx context.Context, id string) error {
	sec := 10
	_, err := c.cli.ContainerRestart(ctx, id, client.ContainerRestartOptions{Timeout: &sec})
	return err
}

// UpdateResources는 실행 중 자원 제한을 바꾼다. memory-swap은 memory와 같게 둔다.
func (c *Client) UpdateResources(ctx context.Context, id string, cpus float64, memory int64) error {
	res := &container.Resources{NanoCPUs: units.NanoCPUs(cpus), Memory: memory}
	if memory > 0 {
		res.MemorySwap = memory
	}
	_, err := c.cli.ContainerUpdate(ctx, id, client.ContainerUpdateOptions{Resources: res})
	if err != nil {
		return fmt.Errorf("자원 변경: %w", err)
	}
	return nil
}

// Commit은 현재 파일시스템을 이미지로 저장한다.
func (c *Client) Commit(ctx context.Context, id, ref string, imageLabels map[string]string) (string, error) {
	res, err := c.cli.ContainerCommit(ctx, id, client.ContainerCommitOptions{
		Reference: ref,
		Comment:   "lanbox snapshot",
		Config: &container.Config{
			Labels: imageLabels,
		},
	})
	if err != nil {
		return "", fmt.Errorf("스냅샷 생성: %w", err)
	}
	return res.ID, nil
}

// StatsSample은 한 차례 측정한 사용량이다.
type StatsSample struct {
	CPUPercent float64
	MemUsage   uint64
	MemLimit   uint64
}

// Stats는 직전 샘플을 포함해 한 번 측정한다.
func (c *Client) Stats(ctx context.Context, id string) (StatsSample, error) {
	res, err := c.cli.ContainerStats(ctx, id, client.ContainerStatsOptions{IncludePreviousSample: true})
	if err != nil {
		return StatsSample{}, err
	}
	defer res.Body.Close()
	var s container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
		return StatsSample{}, fmt.Errorf("사용량 읽기: %w", err)
	}
	return StatsSample{
		CPUPercent: cpuPercent(s),
		MemUsage:   s.MemoryStats.Usage,
		MemLimit:   s.MemoryStats.Limit,
	}, nil
}

func cpuPercent(s container.StatsResponse) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	online := float64(s.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	if cpuDelta > 0 && sysDelta > 0 && online > 0 {
		return (cpuDelta / sysDelta) * online * 100
	}
	return 0
}

// ReadOSRelease는 임시 컨테이너로 /etc/os-release를 읽는다.
func (c *Client) ReadOSRelease(ctx context.Context, image string) (string, error) {
	name := "lanbox-probe-" + randHex(4)
	created, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: name,
		Config: &container.Config{
			Image:      image,
			Entrypoint: []string{"/bin/cat"},
			Cmd:        []string{"/etc/os-release"},
			Labels: map[string]string{
				labels.Managed: "true",
				labels.Role:    labels.RoleProbe,
			},
		},
		HostConfig: &container.HostConfig{NetworkMode: "none"},
	})
	if err != nil {
		return "", fmt.Errorf("계열 감지용 컨테이너 생성: %w", err)
	}
	defer func() { _ = c.RemoveContainer(context.Background(), created.ID) }()

	wait := c.cli.ContainerWait(ctx, created.ID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	if _, err := c.cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("계열 감지용 컨테이너 시작: %w", err)
	}
	select {
	case err := <-wait.Error:
		if err != nil {
			return "", err
		}
	case res := <-wait.Result:
		if res.StatusCode != 0 {
			return "", fmt.Errorf("/etc/os-release를 읽지 못했습니다 (exit %d). --family로 지정하세요", res.StatusCode)
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}
	logs, err := c.cli.ContainerLogs(ctx, created.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}
	defer logs.Close()
	var buf strings.Builder
	if _, err := stdcopy.StdCopy(&buf, io.Discard, logs); err != nil {
		return "", fmt.Errorf("os-release 로그: %w", err)
	}
	return buf.String(), nil
}

// ImageLocal는 로컬에 이미지가 있는지와 아키텍처, 크기를 돌려준다.
type ImageInfo struct {
	Present      bool
	Architecture string
	OS           string
	Size         int64
	Labels       map[string]string
}

func (c *Client) InspectImage(ctx context.Context, ref string) (ImageInfo, error) {
	res, err := c.cli.ImageInspect(ctx, ref)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return ImageInfo{}, nil
		}
		return ImageInfo{}, err
	}
	var lbl map[string]string
	if res.Config != nil {
		lbl = res.Config.Labels
	}
	return ImageInfo{
		Present:      true,
		Architecture: normalizeArch(res.Architecture),
		OS:           res.Os,
		Size:         res.Size,
		Labels:       lbl,
	}, nil
}

// SupportsArch는 로컬 이미지 또는 레지스트리 매니페스트에 호스트 아키텍처가 있는지 본다.
// known이 false면 확인하지 못한 것이다.
func (c *Client) SupportsArch(ctx context.Context, ref string) (ok bool, known bool, detail string, err error) {
	info, err := c.InspectImage(ctx, ref)
	if err != nil {
		return false, false, "", err
	}
	host := runtime.GOARCH
	if info.Present {
		if info.Architecture == "" || info.Architecture == host {
			return true, true, "", nil
		}
		return false, true, fmt.Sprintf("로컬 이미지 아키텍처가 %s 이고 호스트는 %s 입니다", info.Architecture, host), nil
	}
	dist, err := c.cli.DistributionInspect(ctx, ref, client.DistributionInspectOptions{})
	if err != nil {
		return false, false, "", nil
	}
	if len(dist.Platforms) == 0 {
		return false, false, "", nil
	}
	var have []string
	for _, p := range dist.Platforms {
		arch := normalizeArch(p.Architecture)
		have = append(have, p.OS+"/"+arch)
		if p.OS == "linux" && arch == host {
			return true, true, "", nil
		}
	}
	return false, true, fmt.Sprintf("호스트 %s 용 매니페스트가 없습니다 (있음: %s)", host, strings.Join(have, ", ")), nil
}

func normalizeArch(a string) string {
	switch a {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return a
	}
}

// Pull은 호스트 아키텍처로 이미지를 받는다.
func (c *Client) Pull(ctx context.Context, ref string, stderr io.Writer) error {
	rc, err := c.cli.ImagePull(ctx, ref, client.ImagePullOptions{Platforms: []ocispec.Platform{hostPlatform()}})
	if err != nil {
		return fmt.Errorf("이미지를 받지 못했습니다(%s). 이름과 아키텍처를 확인하세요: %w", ref, err)
	}
	defer rc.Close()
	return readStream(rc, stderr, "이미지 받기 실패")
}

// Build는 메모리 tar 컨텍스트로 파생 이미지를 만든다.
func (c *Client) Build(ctx context.Context, contextReader io.Reader, tag string, imageLabels map[string]string, stderr io.Writer) error {
	res, err := c.cli.ImageBuild(ctx, contextReader, client.ImageBuildOptions{
		Tags:      []string{tag},
		Remove:    true,
		Labels:    imageLabels,
		Platforms: []ocispec.Platform{hostPlatform()},
		Version:   "1",
	})
	if err != nil {
		return fmt.Errorf("이미지 빌드 요청: %w", err)
	}
	defer res.Body.Close()
	return readStream(res.Body, stderr, "이미지 빌드 실패")
}

func readStream(r io.Reader, stderr io.Writer, prefix string) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	var lastErr string
	for sc.Scan() {
		line := sc.Bytes()
		var msg jsonstream.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			if stderr != nil {
				fmt.Fprintln(stderr, string(line))
			}
			continue
		}
		if msg.Error != nil && msg.Error.Message != "" {
			lastErr = msg.Error.Message
			continue
		}
		if msg.Stream != "" && stderr != nil {
			fmt.Fprint(stderr, msg.Stream)
		} else if msg.Status != "" && stderr != nil {
			if msg.ID != "" {
				fmt.Fprintf(stderr, "%s: %s\n", msg.ID, msg.Status)
			} else {
				fmt.Fprintln(stderr, msg.Status)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	if lastErr != "" {
		return fmt.Errorf("%s: %s", prefix, lastErr)
	}
	return nil
}

// ManagedImage는 파생 이미지 또는 스냅샷이다.
type ManagedImage struct {
	ID     string
	Tags   []string
	Labels map[string]string
	Size   int64
	UsedBy int64
}

// ListImages는 lanbox 라벨이 있는 이미지를 돌려준다.
func (c *Client) ListImages(ctx context.Context) ([]ManagedImage, error) {
	res, err := c.cli.ImageList(ctx, client.ImageListOptions{
		Filters: managedFilters(""),
	})
	if err != nil {
		return nil, fmt.Errorf("이미지 목록: %w", err)
	}
	var out []ManagedImage
	for _, item := range res.Items {
		out = append(out, ManagedImage{
			ID:     item.ID,
			Tags:   item.RepoTags,
			Labels: item.Labels,
			Size:   item.Size,
			UsedBy: item.Containers,
		})
	}
	return out, nil
}

// RemoveImage는 이미지를 지운다.
func (c *Client) RemoveImage(ctx context.Context, ref string) error {
	_, err := c.cli.ImageRemove(ctx, ref, client.ImageRemoveOptions{PruneChildren: true})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("이미지 삭제(%s): %w", ref, err)
	}
	return nil
}

// ImageUsed는 어떤 컨테이너든 이 이미지를 쓰면 true다.
func (c *Client) ImageUsed(ctx context.Context, imageID string, tags []string) (bool, error) {
	res, err := c.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return false, err
	}
	tagSet := map[string]struct{}{}
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}
	for _, ct := range res.Items {
		if ct.ImageID == imageID {
			return true, nil
		}
		if _, ok := tagSet[ct.Image]; ok {
			return true, nil
		}
	}
	return false, nil
}

// NetworkRecord는 lanbox 네트워크다.
type NetworkRecord struct {
	ID      string
	Name    string
	Driver  string
	Labels  map[string]string
	Options map[string]string
}

func (c *Client) ListNetworks(ctx context.Context) ([]NetworkRecord, error) {
	res, err := c.cli.NetworkList(ctx, client.NetworkListOptions{Filters: managedFilters(labels.RoleNetwork)})
	if err != nil {
		return nil, fmt.Errorf("네트워크 목록: %w", err)
	}
	var out []NetworkRecord
	for _, n := range res.Items {
		out = append(out, NetworkRecord{ID: n.ID, Name: n.Name, Driver: n.Driver, Labels: n.Labels, Options: n.Options})
	}
	return out, nil
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	_, err := c.cli.NetworkRemove(ctx, id, client.NetworkRemoveOptions{})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("네트워크 삭제: %w", err)
	}
	return nil
}

// EnsureNetwork는 같은 설정이면 그대로 두고, 없으면 만든다. 설정이 다르면 오류다.
func (c *Client) EnsureNetwork(ctx context.Context, name, driver, parent, subnet, gateway, ipRange string, netLabels map[string]string) (string, bool, error) {
	existing, err := c.cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	if err == nil {
		gotParent := existing.Network.Options["parent"]
		gotDriver := existing.Network.Driver
		if gotDriver != driver || gotParent != parent {
			return "", false, fmt.Errorf("네트워크 %s 가 이미 있지만 설정이 다릅니다 (driver=%s parent=%s). 이름을 바꾸거나 purge 하세요", name, gotDriver, gotParent)
		}
		return existing.Network.ID, false, nil
	}
	if !cerrdefs.IsNotFound(err) && !isNoSuchNetwork(err) {
		return "", false, err
	}
	sn, err := parsePrefix(subnet)
	if err != nil {
		return "", false, fmt.Errorf("서브넷: %w", err)
	}
	rg, err := parsePrefix(ipRange)
	if err != nil {
		return "", false, fmt.Errorf("IP 범위: %w", err)
	}
	gw, err := parseAddr(gateway)
	if err != nil {
		return "", false, fmt.Errorf("게이트웨이: %w", err)
	}
	v4 := true
	opts := map[string]string{"parent": parent}
	if driver == "ipvlan" {
		opts["ipvlan_mode"] = "l2"
	}
	created, err := c.cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver:     driver,
		EnableIPv4: &v4,
		IPAM: &network.IPAM{Config: []network.IPAMConfig{{
			Subnet:  sn,
			IPRange: rg,
			Gateway: gw,
		}}},
		Options: opts,
		Labels:  netLabels,
	})
	if err != nil {
		return "", false, fmt.Errorf("네트워크 생성: %w", err)
	}
	return created.ID, true, nil
}

func isNoSuchNetwork(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

// VolumeNames는 관리 볼륨 이름이다.
func (c *Client) ListVolumes(ctx context.Context) ([]string, error) {
	res, err := c.cli.VolumeList(ctx, client.VolumeListOptions{Filters: managedFilters(labels.RoleVolume)})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, v := range res.Items {
		names = append(names, v.Name)
	}
	return names, nil
}

// EnsureVolume은 볼륨이 없으면 만들고, 새로 만들었는지 돌려준다.
func (c *Client) EnsureVolume(ctx context.Context, name string, volLabels map[string]string) (created bool, err error) {
	list, err := c.cli.VolumeList(ctx, client.VolumeListOptions{
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return false, err
	}
	for _, v := range list.Items {
		if v.Name == name {
			if !labels.ManagedTrue(v.Labels) {
				return false, fmt.Errorf("볼륨 %s 는 이미 있지만 lanbox 소유가 아닙니다", name)
			}
			return false, nil
		}
	}
	_, err = c.cli.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name, Labels: volLabels})
	if err != nil {
		return false, fmt.Errorf("볼륨 생성(%s): %w", name, err)
	}
	return true, nil
}

// RemoveVolume은 볼륨을 지운다. 없어도 성공이다.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeRemove(ctx, name, client.VolumeRemoveOptions{Force: true})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("볼륨 삭제(%s): %w", name, err)
	}
	return nil
}

// Info는 엔진 정보를 돌려준다.
func (c *Client) Info(ctx context.Context) (DockerInfo, error) {
	res, err := c.cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return DockerInfo{}, err
	}
	return DockerInfo{
		Root:          res.Info.DockerRootDir,
		Kernel:        res.Info.KernelVersion,
		OS:            res.Info.OperatingSystem,
		Architecture:  res.Info.Architecture,
		NCPU:          res.Info.NCPU,
		MemTotal:      res.Info.MemTotal,
		ServerVersion: res.Info.ServerVersion,
	}, nil
}

// DockerInfo는 doctor와 info에 보여줄 엔진 정보다.
type DockerInfo struct {
	Root          string
	Kernel        string
	OS            string
	Architecture  string
	NCPU          int
	MemTotal      int64
	ServerVersion string
}

func parsePrefix(s string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return p.Masked(), nil
}

func parseAddr(s string) (netip.Addr, error) {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, err
	}
	return a.Unmap(), nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
