package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

func addSSH(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "ssh <name> [ssh-args...]",
		Short: "인스턴스에 SSH로 접속한다",
		Args:  cobra.MinimumNArgs(1),
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
			if inst.IP == "" {
				return fmt.Errorf("%s 에 IP가 없습니다", inst.Name)
			}
			if inst.Status != "running" {
				return fmt.Errorf("%s 가 실행 중이 아닙니다 (%s)", inst.Name, inst.Status)
			}
			sshBin, err := exec.LookPath("ssh")
			if err != nil {
				return fmt.Errorf("ssh 명령을 찾지 못했습니다")
			}
			argv := []string{
				"ssh",
				"-o", "StrictHostKeyChecking=accept-new",
				inst.User + "@" + inst.IP,
			}
			argv = append(argv, args[1:]...)
			return syscall.Exec(sshBin, argv, os.Environ())
		},
	}
	cmd.Flags().SetInterspersed(false)
	root.AddCommand(cmd)
}

func addShell(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "shell <name>",
		Short: "docker exec로 셸에 들어간다",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, err := openDocker(ctx)
			if err != nil {
				return err
			}
			inst, err := d.Lookup(ctx, args[0])
			d.Close()
			if err != nil {
				return err
			}
			if inst == nil {
				return fmt.Errorf("인스턴스 %s 가 없습니다", args[0])
			}
			bin, err := exec.LookPath("docker")
			if err != nil {
				return fmt.Errorf("docker CLI가 없습니다. shell은 docker exec를 사용합니다")
			}
			argv := []string{"docker", "exec", "-it", inst.Name, "sh", "-c", "command -v bash >/dev/null && exec bash || exec sh"}
			return syscall.Exec(bin, argv, os.Environ())
		},
	}
	root.AddCommand(cmd)
}
