package core

import (
	"net/http"
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
	mgmtJSON(w, http.StatusOK, resp)
}

// POST /api/v1/update/apply → download + replace the binary via SelfUpdate (blocks until done),
// then trigger the existing restart. On failure returns the reason and the download page.
func (m *ManagementServer) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		mgmtError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	cur := CurrentVersion
	fail := func(status int, msg, url string) {
		mgmtJSON(w, status, map[string]any{"status": "failed", "error": msg, "download_url": url})
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
		fail(http.StatusOK, err.Error(), releasesPage)
		return
	}
	if rel == nil {
		mgmtJSON(w, http.StatusOK, map[string]any{"status": "up_to_date", "version": cur})
		return
	}
	if err := selfUpdateFn(rel.TagName, false); err != nil {
		fail(http.StatusOK, err.Error(), downloadPage(rel))
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
