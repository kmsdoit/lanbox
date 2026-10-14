package cli

import (
	"fmt"
	"sort"

	"github.com/kmsdoit/lanbox/internal/labels"
	"github.com/kmsdoit/lanbox/internal/osreg"
	"github.com/kmsdoit/lanbox/internal/output"
	"github.com/spf13/cobra"
)

func addOS(root *cobra.Command) {
	osCmd := &cobra.Command{Use: "os", Short: "OS 별칭"}
	osCmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "별칭, 이미지, 계열, 캐시된 파생 이미지",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := osreg.Load(configDir(cmd))
			if err != nil {
				return err
			}
			cached := map[string]string{}
			familyOf := map[string]string{}
			ctx, cancel := cmdContext()
			defer cancel()
			if d, derr := openDocker(ctx); derr == nil {
				defer d.Close()
				imgs, ierr := d.ListImages(ctx)
				if ierr == nil {
					for _, img := range imgs {
						if img.Labels[labels.Role] != labels.RoleImage {
							continue
						}
						base := img.Labels[labels.Base]
						tag := img.ID
						if len(img.Tags) > 0 {
							tag = img.Tags[0]
						}
						cached[base] = tag
						if img.Labels[labels.Family] != "" {
							familyOf[base] = img.Labels[labels.Family]
						}
					}
				}
			}
			type row struct {
				Alias  string `json:"alias"`
				Image  string `json:"image"`
				Source string `json:"source"`
				Family string `json:"family"`
				Cached string `json:"cached"`
			}
			var aliases []string
			for alias := range reg.Aliases {
				aliases = append(aliases, alias)
			}
			sort.Strings(aliases)
			var rows []row
			var table [][]string
			for _, alias := range aliases {
				image := reg.Aliases[alias]
				fam := familyOf[image]
				if fam == "" {
					fam = "-"
				}
				cache := cached[image]
				if cache == "" {
					cache = "-"
				}
				rows = append(rows, row{alias, image, reg.Source(alias), fam, cache})
				table = append(table, []string{alias, image, reg.Source(alias), fam, cache})
			}
			return output.Write(cmd.OutOrStdout(), outputFormat(cmd),
				[]string{"ALIAS", "IMAGE", "SOURCE", "FAMILY", "CACHED"}, table, rows)
		},
	})
	osCmd.AddCommand(&cobra.Command{
		Use:   "add <alias> <image>",
		Short: "사용자 별칭을 등록한다",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			image, err := osreg.Add(configDir(cmd), args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", args[0], image)
			return nil
		},
	})
	root.AddCommand(osCmd)
}
