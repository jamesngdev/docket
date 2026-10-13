// Package cli implements the non-interactive `docket` subcommands used by
// agents (questions, features, tickets, reviews, run completion) and by users
// (daemon control, auth token).
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"docket/internal/daemon"
	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/project"
	"docket/internal/store"
)

// args holds positional arguments and --flag values (both `--k v` and `--k=v`).
type args struct {
	pos   []string
	flags map[string]string
}

func parse(in []string) args {
	a := args{flags: map[string]string{}}
	for i := 0; i < len(in); i++ {
		s := in[i]
		if !strings.HasPrefix(s, "--") {
			a.pos = append(a.pos, s)
			continue
		}
		k := strings.TrimPrefix(s, "--")
		if eq := strings.IndexByte(k, '='); eq >= 0 {
			a.flags[k[:eq]] = k[eq+1:]
			continue
		}
		if i+1 < len(in) && !strings.HasPrefix(in[i+1], "--") {
			a.flags[k] = in[i+1]
			i++
		} else {
			a.flags[k] = "true"
		}
	}
	return a
}

// text returns --name, or the contents of --name-file when given.
func (a args) text(name string) (string, error) {
	if f := a.flags[name+"-file"]; f != "" {
		b, err := os.ReadFile(f)
		return strings.TrimSpace(string(b)), err
	}
	return a.flags[name], nil
}

func (a args) need(name string) (string, error) {
	v, err := a.text(name)
	if err == nil && strings.TrimSpace(v) == "" {
		err = fmt.Errorf("--%s (or --%s-file) is required", name, name)
	}
	return v, err
}

func (a args) id(i int) (int64, error) {
	if i >= len(a.pos) {
		return 0, fmt.Errorf("missing id argument")
	}
	return strconv.ParseInt(strings.TrimPrefix(a.pos[i], "#"), 10, 64)
}

// projectRepo resolves the project: --project, $DOCKET_PROJECT, or the
// nearest parent of the cwd containing .docket/po.sqlite.
func projectRepo(a args) (string, error) {
	if p := a.flags["project"]; p != "" {
		return filepath.Abs(p)
	}
	if p := os.Getenv("DOCKET_PROJECT"); p != "" {
		return p, nil
	}
	dir, _ := os.Getwd()
	return project.Find(dir)
}

func openDB(a args) (*store.DB, paths.Project, error) {
	repo, err := projectRepo(a)
	if err != nil {
		return nil, paths.Project{}, err
	}
	p := paths.New(repo)
	db, err := store.Open(p.DB())
	return db, p, err
}

// kick nudges the daemon so state changes are acted on immediately (best effort).
func kick(p paths.Project) { _ = daemon.Send(p, ipc.CmdKick, "") }
