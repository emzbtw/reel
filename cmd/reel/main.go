package main

import (
	"fmt"
	"os"

	"github.com/emzbtw/reel/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+cli.FormatError(err))
		os.Exit(1)
	}
}
