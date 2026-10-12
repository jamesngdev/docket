// docket: a terminal UI that turns project documentation into features and
// tickets, then orchestrates isolated Claude Code agents to implement them.
package main

import (
	"fmt"
	"os"

	"docket/internal/cli"
	"docket/internal/tui"
)

func main() {
	var err error
	if len(os.Args) > 1 {
		err = cli.Run(os.Args[1:])
	} else {
		err = tui.Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "docket:", err)
		os.Exit(1)
	}
}
