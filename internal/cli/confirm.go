package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func confirm(cmd *cobra.Command, yes bool, prompt string) (bool, error) {
	if yes {
		return true, nil
	}
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok {
		fi, err := f.Stat()
		if err == nil && fi.Mode()&os.ModeCharDevice == 0 {
			return false, fmt.Errorf("터미널이 아닙니다. -y 로 확인을 생략하세요")
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", prompt)
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes", nil
}
