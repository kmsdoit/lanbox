package spec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kmsdoit/lanbox/internal/units"
)

// Action은 apply가 반영할 변경 종류다.
type Action string

const (
	ActionCreate   Action = "create"
	ActionDelete   Action = "delete"
	ActionUpdate   Action = "update"
	ActionRecreate Action = "recreate"
)

// Actual은 라벨로 읽은 현재 인스턴스다.
type Actual struct {
	Name     string
	Group    string
	OS       string
	CPUs     string
	Memory   string
	Packages string
	IP       string
	User     string
	Mode     string
}

// Desired는 스펙을 인스턴스 이름으로 펼친 결과다.
type Desired struct {
	Name       string
	Group      string
	OS         string
	CPUs       string
	Memory     string
	GPUs       string
	Packages   []string
	IP         string
	User       string
	Mode       string
	Hostname   string
	HomeVolume bool
	BaseName   string
	Index      int
}

// Change는 한 인스턴스에 대한 변경이다.
type Change struct {
	Action  Action
	Name    string
	Reason  string
	Desired Desired
}

// Expand는 count를 이름으로 펼친다. count가 1이면 이름 그대로, 2 이상이면 name-1..N.
func Expand(inst LabInstance, defaultUser, defaultMode string) ([]Desired, error) {
	names, group, err := Names(inst.Name, inst.Count)
	if err != nil {
		return nil, err
	}
	user := inst.User
	if user == "" {
		user = defaultUser
	}
	mode := inst.Mode
	if mode == "" {
		mode = defaultMode
	}
	if mode == "" {
		mode = DefaultMode
	}
	var out []Desired
	for i, name := range names {
		host := inst.Hostname
		if host == "" {
			host = name
		} else if inst.Count > 1 {
			host = fmt.Sprintf("%s-%d", inst.Hostname, i+1)
		}
		ip := ""
		if inst.IP != "" {
			if inst.Count > 1 && i > 0 {
				return nil, fmt.Errorf("%s는 count가 %d인데 ip가 하나입니다. count 1이거나 ip를 비우세요", inst.Name, inst.Count)
			}
			ip = inst.IP
		}
		out = append(out, Desired{
			Name:       name,
			Group:      group,
			OS:         inst.OS,
			CPUs:       inst.CPUs,
			Memory:     inst.Memory,
			GPUs:       inst.GPUs,
			Packages:   append([]string(nil), inst.Packages...),
			IP:         ip,
			User:       user,
			Mode:       mode,
			Hostname:   host,
			HomeVolume: inst.HomeVolume,
			BaseName:   inst.Name,
			Index:      i + 1,
		})
	}
	return out, nil
}

// Names는 생성 이름 규칙이다.
func Names(base string, count int) (names []string, group string, err error) {
	if base == "" {
		return nil, "", fmt.Errorf("이름이 비어 있습니다")
	}
	if count < 1 {
		return nil, "", fmt.Errorf("생성 개수는 1 이상이어야 합니다")
	}
	if !validName(base) {
		return nil, "", fmt.Errorf("이름은 영문, 숫자, ., _, - 만 쓸 수 있습니다: %s", base)
	}
	if count == 1 {
		return []string{base}, "", nil
	}
	names = make([]string, count)
	for i := 1; i <= count; i++ {
		n := fmt.Sprintf("%s-%d", base, i)
		if !validName(n) {
			return nil, "", fmt.Errorf("생성된 이름이 너무 깁니다: %s", n)
		}
		names[i-1] = n
	}
	return names, base, nil
}

func validName(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if i > 0 {
			ok = ok || r == '.' || r == '_' || r == '-'
		}
		if !ok {
			return false
		}
	}
	return true
}

// Diff는 스펙과 현재 인스턴스를 비교한다.
// 스펙에 없는 다른 인스턴스는 지우지 않는다.
// 같은 그룹에서 count가 줄면 빠진 번호만 삭제한다.
func Diff(desired []Desired, actual []Actual) []Change {
	byName := map[string]Actual{}
	for _, a := range actual {
		byName[a.Name] = a
	}
	want := map[string]Desired{}
	groups := map[string]struct{}{}
	var changes []Change
	for _, d := range desired {
		want[d.Name] = d
		if d.Group != "" {
			groups[d.Group] = struct{}{}
		}
		cur, ok := byName[d.Name]
		if !ok {
			changes = append(changes, Change{Action: ActionCreate, Name: d.Name, Reason: "스펙에만 있음", Desired: d})
			continue
		}
		if reason, recreate := needsRecreate(d, cur); recreate {
			changes = append(changes, Change{Action: ActionRecreate, Name: d.Name, Reason: reason, Desired: d})
			continue
		}
		if reason, update := needsUpdate(d, cur); update {
			changes = append(changes, Change{Action: ActionUpdate, Name: d.Name, Reason: reason, Desired: d})
		}
	}
	for _, a := range actual {
		if _, ok := want[a.Name]; ok {
			continue
		}
		if a.Group != "" {
			if _, owned := groups[a.Group]; owned {
				changes = append(changes, Change{Action: ActionDelete, Name: a.Name, Reason: "그룹 개수 감소"})
			}
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Action != changes[j].Action {
			return actionOrder(changes[i].Action) < actionOrder(changes[j].Action)
		}
		return changes[i].Name < changes[j].Name
	})
	return changes
}

func actionOrder(a Action) int {
	switch a {
	case ActionDelete:
		return 0
	case ActionRecreate:
		return 1
	case ActionUpdate:
		return 2
	default:
		return 3
	}
}

func needsRecreate(d Desired, a Actual) (string, bool) {
	var why []string
	if a.OS != "" && d.OS != "" && a.OS != d.OS {
		why = append(why, fmt.Sprintf("OS %s → %s", a.OS, d.OS))
	}
	if a.Mode != "" && d.Mode != "" && a.Mode != d.Mode {
		why = append(why, fmt.Sprintf("mode %s → %s", a.Mode, d.Mode))
	}
	if a.User != "" && d.User != "" && a.User != d.User {
		why = append(why, fmt.Sprintf("user %s → %s", a.User, d.User))
	}
	wantPkg := strings.Join(sortedCopy(d.Packages), ",")
	if a.Packages != wantPkg {
		why = append(why, fmt.Sprintf("packages %s → %s", emptyDash(a.Packages), emptyDash(wantPkg)))
	}
	if d.IP != "" && a.IP != "" && d.IP != a.IP {
		why = append(why, fmt.Sprintf("ip %s → %s", a.IP, d.IP))
	}
	if len(why) == 0 {
		return "", false
	}
	return strings.Join(why, ", "), true
}

func needsUpdate(d Desired, a Actual) (string, bool) {
	var why []string
	if d.CPUs != "" && !sameCPUs(d.CPUs, a.CPUs) {
		why = append(why, fmt.Sprintf("cpus %s → %s", emptyDash(a.CPUs), d.CPUs))
	}
	if d.Memory != "" && !sameMemory(d.Memory, a.Memory) {
		why = append(why, fmt.Sprintf("memory %s → %s", emptyDash(a.Memory), d.Memory))
	}
	if len(why) == 0 {
		return "", false
	}
	return strings.Join(why, ", "), true
}

func sameCPUs(want, got string) bool {
	w, err1 := units.ParseCPUs(want)
	g, err2 := units.ParseCPUs(got)
	if err1 != nil || err2 != nil {
		return want == got
	}
	return w == g
}

func sameMemory(want, got string) bool {
	w, err1 := units.ParseMemory(want)
	g, err2 := units.ParseMemory(got)
	if err1 != nil || err2 != nil {
		return want == got
	}
	return w == g
}

func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
