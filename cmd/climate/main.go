package main

import (
	"fmt"
	"os"

	"github.com/disk0Dancer/climate/cmd/climate/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
