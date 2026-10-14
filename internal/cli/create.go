package cli

import (
	"fmt"
	"strings"

	"github.com/kmsdoit/lanbox/internal/instance"
	"github.com/kmsdoit/lanbox/internal/osreg"
	"github.com/kmsdoit/lanbox/internal/output"
	"github.com/kmsdoit/lanbox/internal/preflight"
	"github.com/kmsdoit/lanbox/internal/sshkey"
	"github.com/spf13/cobra"
)

func addCreate(root *cobra.Command) {
	var (
		osName, family, user, keyPath, password, cpus, memory, gpus, ip, hostname, mode, from string
		packages                                                                              []string
		count                                                                                 int
		home, dry                                                                             bool
	)
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "SSH가 준비된 OS 컨테이너를 만든다",
		Example: `  lanbox create dev1 --os ubuntu:24.04 --cpus 2 --memory 2g
  lanbox create web --os rocky -n 3 --cpus 1 --memory 1g
  lanbox create edge --os alpine
  lanbox create gpu1 --os ubuntu:24.04 --gpus all`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext()
			defer cancel()
			d, derr := openDocker(ctx)
			if derr != nil {
				if !dry {
					return derr
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "경고: %v\n", derr)
				d = nil
			} else {
				defer d.Close()
			}
			cfg, err := loadConfig(ctx, cmd, d)
			if err != nil {
				return err
			}
			reg, err := osreg.Load(configDir(cmd))
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("user") {
				user = cfg.Defaults.User
			}
			if !cmd.Flags().Changed("mode") {
				mode = cfg.Defaults.Mode
			}
			explicitKey := ""
			if cmd.Flags().Changed("ssh-key") {
				explicitKey = keyPath
			}
			configured := cfg.Defaults.SSHKey
			if explicitKey != "" {
				configured = ""
			}
			path, ok, err := sshkey.Resolve(explicitKey, configured)
			if err != nil {
				return err
			}
			pub := ""
			if ok {
				pub, err = sshkey.Read(path)
				if err != nil {
					return err
				}
			}
			made, report, err := instance.Create(ctx, d, cfg, reg, instance.CreateRequest{
				BaseName:   args[0],
				Count:      count,
				OS:         osName,
				Family:     family,
				User:       user,
				PublicKey:  pub,
				Password:   password,
				Packages:   packages,
				CPUs:       cpus,
				Memory:     memory,
				GPUs:       gpus,
				StartIP:    ip,
				Hostname:   hostname,
				Mode:       mode,
				HomeVolume: home,
				From:       from,
				DryRun:     dry,
			}, cmd.ErrOrStderr())
			printReport(cmd, report)
			if err != nil {
				return err
			}
			if dry {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run: 컨테이너를 만들지 않습니다.")
			}
			return printCreated(cmd, made)
		},
	}
	cmd.Flags().StringVar(&osName, "os", "", "별칭 또는 Docker 이미지")
	cmd.Flags().IntVarP(&count, "count", "n", 1, "생성 개수. 2 이상이면 <name>-1..N")
	cmd.Flags().StringVar(&ip, "ip", "", "시작 IP. 미지정 시 범위의 빈 주소")
	cmd.Flags().StringVar(&cpus, "cpus", "", "CPU 제한")
	cmd.Flags().StringVar(&memory, "memory", "", "메모리 제한 (예: 2g). swap은 같은 값으로 고정")
	cmd.Flags().StringVar(&gpus, "gpus", "", "NVIDIA GPU (all 또는 개수). 생략하면 요청하지 않는다")
	cmd.Flags().StringVar(&user, "user", "lanbox", "sudo 계정")
	cmd.Flags().StringVar(&keyPath, "ssh-key", "", "authorized_keys에 넣을 공개키")
	cmd.Flags().StringVar(&password, "password", "", "지정할 때만 비밀번호 로그인을 허용")
	cmd.Flags().StringSliceVar(&packages, "packages", nil, "추가 패키지. 쉼표로 구분")
	cmd.Flags().StringVar(&hostname, "hostname", "", "호스트 이름. 기본은 인스턴스 이름")
	cmd.Flags().BoolVar(&home, "home-volume", false, "/home을 볼륨으로 둔다. rm 해도 볼륨은 purge 전까지 남는다")
	cmd.Flags().StringVar(&mode, "mode", "light", "light 또는 systemd")
	cmd.Flags().StringVar(&family, "family", "", "계열 감지 실패 시 debian, rhel, alpine, suse, arch")
	cmd.Flags().StringVar(&from, "from", "", "스냅샷 이미지에서 만든다")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "실행할 내용만 출력")
	root.AddCommand(cmd)
}

func printReport(cmd *cobra.Command, report preflight.Report) {
	for _, it := range report.Items {
		switch it.Level {
		case preflight.Warn:
			fmt.Fprintf(cmd.ErrOrStderr(), "경고: %s: %s\n", it.Name, it.Message)
		case preflight.Block:
			fmt.Fprintf(cmd.ErrOrStderr(), "차단: %s: %s\n", it.Name, it.Message)
		}
	}
}

func printCreated(cmd *cobra.Command, made []instance.Created) error {
	if outputFormat(cmd) == "json" {
		return output.Write(cmd.OutOrStdout(), "json", nil, nil, made)
	}
	rows := make([][]string, 0, len(made))
	for _, m := range made {
		rows = append(rows, []string{m.Name, m.Image, m.IP, m.SSH})
	}
	if err := output.Write(cmd.OutOrStdout(), "table", []string{"NAME", "OS", "IP", "SSH"}, rows, made); err != nil {
		return err
	}
	if len(made) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "접속:", strings.Join(sshLines(made), "  "))
	}
	return nil
}

func sshLines(made []instance.Created) []string {
	out := make([]string, len(made))
	for i, m := range made {
		out[i] = m.SSH
	}
	return out
}
