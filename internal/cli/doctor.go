package cli

import (
	"fmt"
	"net"

	"github.com/kmsdoit/lanbox/internal/netx"
	"github.com/kmsdoit/lanbox/internal/output"
	"github.com/kmsdoit/lanbox/internal/preflight"
	"github.com/spf13/cobra"
)

func addDoctor(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "호스트 자원, 네트워크, IP 여유를 점검한다",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			var items []preflight.Item
			d, err := openDocker(ctx)
			if err != nil {
				items = append(items, preflight.Item{Name: "docker", Level: preflight.Block, Message: err.Error()})
				return printDoctor(cmd, items)
			}
			defer d.Close()
			items = append(items, preflight.Item{Name: "docker", Level: preflight.Pass, Message: "데몬에 연결됨"})
			info, ierr := d.Info(ctx)
			if ierr == nil {
				items = append(items, preflight.Item{Name: "kernel", Level: preflight.Pass, Message: info.Kernel + " (컨테이너와 공유)"})
			}
			cfg, cerr := loadConfig(ctx, cmd, d)
			if cerr != nil {
				items = append(items, preflight.Item{Name: "config", Level: preflight.Block, Message: cerr.Error()})
				return printDoctor(cmd, items)
			}
			items = append(items, preflight.Item{Name: "config", Level: preflight.Pass, Message: cfg.Network.Name + " " + cfg.Network.Driver + " " + cfg.Network.Parent})
			if netx.IsWireless("", cfg.Network.Parent) {
				items = append(items, preflight.Item{Name: "parent", Level: preflight.Warn, Message: cfg.Network.Parent + " 는 무선입니다. ipvlan이 불안정할 수 있습니다"})
			}
			if cfg.Shim.Enabled {
				ok, _ := netx.ShimExists()
				if ok {
					items = append(items, preflight.Item{Name: "shim", Level: preflight.Pass, Message: netx.ShimName + " " + cfg.Shim.IP})
				} else {
					items = append(items, preflight.Item{Name: "shim", Level: preflight.Warn, Message: "shim이 없습니다. lanbox shim up"})
				}
			}
			if cfg.Network.HostIP != "" {
				if ip := currentIPv4(cfg.Network.Parent); ip != "" && ip != cfg.Network.HostIP {
					items = append(items, preflight.Item{Name: "hostip", Level: preflight.Warn, Message: fmt.Sprintf("호스트 IP가 %s 에서 %s 로 바뀌었습니다. DHCP 고정 할당을 권장합니다", cfg.Network.HostIP, ip)})
				}
			}
			items = append(items, preflight.Item{Name: "dhcp", Level: preflight.Warn, Message: "공유기 DHCP 범위와 lanbox range가 겹치지 않는지 확인하세요"})
			host, _ := hostResources(ctx, d)
			inst, _ := d.ListInstances(ctx)
			in := preflight.Input{
				MemTotal: host.MemTotal, DiskFree: host.DiskFree, DiskTotal: host.DiskTotal, CPUs: host.CPUs,
				Wireless: netx.IsWireless("", cfg.Network.Parent), Driver: cfg.Network.Driver,
			}
			for _, it := range inst {
				in.Existing = append(in.Existing, preflight.Existing{Memory: it.Memory, CPUs: it.CPUs})
			}
			report := preflight.Check(in)
			items = append(items, report.Items...)
			return printDoctor(cmd, items)
		},
	}
	root.AddCommand(cmd)
}

func printDoctor(cmd *cobra.Command, items []preflight.Item) error {
	blocked := false
	for _, it := range items {
		if it.Level == preflight.Block {
			blocked = true
		}
	}
	if err := output.Write(cmd.OutOrStdout(), outputFormat(cmd), []string{"CHECK", "LEVEL", "MESSAGE"}, doctorRows(items), items); err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("doctor 점검에서 차단 항목이 있습니다")
	}
	return nil
}

func doctorRows(items []preflight.Item) [][]string {
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{it.Name, string(it.Level), it.Message}
	}
	return rows
}

func currentIPv4(parent string) string {
	if parent == "" {
		return ""
	}
	ifi, err := net.InterfaceByName(parent)
	if err != nil {
		return ""
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil {
			continue
		}
		return ipnet.IP.String()
	}
	return ""
}
