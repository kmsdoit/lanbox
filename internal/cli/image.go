package cli

import (
	"fmt"

	"github.com/kmsdoit/lanbox/internal/labels"
	"github.com/spf13/cobra"
)

func addImage(root *cobra.Command) {
	img := &cobra.Command{Use: "image", Short: "파생 이미지"}
	var dry bool
	prune := &cobra.Command{
		Use:   "prune",
		Short: "컨테이너가 쓰지 않는 파생 이미지와 스냅샷을 지운다",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			images, err := d.ListImages(ctx)
			if err != nil {
				return err
			}
			var removed int
			for _, img := range images {
				role := img.Labels[labels.Role]
				if role != labels.RoleImage && role != labels.RoleSnapshot {
					continue
				}
				used, err := d.ImageUsed(ctx, img.ID, img.Tags)
				if err != nil {
					return err
				}
				ref := img.ID
				if len(img.Tags) > 0 {
					ref = img.Tags[0]
				}
				if used {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "미사용: %s\n", ref)
				if dry {
					continue
				}
				if err := d.RemoveImage(ctx, ref); err != nil {
					return err
				}
				removed++
			}
			if dry {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run: 삭제하지 않습니다.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "삭제한 이미지: %d\n", removed)
			return nil
		},
	}
	prune.Flags().BoolVar(&dry, "dry-run", false, "대상만 출력한다")
	img.AddCommand(prune)
	root.AddCommand(img)
}
