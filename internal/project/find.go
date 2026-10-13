package project

import (
	"fmt"
	"os"
	"path/filepath"

	"docket/internal/paths"
)

// Find returns the docket project containing dir: the nearest ancestor (or
// dir itself) holding a .docket database, migrating a po-tui era project.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(paths.New(dir).DB()); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".po-tui", "po.sqlite")); err == nil {
			return dir, MigrateLegacy(dir) // project from the po-tui days
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a docket project (use --project or DOCKET_PROJECT)")
		}
		dir = parent
	}
}
