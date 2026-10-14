// Package output는 표와 JSON 출력을 담당한다.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// Write는 format이 json이면 v를, 아니면 표를 쓴다.
func Write(w io.Writer, format string, headers []string, rows [][]string, v any) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, join(headers))
	for _, row := range rows {
		fmt.Fprintln(tw, join(row))
	}
	return tw.Flush()
}

func join(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += "\t"
		}
		if c == "" {
			c = "-"
		}
		out += c
	}
	return out
}
