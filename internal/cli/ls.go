package cli

import (
	"fmt"

	"github.com/kmsdoit/lanbox/internal/output"
	"github.com/kmsdoit/lanbox/internal/units"
	"github.com/spf13/cobra"
)

func addLs(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "인스턴스 목록",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			items, err := d.ListInstances(ctx)
			if err != nil {
				return err
			}
			type row struct {
				Name   string `json:"name"`
				OS     string `json:"os"`
				IP     string `json:"ip"`
				Status string `json:"status"`
				CPUs   string `json:"cpus"`
				Mem    string `json:"memory"`
				SSH    string `json:"ssh"`
			}
			var rows []row
			var table [][]string
			for _, it := range items {
				ssh := ""
				if it.IP != "" {
					ssh = "ssh " + it.User + "@" + it.IP
				}
				cpus := it.CPUText
				if cpus == "" {
					cpus = units.FormatCPUs(it.CPUs)
				}
				mem := it.MemoryText
				if mem == "" {
					mem = units.FormatMemory(it.Memory)
				}
				rows = append(rows, row{it.Name, it.OS, it.IP, it.Status, cpus, mem, ssh})
				table = append(table, []string{it.Name, it.OS, it.IP, it.Status, cpus, mem, ssh})
			}
			return output.Write(cmd.OutOrStdout(), outputFormat(cmd),
				[]string{"NAME", "OS", "IP", "STATUS", "CPUS", "MEM", "SSH"}, table, rows)
		},
	}
	root.AddCommand(cmd)
}

func addInfo(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "인스턴스 상세와 커널 공유 정보",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			inst, err := d.Lookup(ctx, args[0])
			if err != nil {
				return err
			}
			if inst == nil {
				return fmt.Errorf("인스턴스 %s 가 없습니다", args[0])
			}
			info, _ := d.Info(ctx)
			sample, serr := d.Stats(ctx, inst.ID)
			type view struct {
				Name     string `json:"name"`
				OS       string `json:"os"`
				Family   string `json:"family"`
				User     string `json:"user"`
				IP       string `json:"ip"`
				Status   string `json:"status"`
				Mode     string `json:"mode"`
				CPUs     string `json:"cpus"`
				Memory   string `json:"memory"`
				Packages string `json:"packages"`
				SSH      string `json:"ssh"`
				CPUUsage string `json:"cpu_usage,omitempty"`
				MemUsage string `json:"mem_usage,omitempty"`
				Kernel   string `json:"kernel"`
				Note     string `json:"note"`
			}
			v := view{
				Name: inst.Name, OS: inst.OS, Family: inst.Family, User: inst.User, IP: inst.IP,
				Status: inst.Status, Mode: inst.Mode,
				CPUs: units.FormatCPUs(inst.CPUs), Memory: units.FormatMemory(inst.Memory),
				Packages: inst.Packages, SSH: "ssh " + inst.User + "@" + inst.IP,
				Kernel: info.Kernel,
				Note:   "컨테이너는 호스트 커널을 공유합니다. uname -r은 호스트 값입니다.",
			}
			if serr == nil {
				v.CPUUsage = fmt.Sprintf("%.1f%%", sample.CPUPercent)
				v.MemUsage = units.FormatMemory(int64(sample.MemUsage))
			}
			if outputFormat(cmd) == "json" {
				return output.Write(cmd.OutOrStdout(), "json", nil, nil, v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "name: %s\nos: %s\nfamily: %s\nuser: %s\nip: %s\nstatus: %s\nmode: %s\ncpus: %s\nmemory: %s\npackages: %s\nssh: %s\ncpu_usage: %s\nmem_usage: %s\nkernel: %s\nnote: %s\n",
				v.Name, dash(v.OS), dash(v.Family), v.User, dash(v.IP), v.Status, dash(v.Mode), v.CPUs, v.Memory, dash(v.Packages), v.SSH, dash(v.CPUUsage), dash(v.MemUsage), dash(v.Kernel), v.Note)
			return nil
		},
	}
	root.AddCommand(cmd)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func addStats(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "인스턴스 CPU, 메모리 사용량",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			items, err := d.ListInstances(ctx)
			if err != nil {
				return err
			}
			type row struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
				Limit  string `json:"limit"`
			}
			var rows []row
			var table [][]string
			for _, it := range items {
				cpu, mem, limit := "-", "-", units.FormatMemory(it.Memory)
				if it.Status == "running" {
					s, serr := d.Stats(ctx, it.ID)
					if serr == nil {
						cpu = fmt.Sprintf("%.1f%%", s.CPUPercent)
						mem = units.FormatMemory(int64(s.MemUsage))
						if s.MemLimit > 0 {
							limit = units.FormatMemory(int64(s.MemLimit))
						}
					}
				}
				rows = append(rows, row{it.Name, it.Status, cpu, mem, limit})
				table = append(table, []string{it.Name, it.Status, cpu, mem, limit})
			}
			return output.Write(cmd.OutOrStdout(), outputFormat(cmd),
				[]string{"NAME", "STATUS", "CPU", "MEM", "LIMIT"}, table, rows)
		},
	}
	root.AddCommand(cmd)
}
