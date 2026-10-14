package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kmsdoit/lanbox/internal/netx"
	"github.com/spf13/cobra"
)

func addShim(root *cobra.Command) {
	shim := &cobra.Command{Use: "shim", Short: "호스트에서 컨테이너로 가는 shim"}
	shim.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "설정에 따라 shim을 만든다",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if !cfg.Shim.Enabled || cfg.Shim.IP == "" {
				return fmt.Errorf("설정에서 shim이 꺼져 있습니다. lanbox init --shim")
			}
			if err := netx.ShimUp(cfg.Network.Parent, cfg.Network.Driver, cfg.Shim.IP, cfg.Network.Subnet, cfg.Network.Range); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "shim %s %s\n경로 %s dev %s src %s\n", netx.ShimName, cfg.Shim.IP, cfg.Network.Range, netx.ShimName, cfg.Shim.IP)
			return nil
		},
	})
	shim.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "shim 인터페이스를 지운다",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := netx.ShimDown(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "shim 삭제")
			return nil
		},
	})
	shim.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "shim을 systemd 유닛으로 등록한다",
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return err
			}
			unit := fmt.Sprintf(`[Unit]
Description=lanbox host shim
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=%s shim up
ExecStop=%s shim down

[Install]
WantedBy=multi-user.target
`, exe, exe)
			path := "/etc/systemd/system/lanbox-shim.service"
			if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
				return fmt.Errorf("systemd 유닛 기록: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "기록함: %s\n", path)
			if _, err := exec.LookPath("systemctl"); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "systemctl이 없습니다. 유닛 파일만 남겼습니다.")
				return nil
			}
			if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
				return fmt.Errorf("daemon-reload: %s", out)
			}
			if out, err := exec.Command("systemctl", "enable", "--now", "lanbox-shim.service").CombinedOutput(); err != nil {
				return fmt.Errorf("enable: %s", out)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "lanbox-shim.service 활성화")
			return nil
		},
	})
	root.AddCommand(shim)
}
