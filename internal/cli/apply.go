package cli

import (
	"fmt"

	"github.com/kmsdoit/lanbox/internal/instance"
	"github.com/kmsdoit/lanbox/internal/osreg"
	"github.com/kmsdoit/lanbox/internal/output"
	"github.com/kmsdoit/lanbox/internal/spec"
	"github.com/spf13/cobra"
)

func addApply(root *cobra.Command) {
	var file string
	var yes, dry bool
	cmd := &cobra.Command{
		Use:   "apply -f lab.yaml",
		Short: "선언형 스펙과 차이를 반영한다",
		Long:  "개수 변경은 생성과 삭제, 자원 변경은 docker update, OS·패키지·모드·계정·IP 변경은 재생성이다. 스펙에 없는 다른 인스턴스는 지우지 않는다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("-f 스펙 파일이 필요합니다")
			}
			lab, err := spec.LoadLab(file)
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			defer d.Close()
			cfg, err := loadConfig(ctx, cmd, d)
			if err != nil {
				return err
			}
			reg, err := osreg.Load(configDir(cmd))
			if err != nil {
				return err
			}
			if dry {
				changes, err := instance.Apply(ctx, d, cfg, reg, lab, true, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				return printChanges(cmd, changes)
			}
			// 계획을 먼저 출력한 뒤 적용한다.
			preview, err := instance.Apply(ctx, d, cfg, reg, lab, true, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if err := printChanges(cmd, preview); err != nil {
				return err
			}
			if len(preview) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "변경 없음")
				return nil
			}
			ok, err := confirm(cmd, yes, "이 변경을 적용할까요? 재생성은 쓰기 레이어를 지웁니다.")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "취소했습니다.")
				return nil
			}
			if _, err := instance.Apply(ctx, d, cfg, reg, lab, false, cmd.ErrOrStderr()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "적용 완료")
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "랩 스펙 YAML")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "확인을 생략한다")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "변경 계획만 출력한다")
	root.AddCommand(cmd)
}

func printChanges(cmd *cobra.Command, changes []spec.Change) error {
	if len(changes) == 0 {
		if outputFormat(cmd) == "json" {
			return output.Write(cmd.OutOrStdout(), "json", nil, nil, changes)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "변경 없음")
		return nil
	}
	var rows [][]string
	for _, ch := range changes {
		rows = append(rows, []string{string(ch.Action), ch.Name, ch.Reason})
	}
	return output.Write(cmd.OutOrStdout(), outputFormat(cmd), []string{"ACTION", "NAME", "REASON"}, rows, changes)
}
