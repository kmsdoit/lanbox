// Package preflight는 만들기 전에 자원, IP, 아키텍처, 계열을 검사한다.
package preflight

import "fmt"

const (
	// MemReserve는 호스트에 남겨 둘 OS 예약 메모리(2GB)다.
	MemReserve = 2 << 30
	// InstanceDisk는 인스턴스 쓰기 레이어의 예상 사용량이다.
	InstanceDisk = 256 << 20
	// GuessBase는 로컬에 없는 베이스 이미지의 대략적인 크기다.
	GuessBase = 400 << 20
	// GuessDerived는 아직 없는 파생 이미지의 대략적인 추가 크기다.
	GuessDerived = 250 << 20
)

// Level은 통과, 경고, 차단이다.
type Level string

const (
	Pass  Level = "pass"
	Warn  Level = "warn"
	Block Level = "block"
)

// Item은 검사 한 줄이다.
type Item struct {
	Name    string `json:"name"`
	Level   Level  `json:"level"`
	Message string `json:"message"`
}

// Report는 사전 검증 결과다.
type Report struct {
	Items []Item `json:"items"`
}

func (r *Report) add(level Level, name, msg string) {
	r.Items = append(r.Items, Item{Name: name, Level: level, Message: msg})
}

// Blocked는 차단 항목이 있는지 본다.
func (r Report) Blocked() bool {
	for _, it := range r.Items {
		if it.Level == Block {
			return true
		}
	}
	return false
}

// Error는 차단 사유를 한 문장으로 모은다.
func (r Report) Error() string {
	var parts []string
	for _, it := range r.Items {
		if it.Level == Block {
			parts = append(parts, it.Name+": "+it.Message)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	out := "사전 검증에서 차단되었습니다"
	for _, p := range parts {
		out += "\n- " + p
	}
	return out
}

// Existing은 이미 떠 있는 lanbox 인스턴스의 제한값이다.
type Existing struct {
	Memory int64
	CPUs   float64
}

// Input은 검사에 필요한 사실이다. 값을 모으는 쪽과 판정하는 쪽을 나눈다.
type Input struct {
	MemTotal       int64
	DiskFree       int64
	DiskTotal      int64
	CPUs           int
	Existing       []Existing
	NewCount       int
	NewMemoryEach  int64
	NewCPUsEach    float64
	NeedIPs        int
	FreeIPs        int
	ArchKnown      bool
	ArchOK         bool
	ArchDetail     string
	Family         string
	FamilyKnown    bool
	EOL            string
	HasSSHKey      bool
	HasPassword    bool
	BaseMissing    bool
	DerivedMissing bool
	Wireless       bool
	Driver         string
	ARPConflicts   []string
	ARPSkipped     string
}

// Check는 계획서의 판정표대로 통과, 경고, 차단을 매긴다.
func Check(in Input) Report {
	var r Report
	checkMemory(&r, in)
	checkCPU(&r, in)
	checkUnlimited(&r, in)
	checkIP(&r, in)
	checkArch(&r, in)
	checkFamily(&r, in)
	checkSSH(&r, in)
	checkDisk(&r, in)
	checkParent(&r, in)
	checkARP(&r, in)
	if len(r.Items) == 0 {
		r.add(Pass, "preflight", "확인할 신규 리소스가 없습니다")
	}
	return r
}

func checkMemory(r *Report, in Input) {
	if in.MemTotal <= 0 || in.NewCount == 0 {
		return
	}
	var used int64
	for _, e := range in.Existing {
		used += e.Memory
	}
	var add int64
	if in.NewMemoryEach > 0 {
		add = in.NewMemoryEach * int64(in.NewCount)
	}
	remain := in.MemTotal - used - add
	if remain < MemReserve {
		r.add(Block, "memory", fmt.Sprintf("남는 메모리가 OS 예약분 2GB보다 작습니다 (남음 %s)", bytes(remain)))
		return
	}
	if float64(remain) < float64(in.MemTotal)*0.20 {
		r.add(Warn, "memory", fmt.Sprintf("남는 메모리가 전체의 20%% 미만입니다 (남음 %s)", bytes(remain)))
		return
	}
	r.add(Pass, "memory", fmt.Sprintf("남는 메모리 %s", bytes(remain)))
}

func checkCPU(r *Report, in Input) {
	if in.CPUs <= 0 || in.NewCount == 0 {
		return
	}
	sum := in.NewCPUsEach * float64(in.NewCount)
	for _, e := range in.Existing {
		sum += e.CPUs
	}
	if sum > float64(in.CPUs) {
		r.add(Warn, "cpu", fmt.Sprintf("CPU 합계 %.2f 가 코어 %d개를 넘습니다 (오버커밋)", sum, in.CPUs))
		return
	}
	r.add(Pass, "cpu", fmt.Sprintf("CPU 합계 %.2f / %d", sum, in.CPUs))
}

func checkUnlimited(r *Report, in Input) {
	n := 0
	for _, e := range in.Existing {
		if e.Memory == 0 || e.CPUs == 0 {
			n++
		}
	}
	if in.NewCount > 0 && (in.NewMemoryEach == 0 || in.NewCPUsEach == 0) {
		n += in.NewCount
	}
	if n > 0 {
		r.add(Warn, "unlimited", fmt.Sprintf("자원 제한이 없는 인스턴스가 %d개라 합계를 모두 계산할 수 없습니다", n))
	}
}

func checkIP(r *Report, in Input) {
	if in.NeedIPs <= 0 {
		return
	}
	if in.FreeIPs < in.NeedIPs {
		r.add(Block, "ip", fmt.Sprintf("빈 IP가 %d개 필요하지만 %d개 남았습니다", in.NeedIPs, in.FreeIPs))
		return
	}
	r.add(Pass, "ip", fmt.Sprintf("빈 IP %d개 (필요 %d개)", in.FreeIPs, in.NeedIPs))
}

func checkArch(r *Report, in Input) {
	if !in.ArchKnown {
		if in.NewCount > 0 {
			r.add(Warn, "arch", "이미지 아키텍처를 확인하지 못했습니다. 생성 시 다시 확인합니다")
		}
		return
	}
	if !in.ArchOK {
		detail := in.ArchDetail
		if detail == "" {
			detail = "호스트 아키텍처용 매니페스트가 없습니다"
		}
		r.add(Block, "arch", detail)
		return
	}
	r.add(Pass, "arch", "호스트 아키텍처와 이미지가 맞습니다")
}

func checkFamily(r *Report, in Input) {
	if in.NewCount == 0 {
		return
	}
	if !in.FamilyKnown {
		r.add(Warn, "family", "OS 계열은 실행 시 /etc/os-release로 감지합니다")
		return
	}
	if in.EOL != "" {
		r.add(Warn, "family", in.EOL)
		return
	}
	r.add(Pass, "family", "계열 "+in.Family)
}

func checkSSH(r *Report, in Input) {
	if in.NewCount == 0 {
		return
	}
	if !in.HasSSHKey && !in.HasPassword {
		r.add(Warn, "ssh", "SSH 공개키와 비밀번호가 없어 접속할 수 없습니다")
		return
	}
	r.add(Pass, "ssh", "접속 수단이 있습니다")
}

func checkDisk(r *Report, in Input) {
	if in.DiskTotal <= 0 || in.NewCount == 0 {
		return
	}
	need := int64(in.NewCount) * InstanceDisk
	if in.BaseMissing {
		need += GuessBase
	}
	if in.DerivedMissing {
		need += GuessDerived
	}
	if in.DiskFree < need {
		r.add(Block, "disk", fmt.Sprintf("디스크 여유가 %s 필요한데 %s 남았습니다", bytes(need), bytes(in.DiskFree)))
		return
	}
	remain := in.DiskFree - need
	if float64(remain) < float64(in.DiskTotal)*0.10 {
		r.add(Warn, "disk", fmt.Sprintf("디스크 여유가 전체의 10%% 미만입니다 (남음 %s)", bytes(remain)))
		return
	}
	r.add(Pass, "disk", fmt.Sprintf("디스크 여유 %s", bytes(in.DiskFree)))
}

func checkParent(r *Report, in Input) {
	if in.Wireless && in.Driver == "macvlan" {
		r.add(Block, "parent", "무선 인터페이스에는 macvlan을 쓸 수 없습니다. ipvlan을 사용하세요")
		return
	}
	if in.Wireless && in.Driver == "ipvlan" {
		r.add(Warn, "parent", "Wi-Fi 위 ipvlan은 불안정할 수 있습니다. 가능하면 유선 macvlan을 쓰세요")
	}
}

func checkARP(r *Report, in Input) {
	if len(in.ARPConflicts) > 0 {
		r.add(Block, "arp", "LAN에서 이미 응답하는 IP가 있습니다: "+join(in.ARPConflicts))
		return
	}
	if in.ARPSkipped != "" && in.NeedIPs > 0 {
		r.add(Warn, "arp", in.ARPSkipped)
	}
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func bytes(n int64) string {
	if n < 0 {
		return fmt.Sprintf("-%s", bytes(-n))
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGT"[exp])
}
