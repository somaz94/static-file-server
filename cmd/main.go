package main

import (
	"os"

	"github.com/somaz94/static-file-server/cmd/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
