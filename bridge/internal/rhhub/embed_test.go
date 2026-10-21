package rhhub

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var relImport = regexp.MustCompile(`(?m)^\s*from \.(\w+) import`)

// The bundled hub is the repo's role-household/rh byte for byte, and every module it imports
// relatively is bundled too (a new import in hub.py must be added to HUB_FILES in the Makefile).
func TestBundledHubMatchesRepo(t *testing.T) {
	sub, _ := fs.Sub(srcFS, "src")
	if _, err := fs.Stat(sub, "rh/hub.py"); err != nil {
		t.Skip("hub source not bundled (run make hubsrc)")
	}
	repo := filepath.Join("..", "..", "..", "role-household", "rh")
	err := fs.WalkDir(sub, "rh", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" { // skipped by core.extractHubSource too
				return fs.SkipDir
			}
			return nil
		}
		got, _ := fs.ReadFile(sub, p)
		want, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(p[len("rh/"):])))
		if err != nil {
			t.Errorf("%s: not in role-household/rh any more: %v", p, err)
			return nil
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from role-household/rh: run make hubsrc", p)
		}
		if filepath.Ext(p) == ".py" {
			for _, m := range relImport.FindAllStringSubmatch(string(got), -1) {
				if _, err := fs.Stat(sub, "rh/"+m[1]+".py"); err != nil {
					t.Errorf("%s imports .%s, which is not bundled: add %s.py to HUB_FILES", p, m[1], m[1])
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
