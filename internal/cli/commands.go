package cli

import (
	"fmt"
	"os"
	"strings"

	"docket/internal/config"
	"docket/internal/daemon"
	"docket/internal/gitops"
	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/store"
)

const usage = `docket                         open the TUI
docket <path>                  open the TUI on the project containing <path> (new-project form if none)
docket daemon [--project P]    run the project daemon in the foreground
docket daemon stop|status      stop / check the project daemon
docket auth set-token <token>  optional: dedicated agent token (from 'claude setup-token'); default reuses your claude login
docket auth clear-token        go back to reusing your claude login
docket discord set-token <bot-token> | clear-token   Discord bot for Backend Support (all projects)
docket discord channel <channel-id>|off | status   answer @mentions in this project's channel
docket phase list | phase set <docs-branch>   list / switch the active phase (docs branch)
docket show questions|features|tickets|ticket <id>
docket question add --text T [--source S] | question answer <id> --text T
docket feature add --title T --description D [--docs a.md,b.md] | feature outdated <id>
docket ticket add --feature F --title T --requirements R --acceptance A --files a/,b.go [--depends 1,2] [--model M]
docket ticket outdated <id> | ticket status <id> <status> [--pr URL]
docket ticket update <id> [--title T] [--requirements R] [--acceptance A] [--depends 1,2] [--files F] [--model M|default]
docket ticket redo <id> --reason R          rework an in-flight ticket on its branch
docket ticket merge <id>                    squash-merge an approved PR (TechLead, auto-merge on)
docket ticket flag <id> --reason R          flag a ticket for the user
docket automerge on|off|status              merge Tester-approved PRs (this project)
docket automerge review risky|all           risky (default): low-risk PRs with green CI merge directly, TechLead reviews the rest
docket note add --text T                    record a repo fact for every later agent (.docket/REPO.md)
docket review submit <ticket> --result approve|request_changes --summary S
docket run done [--summary S]
docket version                              print the installed version
Any text flag X also accepts --X-file <path>.`

// Version is stamped at release time via -ldflags "-X docket/internal/cli.Version=…".
var Version = "dev"

// commands are the subcommands Run understands; anything else may be a path.
var commands = map[string]bool{"help": true, "--help": true, "daemon": true, "auth": true, "show": true,
	"discord": true, "automerge": true, "phase": true, "question": true, "feature": true, "ticket": true, "review": true, "run": true, "note": true,
	"version": true, "--version": true}

// IsCommand reports whether name is a docket subcommand (not a project path).
func IsCommand(name string) bool { return commands[name] }

// Run dispatches a subcommand. args excludes the program name.
func Run(argv []string) error {
	if len(argv) == 0 || argv[0] == "help" || argv[0] == "--help" {
		fmt.Println(usage)
		return nil
	}
	cmd, sub := argv[0], ""
	rest := argv[1:]
	if len(rest) > 0 {
		sub = rest[0]
	}
	a := parse(rest)
	switch cmd {
	case "version", "--version":
		fmt.Println("docket", Version)
		return nil
	case "daemon":
		return daemonCmd(a, sub)
	case "auth":
		switch {
		case sub == "clear-token":
			return config.SetToken("")
		case sub == "set-token" && len(a.pos) >= 2:
			tok := strings.TrimSpace(a.pos[1])
			if !strings.HasPrefix(tok, "sk-ant-") {
				return fmt.Errorf("that is not a Claude token (expected sk-ant-…, printed at the end of `claude setup-token`)")
			}
			return config.SetToken(tok)
		}
		return fmt.Errorf("usage: docket auth set-token <sk-ant-…> | auth clear-token\n(no token = agents reuse your normal `claude` login)")
	case "show":
		return show(a, sub)
	case "discord":
		return discordCmd(a, sub)
	case "automerge":
		return autoMergeCmd(a, sub)
	case "phase":
		return phaseCmd(a, sub)
	case "question", "feature", "ticket", "review", "run", "note":
		return write(cmd, sub, parse(rest[min(1, len(rest)):]))
	}
	return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
}

func daemonCmd(a args, sub string) error {
	repo, err := projectRepo(a)
	if err != nil {
		return err
	}
	p := paths.New(repo)
	switch sub {
	case "stop":
		return daemon.Stop(p)
	case "status":
		if daemon.Alive(p) {
			fmt.Println("running")
		} else {
			fmt.Println("stopped")
		}
		return nil
	}
	return daemon.Run(repo)
}

func show(a args, what string) error {
	db, _, err := openDB(a)
	if err != nil {
		return err
	}
	defer db.Close()
	switch what {
	case "questions":
		qs, err := db.Questions()
		for _, q := range qs {
			fmt.Printf("Q%d [%s] %s\n   source: %s\n   answer: %s\n", q.ID, q.Status, q.Text, q.Source, q.Answer)
		}
		return err
	case "features":
		fs, err := db.Features()
		for _, f := range fs {
			fmt.Printf("F%d [%s] %s\n   docs: %s\n%s\n\n", f.ID, f.Status, f.Title, f.Documents, f.Description)
		}
		return err
	case "tickets":
		ts, err := db.Tickets()
		for _, t := range ts {
			fmt.Printf("#%d [%s] F%d %s depends:%s pr:%s\n", t.ID, t.Status, t.FeatureID, t.Title, store.JoinIDs(t.DependsOn), t.PRURL)
		}
		return err
	case "ticket":
		id, err := a.id(1)
		if err != nil {
			return err
		}
		t, err := db.Ticket(id)
		if err != nil {
			return err
		}
		fmt.Printf("#%d %s [%s]\nbranch: %s\npr: %s\n\n## Requirements\n%s\n\n## Acceptance\n%s\n\n## Review (%s)\n%s\n",
			t.ID, t.Title, t.Status, t.Branch, t.PRURL, t.Requirements, t.Acceptance, t.ReviewState, t.ReviewSummary)
		return nil
	}
	return fmt.Errorf("show questions|features|tickets|ticket <id>")
}

var validTicketStatus = []string{store.TBacklog, store.TPending, store.TProcessing, store.TWaitReview, store.TDone, store.TOutdated}

func runID() (string, error) {
	id := os.Getenv("DOCKET_RUN")
	if id == "" {
		return "", fmt.Errorf("DOCKET_RUN is not set (only agent sessions can finish runs)")
	}
	return id, nil
}

// phaseCmd lists docs branches or asks the daemon to switch the active phase.
func phaseCmd(a args, sub string) error {
	repo, err := projectRepo(a)
	if err != nil {
		return err
	}
	p := paths.New(repo)
	switch sub {
	case "list":
		db, err := store.Open(p.DB())
		if err != nil {
			return err
		}
		defer db.Close()
		proj, _ := db.Project()
		_ = gitops.Fetch(p.Docs())
		branches, err := gitops.Branches(p.Docs())
		for _, b := range branches {
			mark := " "
			if b == proj.ActivePhase {
				mark = "*"
			}
			fmt.Println(mark, b)
		}
		return err
	case "set":
		if len(a.pos) < 2 {
			return fmt.Errorf("usage: docket phase set <docs-branch>")
		}
		if err := daemon.EnsureRunning(p); err != nil { // switching is done by the daemon
			return err
		}
		if err := daemon.Send(p, ipc.CmdPhase, a.pos[1]); err != nil {
			return err
		}
		fmt.Printf("switching to phase %s; progress shows in the TUI dashboard activity\n", a.pos[1])
		return nil
	}
	return fmt.Errorf("usage: docket phase list | phase set <docs-branch>")
}
