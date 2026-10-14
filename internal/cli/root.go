// Package cli는 lanbox 서브커맨드를 묶는다.
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kmsdoit/lanbox/internal/dockerx"
	"github.com/kmsdoit/lanbox/internal/spec"
	"github.com/spf13/cobra"
)

// Version은 빌드 시 -X로 덮어쓴다.
var Version = "dev"

// Execute는 프로세스가 호출하는 진입점이다.
func Execute() error {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "오류:", err)
		return err
	}
	return nil
}

// NewRoot는 테스트에서 인자만 바꿔 실행할 수 있는 루트 커맨드다.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "lanbox",
		Short:         "LAN IP를 가진 OS 컨테이너를 만들고 지운다",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}
	root.PersistentFlags().String("config-dir", "", "설정 디렉터리 (기본 /etc/lanbox, 환경변수 LANBOX_CONFIG_DIR)")
	root.PersistentFlags().StringP("output", "o", "table", "출력 형식: table 또는 json")
	addInit(root)
	addOS(root)
	addCreate(root)
	addLs(root)
	addSSH(root)
	addShell(root)
	addRm(root)
	addPurge(root)
	addLifecycle(root)
	addInfo(root)
	addDoctor(root)
	addStats(root)
	addImage(root)
	addShim(root)
	addSnapshot(root)
	addApply(root)
	return root
}

func configDir(cmd *cobra.Command) string {
	v, _ := cmd.Root().PersistentFlags().GetString("config-dir")
	return spec.Dir(v)
}

func outputFormat(cmd *cobra.Command) string {
	v, _ := cmd.Root().PersistentFlags().GetString("output")
	if v != "json" {
		return "table"
	}
	return v
}

func cmdContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Minute)
}

func openDocker(ctx context.Context) (*dockerx.Client, error) {
	return dockerx.New(ctx)
}

func loadConfig(ctx context.Context, cmd *cobra.Command, d *dockerx.Client) (spec.Config, error) {
	cfg, err := spec.Load(configDir(cmd))
	if err == nil {
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return spec.Config{}, err
	}
	if d == nil {
		return spec.Config{}, fmt.Errorf("설정 파일이 없습니다. lanbox init을 먼저 실행하세요")
	}
	nets, nerr := d.ListNetworks(ctx)
	if nerr != nil || len(nets) == 0 {
		return spec.Config{}, fmt.Errorf("설정 파일이 없습니다. lanbox init을 먼저 실행하세요")
	}
	recovered, rerr := spec.ConfigFromLabels(nets[0].Name, nets[0].Labels)
	if rerr != nil {
		return spec.Config{}, fmt.Errorf("설정 파일이 없고 네트워크 라벨로도 복구하지 못했습니다: %w", rerr)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "설정 파일이 없어 네트워크 라벨에서 복구했습니다.")
	return recovered, nil
}
