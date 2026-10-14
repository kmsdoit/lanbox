package cli

import (
	"fmt"
	"strings"

	"github.com/kmsdoit/lanbox/internal/instance"
	"github.com/kmsdoit/lanbox/internal/output"
	"github.com/spf13/cobra"
)

func addRm(root *cobra.Command) {
	var group string
	var yes, dry bool
	cmd := &cobra.Command{
		Use:   "rm [name...]",
		Short: "인스턴스 또는 그룹을 삭제한다",
		Long:  "쓰기 레이어는 삭제된다. --home-volume으로 만든 볼륨은 purge 전까지 남는다. lanbox.managed 라벨이 없는 컨테이너는 지우지 않는다.",
		Example: `  lanbox rm dev1
  lanbox rm -g web
  lanbox rm -g web -y`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && group == "" {
				return fmt.Errorf("지울 이름이나 -g 그룹이 필요합니다")
			}
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			picked, missing, err := instance.Select(ctx, d, instance.RemoveRequest{Names: args, Group: group})
			if err != nil {
				return err
			}
			names := make([]string, len(picked))
			for i, p := range picked {
				names[i] = p.Name
			}
			fmt.Fprintf(cmd.OutOrStdout(), "삭제 대상: %s\n", emptyList(names))
			if len(missing) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "없음: %s\n", strings.Join(missing, ", "))
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "경고: 컨테이너 쓰기 레이어의 패키지와 파일은 함께 사라집니다. 홈 볼륨은 purge 전까지 남습니다.")
			if dry {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run: 삭제하지 않습니다.")
				return nil
			}
			if len(picked) == 0 {
				return nil
			}
			ok, err := confirm(cmd, yes, "삭제할까요?")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "취소했습니다.")
				return nil
			}
			res, err := instance.Remove(ctx, d, picked)
			if err != nil {
				return err
			}
			res.Missing = missing
			if len(res.Volumes) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "유지한 홈 볼륨: %s\n", strings.Join(res.Volumes, ", "))
			}
			if outputFormat(cmd) == "json" {
				return output.Write(cmd.OutOrStdout(), "json", nil, nil, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "삭제함: %s\n", emptyList(res.Removed))
			return nil
		},
	}
	cmd.Flags().StringVarP(&group, "group", "g", "", "그룹 이름")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "확인을 생략한다")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "대상만 출력한다")
	root.AddCommand(cmd)
}

func addPurge(root *cobra.Command) {
	var yes, dry, images bool
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "lanbox가 만든 리소스를 모두 삭제한다",
		Long:  "컨테이너, 홈 볼륨, 네트워크, shim을 지운다. --images를 주면 파생 이미지와 스냅샷도 지운다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			plan, err := instance.PlanPurge(ctx, d, images)
			if err != nil {
				return err
			}
			if outputFormat(cmd) == "json" && dry {
				return output.Write(cmd.OutOrStdout(), "json", nil, nil, plan)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "컨테이너: %s\n볼륨: %s\n네트워크: %s\n이미지: %s\nshim: %v\n",
				emptyList(plan.Containers), emptyList(plan.Volumes), emptyList(plan.Networks), emptyList(plan.Images), plan.Shim)
			if plan.Empty() {
				fmt.Fprintln(cmd.OutOrStdout(), "삭제할 lanbox 리소스가 없습니다.")
				return nil
			}
			if dry {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run: 삭제하지 않습니다.")
				return nil
			}
			ok, err := confirm(cmd, yes, "lanbox 리소스를 모두 삭제할까요? 쓰기 레이어는 복구되지 않습니다.")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "취소했습니다.")
				return nil
			}
			if _, err := instance.ApplyPurge(ctx, d, images); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "purge 완료")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "확인을 생략한다")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "대상만 출력한다")
	cmd.Flags().BoolVar(&images, "images", false, "파생 이미지와 스냅샷도 삭제한다")
	root.AddCommand(cmd)
}

func emptyList(ss []string) string {
	if len(ss) == 0 {
		return "-"
	}
	return strings.Join(ss, ", ")
}
