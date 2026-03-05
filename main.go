package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/seb07-cloud/cactl/cmd"
	"github.com/seb07-cloud/cactl/pkg/types"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	if err := cmd.Execute(); err != nil {
		var exitErr *types.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, "Error: "+exitErr.Message)
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(types.ExitFatalError)
	}
}
