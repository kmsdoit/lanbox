package main

import (
	"os"

	"github.com/kmsdoit/lanbox/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
