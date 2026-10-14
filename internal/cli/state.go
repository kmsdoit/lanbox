package cli

import (
	"context"
	"fmt"

	"github.com/kmsdoit/lanbox/internal/dockerx"
	"github.com/kmsdoit/lanbox/internal/preflight"
	"github.com/spf13/cobra"
)

type runner struct {
	ctx context.Context
	d   *dockerx.Client
	id  string
}

func stateCmd(use, short string, fn func(runner) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <name>",
		Short: short,
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
			if err := fn(runner{ctx: ctx, d: d, id: inst.ID}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", use, inst.Name)
			return nil
		},
	}
}

func hostResources(ctx context.Context, d *dockerx.Client) (preflight.Resources, error) {
	info, err := d.Info(ctx)
	root := ""
	if err == nil {
		root = info.Root
	}
	return preflight.HostResources(root)
}

func resizeReport(host preflight.Resources, others []preflight.Existing, nextCPU float64, nextMem int64) preflight.Report {
	return preflight.Check(preflight.Input{
		MemTotal:      host.MemTotal,
		CPUs:          host.CPUs,
		NewCount:      1,
		NewMemoryEach: nextMem,
		NewCPUsEach:   nextCPU,
		Existing:      others,
	})
}
