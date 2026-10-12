// Package docscan walks the cloned docs repo and diffs it against the
// previous scan so TechLead only receives new / modified / deleted files.
package docscan

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"docket/internal/store"
)

// Hashes returns path → sha256 for every *.md file under root (skipping .git).
func Hashes(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		sum := sha256.Sum256(b)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out, err
}

// Diff compares the previous scan with current hashes. Deleted files are kept
// with change "D" so the TechLead can react to them; they disappear next scan.
func Diff(prev []store.Document, cur map[string]string, commit string) (docs []store.Document, changed int) {
	old := map[string]string{}
	for _, d := range prev {
		if d.Change != "D" {
			old[d.Path] = d.Hash
		}
	}
	for path, h := range cur {
		change := ""
		if oh, ok := old[path]; !ok {
			change = "A"
		} else if oh != h {
			change = "M"
		}
		if change != "" {
			changed++
		}
		docs = append(docs, store.Document{Path: path, Hash: h, Change: change, Commit: commit})
	}
	for path, h := range old {
		if _, ok := cur[path]; !ok {
			changed++
			docs = append(docs, store.Document{Path: path, Hash: h, Change: "D", Commit: commit})
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, changed
}
