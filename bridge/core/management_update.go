package core

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
)

// releasesPage is where users download a release by hand when the in-app update fails.
const releasesPage = "https://github.com/zuiho-kai/bothouse/releases/latest"

// Swappable in tests; production uses the existing updater (same path as `/upgrade confirm`).
var (
	updateCheckFn = CheckForUpdate
	selfUpdateFn  = SelfUpdate
	updateMu      sync.Mutex // one apply at a time
)

func isDevVersion(v string) bool { return v == "" || v == "dev" }

func downloadPage(r *ReleaseInfo) string {
	if r != nil && r.HTMLURL != "" {
		return r.HTMLURL
	}
	return releasesPage
}

// GET /api/v1/update → current version, latest version, whether an update exists.
func (m *ManagementServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		mgmtError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	cur := CurrentVersion
	resp := map[string]any{
		"current":      cur,
		"latest":       "",
		"has_update":   false,
		"dev":          false,
		"download_url": releasesPage,
		"error":        "",
	}
	if isDevVersion(cur) {
		resp["dev"] = true
		resp["error"] = "开发版不检查更新 / development build: update check disabled"
		mgmtJSON(w, http.StatusOK, resp)
		return
	}
	rel, err := updateCheckFn(cur, false)
	if err != nil {
		resp["error"] = err.Error()
		mgmtJSON(w, http.StatusOK, resp)
		return
	}
	if rel != nil {
		resp["latest"] = rel.TagName
		resp["has_update"] = true
		resp["download_url"] = downloadPage(rel)
	} else {
		resp["latest"] = cur
	}
	if lr := ReadUpdateResult(UpdateStateDir); lr != nil && lr.Status != "ok" {
		resp["last_result"] = lr
	}
	mgmtJSON(w, http.StatusOK, resp)
}

// POST /api/v1/update/ack clears the rollback note once the page showed it.
func (m *ManagementServer) handleUpdateAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		mgmtError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	ClearUpdateResult(UpdateStateDir)
	mgmtJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// updateFailReason maps an update error to a kind the page explains in plain words:
// permission (cannot replace the program file), download, checksum, restart, other.
func updateFailReason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "permission"
	case errors.Is(err, ErrUpdateChecksum):
		return "checksum"
	case errors.Is(err, ErrUpdateDownload):
		return "download"
	case errors.Is(err, errUpdateRestart):
		return "restart"
	}
	return "other"
}

var errUpdateRestart = errors.New("cannot start the update self-check")

// POST /api/v1/update/apply → download + replace the binary via SelfUpdate (blocks until done),
// then trigger the existing restart. On failure returns the reason and the download page.
func (m *ManagementServer) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		mgmtError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	cur := CurrentVersion
	fail := func(status int, msg, url string, reason ...string) {
		rs := "other"
		if len(reason) > 0 {
			rs = reason[0]
		}
		mgmtJSON(w, status, map[string]any{"status": "failed", "error": msg, "reason": rs, "download_url": url})
	}
	if isDevVersion(cur) {
		fail(http.StatusOK, "开发版不检查更新 / development build: update check disabled", releasesPage)
		return
	}
	if !updateMu.TryLock() {
		fail(http.StatusConflict, "update already in progress", releasesPage)
		return
	}
	defer updateMu.Unlock()

	rel, err := updateCheckFn(cur, false)
	if err != nil {
		fail(http.StatusOK, err.Error(), releasesPage, "download")
		return
	}
	if rel == nil {
		mgmtJSON(w, http.StatusOK, map[string]any{"status": "up_to_date", "version": cur})
		return
	}
	if err := selfUpdateFn(rel.TagName, false); err != nil {
		fail(http.StatusOK, err.Error(), downloadPage(rel), updateFailReason(err))
		return
	}
	// Self-check: the old binary watches the new one and puts itself back if it does not come up.
	spec := m.guardSpec(rel.TagName)
	if spec.Backup == "" {
		slog.Warn("update: no backup path, restarting without self-check")
	} else if err := startUpdateGuard(spec); err != nil {
		// Without a guard a broken new version would stay; undo the replacement instead.
		slog.Error("update: start self-check failed, undoing update", "error", err)
		if uerr := undoReplace(spec); uerr != nil {
			slog.Error("update: undo failed", "error", uerr)
		}
		fail(http.StatusOK, errors.Join(errUpdateRestart, err).Error(), downloadPage(rel), "restart")
		return
	}
	mgmtJSON(w, http.StatusOK, map[string]any{"status": "restarting", "version": rel.TagName})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	select {
	case RestartCh <- RestartRequest{}:
	default:
	}
}

// undoReplace puts the backup back when the self-check could not be started.
func undoReplace(spec updateGuardSpec) error {
	failed := spec.Exe + ".failed"
	if err := os.Rename(spec.Exe, failed); err != nil {
		return err
	}
	if err := os.Rename(spec.Backup, spec.Exe); err != nil {
		_ = os.Rename(failed, spec.Exe)
		return err
	}
	_ = os.Remove(failed)
	return nil
}
