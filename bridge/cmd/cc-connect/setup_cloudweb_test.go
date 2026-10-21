package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhg5/cc-connect/config"
	"github.com/chenhg5/cc-connect/core"
)

// End to end on a real config.toml: an existing Discord role joins the self-hosted platform,
// a new role is added; the Discord section stays as it was and the written cloud_web section
// is accepted by the cloud_web platform itself (platform/cloud-web New).
func TestWizardCloudWebConfigKeepsDiscordAndLoads(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cloud-web/ws" || r.Header.Get("Authorization") != "Bearer bot-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, buf, _ := w.(http.Hijacker).Hijack()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buf.Flush()
		conn.Close()
	}))
	defer hub.Close()

	dir := t.TempDir()
	old := config.ConfigPath
	config.ConfigPath = filepath.Join(dir, "config.toml")
	defer func() { config.ConfigPath = old }()
	discordToken := base64.RawStdEncoding.EncodeToString([]byte("1999999999999999002")) + ".abc.def"
	work := filepath.Join(dir, "juzi")
	cfgText := `data_dir = "` + filepath.ToSlash(dir) + `"

[[projects]]
name = "juzi"

[projects.agent]
type = "claudecode"

[projects.agent.options]
work_dir = "` + filepath.ToSlash(work) + `"

[[projects.platforms]]
type = "discord"

[projects.platforms.options]
token = "` + discordToken + `"
guild_id = "900"
allow_from = "199999999999999011"
`
	if err := os.WriteFile(config.ConfigPath, []byte(cfgText), 0o600); err != nil {
		t.Fatal(err)
	}
	clis := []core.LocalCLI{{AgentType: "claudecode", Supported: true, Login: "logged_in"}}
	res := core.ApplyWizard(context.Background(), core.WizardEnv{Store: configWizardStore{}, DataDir: dir, CLIs: clis},
		core.WizardRequest{Platform: "cloud_web", Language: "zh", CloudWeb: &core.WizardCloudWeb{URL: hub.URL, Token: "bot-secret"},
			Roles: []core.WizardRole{{ReuseProject: "juzi", Name: "橘"}, {Name: "Whale"}}})
	if !res.Saved || res.Error != "" {
		t.Fatalf("apply: %+v", res)
	}
	cfg, err := config.LoadPermissive(config.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]config.ProjectConfig{}
	for _, p := range cfg.Projects {
		byName[p.Name] = p
	}
	juzi := byName["juzi"]
	var discord, cw map[string]any
	for _, pl := range juzi.Platforms {
		switch pl.Type {
		case "discord":
			discord = pl.Options
		case "cloud_web":
			cw = pl.Options
		}
	}
	if discord == nil || discord["token"] != discordToken || discord["guild_id"] != "900" || discord["allow_from"] != "199999999999999011" {
		t.Fatalf("Discord section changed: %v", discord)
	}
	if cw == nil || !strings.Contains(cw["ws_url"].(string), "?bot=juzi&name=%E6%A9%98&mention_id=1999999999999999002") {
		t.Fatalf("cloud_web section: %v", cw)
	}
	for _, name := range []string{"juzi", "whale"} {
		p, ok := byName[name]
		if !ok {
			t.Fatalf("project %s missing: %v", name, cfg.Projects)
		}
		for _, pl := range p.Platforms {
			if pl.Type != "cloud_web" {
				continue
			}
			opts := map[string]any{"cc_project": p.Name, "cc_data_dir": dir}
			for k, v := range pl.Options {
				opts[k] = v
			}
			if _, err := core.CreatePlatform("cloud_web", opts); err != nil {
				t.Fatalf("%s: cloud_web rejects the wizard's options: %v (%v)", name, err, pl.Options)
			}
		}
	}
}
