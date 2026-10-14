package cli

import (
	"fmt"

	"github.com/kmsdoit/lanbox/internal/preflight"
	"github.com/kmsdoit/lanbox/internal/units"
	"github.com/spf13/cobra"
)

func addLifecycle(root *cobra.Command) {
	root.AddCommand(stateCmd("start", "인스턴스를 시작한다", func(run runner) error { return run.d.Start(run.ctx, run.id) }))
	root.AddCommand(stateCmd("stop", "인스턴스를 멈춘다", func(run runner) error { return run.d.Stop(run.ctx, run.id) }))
	root.AddCommand(stateCmd("restart", "인스턴스를 다시 시작한다", func(run runner) error { return run.d.Restart(run.ctx, run.id) }))

	var cpus, memory string
	resize := &cobra.Command{
		Use:   "resize <name>",
		Short: "실행 중 자원 제한을 바꾼다",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cpus == "" && memory == "" {
				return fmt.Errorf("--cpus 또는 --memory가 필요합니다")
			}
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
			nextCPU := inst.CPUs
			nextMem := inst.Memory
			if cpus != "" {
				nextCPU, err = units.ParseCPUs(cpus)
				if err != nil {
					return err
				}
			}
			if memory != "" {
				nextMem, err = units.ParseMemory(memory)
				if err != nil {
					return err
				}
			}
			host, _ := hostResources(ctx, d)
			all, err := d.ListInstances(ctx)
			if err != nil {
				return err
			}
			var others []preflight.Existing
			for _, item := range all {
				if item.Name == inst.Name {
					continue
				}
				others = append(others, preflight.Existing{Memory: item.Memory, CPUs: item.CPUs})
			}
			report := resizeReport(host, others, nextCPU, nextMem)
			printReport(cmd, report)
			if report.Blocked() {
				return fmt.Errorf("%s", report.Error())
			}
			if err := d.UpdateResources(ctx, inst.ID, nextCPU, nextMem); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s cpus=%s memory=%s\n", inst.Name, units.FormatCPUs(nextCPU), units.FormatMemory(nextMem))
			return nil
		},
	}
	resize.Flags().StringVar(&cpus, "cpus", "", "CPU 제한")
	resize.Flags().StringVar(&memory, "memory", "", "메모리 제한")
	root.AddCommand(resize)
}
