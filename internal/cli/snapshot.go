package cli

import (
	"fmt"
	"time"

	"github.com/kmsdoit/lanbox/internal/builder"
	"github.com/kmsdoit/lanbox/internal/labels"
	"github.com/spf13/cobra"
)

func addSnapshot(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "snapshot <name>",
		Short: "인스턴스 파일시스템을 이미지로 저장한다",
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
			ref, err := builder.SnapshotRef(inst.Name, time.Now())
			if err != nil {
				return err
			}
			id, err := d.Commit(ctx, inst.ID, ref, map[string]string{
				labels.Managed: "true",
				labels.Role:    labels.RoleSnapshot,
				labels.Name:    inst.Name,
				labels.OS:      inst.OS,
				labels.Family:  inst.Family,
				labels.User:    inst.User,
				labels.Mode:    inst.Mode,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", ref, id)
			fmt.Fprintf(cmd.ErrOrStderr(), "복제: lanbox create <name> --from %s\n", ref)
			return nil
		},
	}
	root.AddCommand(cmd)
}
