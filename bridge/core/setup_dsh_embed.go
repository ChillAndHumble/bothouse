package core

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:dsh_bundle
var embeddedDSH embed.FS

// ExtractEmbeddedDSH extracts the embedded DSH files into targetDir (default ~/.bothouse/dsh).
// It respects os.UserHomeDir() so HOME overrides in tests or isolated environments work.
func ExtractEmbeddedDSH(targetDir string) (string, error) {
	if targetDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home dir: %w", err)
		}
		targetDir = filepath.Join(home, ".bothouse", "dsh")
	}

	sub, err := fs.Sub(embeddedDSH, "dsh_bundle")
	if err != nil {
		return "", fmt.Errorf("get embedded dsh_bundle sub fs: %w", err)
	}

	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		dest := filepath.Join(targetDir, path)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", path, err)
		}
		mode := os.FileMode(0644)
		if filepath.Ext(path) == ".sh" || d.Name() == "install.sh" {
			mode = 0755
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, mode)
	})
	if err != nil {
		return "", fmt.Errorf("unpack embedded dsh to %s: %w", targetDir, err)
	}

	scriptPath := filepath.Join(targetDir, "install.sh")
	_ = os.Chmod(scriptPath, 0755)
	_ = os.WriteFile(filepath.Join(targetDir, dshVersionFile), []byte(CurrentVersion), 0644)
	return scriptPath, nil
}

// dshVersionFile records which program version last unpacked ~/.bothouse/dsh.
const dshVersionFile = ".version"

// keepIfExists lists bundle files the user (or the wizard) may edit after install; a refresh
// never overwrites them once present.
func keepIfExists(rel string) bool {
	base := filepath.Base(rel)
	return base == "cordis.patch.yml" || filepath.Ext(base) == ".env"
}

// RefreshEmbeddedDSH re-unpacks the embedded plugin code into targetDir (default ~/.bothouse/dsh)
// when it was installed by a different program version, so plugin fixes reach users after an
// upgrade. It only runs for an existing install (install.sh present), writes only bundle files,
// keeps cordis.patch.yml / *.env that already exist, and never touches node_modules, the
// per-project .env/.patch.yml beside it, or the collab database (~/.conest-collab).
// Returns true when files were refreshed.
func RefreshEmbeddedDSH(targetDir, version string) (bool, error) {
	if targetDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, fmt.Errorf("resolve user home dir: %w", err)
		}
		targetDir = filepath.Join(home, ".bothouse", "dsh")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "install.sh")); err != nil {
		return false, nil // not installed yet; first install extracts everything
	}
	stamp := filepath.Join(targetDir, dshVersionFile)
	if old, err := os.ReadFile(stamp); err == nil && string(old) == version {
		return false, nil
	}
	sub, err := fs.Sub(embeddedDSH, "dsh_bundle")
	if err != nil {
		return false, err
	}
	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		dest := filepath.Join(targetDir, path)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		if keepIfExists(path) {
			if _, err := os.Stat(dest); err == nil {
				return nil
			}
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if filepath.Ext(path) == ".sh" {
			mode = 0755
		}
		return os.WriteFile(dest, data, mode)
	})
	if err != nil {
		return false, fmt.Errorf("refresh embedded dsh in %s: %w", targetDir, err)
	}
	_ = os.Chmod(filepath.Join(targetDir, "install.sh"), 0755)
	if err := os.WriteFile(stamp, []byte(version), 0644); err != nil {
		return true, err
	}
	return true, nil
}
