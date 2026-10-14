package cli

import (
	"fmt"
	"strings"

	"github.com/kmsdoit/lanbox/internal/ipam"
	"github.com/kmsdoit/lanbox/internal/netx"
	"github.com/kmsdoit/lanbox/internal/spec"
	"github.com/spf13/cobra"
)

func addInit(root *cobra.Command) {
	var (
		parent, subnet, gateway, ipRange, driver, name, shimIP string
		reserve                                                []string
		shim, dry                                              bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "LAN 네트워크와 설정을 준비한다",
		Long:  "부모 인터페이스와 서브넷을 감지하거나 인자로 받아 ipvlan/macvlan 네트워크를 만든다. Wi-Fi면 ipvlan, 유선이면 macvlan을 제안한다.",
		Example: `  sudo lanbox init --range 192.168.1.64/28 --shim
  lanbox init --dry-run --parent eth0 --subnet 192.168.1.0/24 --gateway 192.168.1.1 --range 192.168.1.64/28 --driver ipvlan`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			if parent == "" || subnet == "" || gateway == "" || driver == "" {
				info, err := netx.Detect()
				if err != nil && (parent == "" || subnet == "" || gateway == "" || driver == "") {
					return err
				}
				if parent == "" {
					parent = info.Name
				}
				if subnet == "" {
					subnet = info.Subnet
				}
				if gateway == "" {
					gateway = info.Gateway
				}
				if driver == "" {
					driver = info.SuggestedDriver
					fmt.Fprintf(cmd.ErrOrStderr(), "드라이버 제안: %s (%s)\n", driver, parent)
				}
			}
			var hostIP string
			if detected, err := netx.Detect(); err == nil && detected.Name == parent {
				hostIP = detected.HostIP
			}
			driver = strings.ToLower(driver)
			if driver != "ipvlan" && driver != "macvlan" {
				return fmt.Errorf("--driver는 ipvlan 또는 macvlan 이어야 합니다")
			}
			if ipRange == "" {
				return fmt.Errorf("--range가 필요합니다. DHCP와 겹치지 않는 구간을 지정하세요")
			}
			if err := ipam.ValidateNetwork(subnet, gateway, ipRange); err != nil {
				return err
			}
			wireless := netx.IsWireless("", parent)
			if wireless && driver == "macvlan" {
				return fmt.Errorf("무선 인터페이스 %s 에는 macvlan을 쓸 수 없습니다. ipvlan을 사용하세요", parent)
			}
			if wireless {
				fmt.Fprintf(cmd.ErrOrStderr(), "경고: %s 는 무선입니다. ipvlan이 불안정하면 유선 macvlan으로 바꾸세요.\n", parent)
			}
			pool, err := ipam.New(ipRange, nil)
			if err != nil {
				return err
			}
			if shim && shimIP == "" {
				shimIP = pool.LastUsable()
			}
			cfg := spec.Config{
				Network: spec.Network{
					Name: name, Driver: driver, Parent: parent,
					Subnet: subnet, Gateway: gateway, Range: ipRange, HostIP: hostIP,
				},
				Shim:    spec.Shim{Enabled: shim, IP: shimIP},
				Reserve: append([]string{}, reserve...),
			}
			if shimIP != "" {
				cfg.Reserve = appendUnique(cfg.Reserve, shimIP)
			}
			cfg.Reserve = appendUnique(cfg.Reserve, gateway)
			if hostIP != "" {
				cfg.Reserve = appendUnique(cfg.Reserve, hostIP)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "네트워크 %s\n  driver: %s\n  parent: %s\n  subnet: %s\n  gateway: %s\n  range: %s\n  shim: %v %s\n  reserve: %s\n",
				cfg.Network.Name, driver, parent, subnet, gateway, ipRange, shim, shimIP, strings.Join(cfg.Reserve, ","))
			fmt.Fprintln(cmd.ErrOrStderr(), "호스트 IP가 DHCP라면 공유기에 고정 할당하세요. 주소가 바뀌면 shim과 접속 경로가 깨집니다.")
			if dry {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run: 네트워크와 설정 파일을 만들지 않습니다.")
				return nil
			}
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			if _, _, err := d.EnsureNetwork(ctx, cfg.Network.Name, driver, parent, subnet, gateway, ipRange, cfg.Labels()); err != nil {
				return err
			}
			if shim {
				if err := netx.ShimUp(parent, driver, shimIP, subnet, ipRange); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "shim %s 주소 %s 경로 %s dev %s src %s\n", netx.ShimName, shimIP, ipRange, netx.ShimName, shimIP)
			}
			if err := spec.Save(configDir(cmd), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "설정을 기록했습니다: %s\n", spec.Path(configDir(cmd)))
			return nil
		},
	}
	cmd.Flags().StringVar(&ipRange, "range", "", "할당할 IP 범위 CIDR (예: 192.168.1.64/28)")
	cmd.Flags().StringVar(&parent, "parent", "", "부모 인터페이스")
	cmd.Flags().StringVar(&subnet, "subnet", "", "LAN 서브넷 CIDR")
	cmd.Flags().StringVar(&gateway, "gateway", "", "게이트웨이")
	cmd.Flags().StringVar(&driver, "driver", "", "ipvlan 또는 macvlan")
	cmd.Flags().StringVar(&name, "name", spec.DefaultNet, "Docker 네트워크 이름")
	cmd.Flags().BoolVar(&shim, "shim", false, "호스트 접속용 shim 인터페이스를 만든다")
	cmd.Flags().StringVar(&shimIP, "shim-ip", "", "shim IP (미지정 시 범위의 마지막 주소)")
	cmd.Flags().StringSliceVar(&reserve, "reserve", nil, "할당하지 않을 IP")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "만들 내용만 출력한다")
	root.AddCommand(cmd)
}

func appendUnique(ss []string, v string) []string {
	if v == "" {
		return ss
	}
	for _, s := range ss {
		if s == v {
			return ss
		}
	}
	return append(ss, v)
}
