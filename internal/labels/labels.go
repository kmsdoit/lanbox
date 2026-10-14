// Package labels는 lanbox가 Docker 리소스에 붙이는 라벨 키다.
// 조회와 삭제는 이 라벨만으로 하며, managed가 아닌 리소스는 건드리지 않는다.
package labels

const (
	Managed      = "lanbox.managed"
	Group        = "lanbox.group"
	OS           = "lanbox.os"
	Family       = "lanbox.family"
	User         = "lanbox.user"
	Name         = "lanbox.name"
	IP           = "lanbox.ip"
	CPUs         = "lanbox.cpus"
	Memory       = "lanbox.memory"
	GPUs         = "lanbox.gpus"
	Mode         = "lanbox.mode"
	Role         = "lanbox.role"
	Base         = "lanbox.base"
	Template     = "lanbox.template"
	Packages     = "lanbox.packages"
	Parent       = "lanbox.parent"
	Subnet       = "lanbox.subnet"
	Gateway      = "lanbox.gateway"
	Range        = "lanbox.range"
	Driver       = "lanbox.driver"
	Shim         = "lanbox.shim"
	ShimIP       = "lanbox.shimip"
	Reserve      = "lanbox.reserve"
	HostIP       = "lanbox.hostip"
	PasswordAuth = "lanbox.password_auth"
)

const (
	RoleInstance = "instance"
	RoleNetwork  = "network"
	RoleImage    = "image"
	RoleVolume   = "volume"
	RoleSnapshot = "snapshot"
	RoleProbe    = "probe"
)

// ManagedTrue는 라벨 맵이 lanbox 소유인지 확인한다.
func ManagedTrue(m map[string]string) bool {
	return m != nil && m[Managed] == "true"
}
