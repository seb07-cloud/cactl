package main

import (
	"errors"
	"os"

	"github.com/sebdah/cactl/cmd"
	"github.com/sebdah/cactl/pkg/types"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var exitErr *types.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(types.ExitFatalError)
	}
}
