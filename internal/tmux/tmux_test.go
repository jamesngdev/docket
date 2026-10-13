package tmux

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDetachOnCtrlD checks the Ctrl-D key table really lands on the session
// (tmux rejects "=name" without the trailing ":" for set-option targets).
func TestDetachOnCtrlD(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not installed")
	}
	name := "docket-test-keytable"
	if err := Start(Spec{Name: name, Dir: t.TempDir(), Command: "sleep 30"}); err != nil {
		t.Fatal(err)
	}
	defer Kill(name)
	detachOnCtrlD(name)
	out, err := exec.Command("tmux", "show-options", "-t", "="+name+":", "-v", "key-table").Output()
	if err != nil || strings.TrimSpace(string(out)) != keyTable {
		t.Fatalf("key-table = %q (%v), want %q", out, err, keyTable)
	}
	keys, _ := exec.Command("tmux", "list-keys", "-T", keyTable).Output()
	if !strings.Contains(string(keys), "C-d detach-client") {
		t.Fatalf("missing C-d binding: %s", keys)
	}
}

func hasTmux() bool { _, err := exec.LookPath("tmux"); return err == nil }
