package docscan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashesAndDiff(t *testing.T) {
	root := t.TempDir()
	write := func(p, s string) {
		full := filepath.Join(root, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.md", "one")
	write("sub/b.md", "two")
	write("c.md", "three")
	write("img.png", "x")
	write(".hidden/d.md", "skip")

	h, err := Hashes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 3 {
		t.Fatalf("want 3 md files, got %v", h)
	}
	first, changed := Diff(nil, h, "c1")
	if changed != 3 {
		t.Fatalf("first scan: every file is new, got %d", changed)
	}

	write("a.md", "one-changed")
	_ = os.Remove(filepath.Join(root, "c.md"))
	write("e.md", "new")
	h2, _ := Hashes(root)
	second, changed := Diff(first, h2, "c2")
	flags := map[string]string{}
	for _, d := range second {
		flags[d.Path] = d.Change
	}
	want := map[string]string{"a.md": "M", "sub/b.md": "", "c.md": "D", "e.md": "A"}
	if changed != 3 || len(flags) != 4 {
		t.Fatalf("changed=%d flags=%v", changed, flags)
	}
	for p, f := range want {
		if flags[p] != f {
			t.Errorf("%s: got %q want %q", p, flags[p], f)
		}
	}

	// Deleted docs drop out on the following scan.
	third, changed := Diff(second, h2, "c3")
	if changed != 0 || len(third) != 3 {
		t.Fatalf("third scan: changed=%d len=%d", changed, len(third))
	}
}
