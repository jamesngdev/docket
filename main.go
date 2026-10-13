// docket: a terminal UI that turns project documentation into features and
// tickets, then orchestrates isolated Claude Code agents to implement them.
package main

import (
	"fmt"
	"os"
	"strings"

	"docket/internal/cli"
	"docket/internal/tui"
)

func main() {
	var err error
	switch args := os.Args[1:]; {
	case len(args) == 0:
		err = tui.Run("")
	case len(args) == 1 && !cli.IsCommand(args[0]) && isDir(args[0]):
		err = tui.Run(args[0]) // docket <path>: open that project directly
	case len(args) == 1 && !cli.IsCommand(args[0]) && strings.ContainsAny(args[0], "/.~"):
		err = fmt.Errorf("no such directory: %s", args[0])
	default:
		err = cli.Run(args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "docket:", err)
		os.Exit(1)
	}
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
