package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/config"
	"github.com/chenhg5/cc-connect/core"
	"github.com/gorilla/websocket"
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

// Two roles on the self-hosted platform: the sub's config (as the wizard wrote it) recognises the
// lead's dispatch by the lead's mention_id and hands the finished answer back with <@lead>
// (platform/cloud-web lead_return.go; same frames as rh/cloudweb.py sends).
func TestWizardCloudWebTeamSubReturnsToLead(t *testing.T) {
	var mu sync.Mutex
	var replies []string
	got := make(chan struct{}, 1)
	var leadID string
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bot-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Upgrade") != "websocket" || r.URL.Query().Get("bot") == "" { // the wizard's connection test
			conn, buf, _ := w.(http.Hijacker).Hijack()
			_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			_ = buf.Flush()
			conn.Close()
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil { // register
			return
		}
		_ = conn.WriteJSON(map[string]any{"type": "register_ack", "ok": true, "capabilities": []string{"text"}})
		mu.Lock()
		lead := leadID
		mu.Unlock()
		// the lead @-s this sub (rh/cloudweb.py for_bot: from_bot, user_mention_id = the lead's mention_id)
		_ = conn.WriteJSON(map[string]any{"type": "message", "msg_id": "hub-1", "chat_id": "room-1", "chat_type": "group",
			"user_id": "juzi", "user_name": "橘", "user_mention_id": lead, "content": "【橘】@小龙 你来算 17×23",
			"reply_ctx": "room-1", "mentioned": true, "from_bot": true})
		for {
			var m map[string]any
			if err := conn.ReadJSON(&m); err != nil {
				return
			}
			if m["type"] == "reply" {
				mu.Lock()
				replies = append(replies, fmt.Sprint(m["content"]))
				mu.Unlock()
				select {
				case got <- struct{}{}:
				default:
				}
			}
		}
	}))
	defer hub.Close()

	dir := t.TempDir()
	old := config.ConfigPath
	config.ConfigPath = filepath.Join(dir, "config.toml")
	defer func() { config.ConfigPath = old }()
	if err := os.WriteFile(config.ConfigPath, []byte(`data_dir = "`+filepath.ToSlash(dir)+`"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clis := []core.LocalCLI{{AgentType: "claudecode", Supported: true, Login: "logged_in"}}
	res := core.ApplyWizard(context.Background(), core.WizardEnv{Store: configWizardStore{}, DataDir: dir, CLIs: clis},
		core.WizardRequest{Platform: "cloud_web", Language: "zh", CloudWeb: &core.WizardCloudWeb{URL: hub.URL, Token: "bot-secret"},
			Roles: []core.WizardRole{{Name: "橘", Project: "juzi"}, {Name: "小龙", Project: "xiaolong"}}})
	if !res.Saved || res.FallbackRole != "juzi" {
		t.Fatalf("apply: %+v", res)
	}
	cfg, err := config.LoadPermissive(config.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	opts := map[string]map[string]any{}
	for _, p := range cfg.Projects {
		for _, pl := range p.Platforms {
			if pl.Type == "cloud_web" {
				opts[p.Name] = pl.Options
			}
		}
	}
	lid := cloudWebQuery(fmt.Sprint(opts["juzi"]["ws_url"]), "mention_id")
	if lid == "" || opts["xiaolong"]["team_lead_bot"] != lid || opts["juzi"]["team_lead_bot"] != nil {
		t.Fatalf("team wiring: lead=%v sub=%v", opts["juzi"], opts["xiaolong"])
	}
	mu.Lock()
	leadID = lid
	mu.Unlock()

	po := map[string]any{"cc_project": "xiaolong", "cc_data_dir": dir}
	for k, v := range opts["xiaolong"] {
		po[k] = v
	}
	p, err := core.CreatePlatform("cloud_web", po)
	if err != nil {
		t.Fatal(err)
	}
	err = p.Start(func(pl core.Platform, msg *core.Message) {
		rc := msg.ReplyCtx
		if m, ok := pl.(core.FinalReplyMarker); ok {
			rc = m.MarkFinalReply(rc) // the engine marks a turn's final answer the same way
		}
		_ = pl.Send(context.Background(), rc, "391")
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	select {
	case <-got:
	case <-time.After(10 * time.Second): // 来源：测试等待上限（实现者定）
		t.Fatal("no reply from the sub")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(replies) != 1 || replies[0] != "<@"+lid+"> 391" {
		t.Fatalf("sub's answer = %q, want it handed back to the lead <@%s>", replies, lid)
	}
}

func cloudWebQuery(wsURL, key string) string {
	u, err := url.Parse(wsURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}
