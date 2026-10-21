package core

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeHub answers the bot WebSocket handshake like rh/cloudweb.py handle: 401 for a wrong token,
// 101 for the right one (the connection is then dropped; the wizard never registers).
type fakeHubServer struct {
	*httptest.Server
	mu        sync.Mutex
	handshake int // successful handshakes
}

func fakeHub(t *testing.T, token string) *fakeHubServer {
	t.Helper()
	h := &fakeHubServer{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cloud-web/ws" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buf.Flush()
		conn.Close()
		h.mu.Lock()
		h.handshake++
		h.mu.Unlock()
	}))
	t.Cleanup(h.Close)
	return h
}

func (h *fakeHubServer) port(t *testing.T) int {
	_, p, _ := net.SplitHostPort(strings.TrimPrefix(h.URL, "http://"))
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// noLocalHub: nothing answers on the "default hub port" (whatever runs on this machine's 8790).
func noLocalHub(t *testing.T) {
	old := localHubPort
	localHubPort = 1
	t.Cleanup(func() { localHubPort = old })
}

func TestCloudWebWSBase(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:8790":                 "ws://127.0.0.1:8790/cloud-web/ws",
		"http://127.0.0.1:8790/":                "ws://127.0.0.1:8790/cloud-web/ws",
		"127.0.0.1:8790":                        "ws://127.0.0.1:8790/cloud-web/ws",
		"https://hub.example.com":               "wss://hub.example.com/cloud-web/ws",
		"https://example.com/hub":               "wss://example.com/hub/cloud-web/ws",
		"wss://1.2.3.4/cloud-web/ws":            "wss://1.2.3.4/cloud-web/ws",
		"ws://h:1/cloud-web/ws?bot=x":           "ws://h:1/cloud-web/ws?bot=x", // query is the caller's to strip
		"https://hub.example.com/cloud-web/ws/": "wss://hub.example.com/cloud-web/ws",
	}
	for in, want := range cases {
		got, err := cloudWebWSBase(in)
		if strings.Contains(in, "?") {
			got, err = cloudWebWSBase(stripQuery(in))
			want = stripQuery(want)
		}
		if err != nil || got != want {
			t.Errorf("cloudWebWSBase(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://x", "http://"} {
		if _, err := cloudWebWSBase(bad); err == nil {
			t.Errorf("cloudWebWSBase(%q) accepted", bad)
		}
	}
	if got := cloudWebPageURL("wss://example.com/hub/cloud-web/ws"); got != "https://example.com/hub/" {
		t.Errorf("page url = %q", got)
	}
	if got := cloudWebBotURL("wss://h/cloud-web/ws", "juzi", "橘", "123"); got != "wss://h/cloud-web/ws?bot=juzi&name=%E6%A9%98&mention_id=123" {
		t.Errorf("bot url = %q", got)
	}
}

// 测试连接：通 / token 错（401）/ 连不上 / 地址不是机器人入口，各给修法。
func TestTestCloudWebCodes(t *testing.T) {
	hub := fakeHub(t, "bot-secret")
	ctx := context.Background()
	if r := TestCloudWeb(ctx, hub.URL, "bot-secret", "zh"); !r.OK || r.Code != "ok" || r.Status != 101 || r.PageURL != hub.URL+"/" {
		t.Fatalf("ok: %+v", r)
	}
	if r := TestCloudWeb(ctx, hub.URL, "wrong", "zh"); r.OK || r.Code != "unauthorized" || r.Status != 401 || !strings.Contains(r.Fix, "token") {
		t.Fatalf("401: %+v", r)
	}
	if r := TestCloudWeb(ctx, "ws"+strings.TrimPrefix(hub.URL, "http")+"/elsewhere", "bot-secret", "en"); r.Code != "not_found" || r.Fix == "" {
		t.Fatalf("404: %+v", r)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	addr := closed.URL
	closed.Close()
	if r := TestCloudWeb(ctx, addr, "bot-secret", "ja"); r.OK || r.Code != "unreachable" || !strings.Contains(r.Fix, "127.0.0.1") {
		t.Fatalf("unreachable: %+v", r)
	}
	if r := TestCloudWeb(ctx, "ftp://x", "t", "en"); r.Code != "bad_url" {
		t.Fatalf("bad url: %+v", r)
	}
}

func cloudWebReq(url, token string, roles ...WizardRole) WizardRequest {
	return WizardRequest{Platform: "cloud_web", Language: "zh", CloudWeb: &WizardCloudWeb{URL: url, Token: token}, Roles: roles}
}

// dry-run 规划出 cloud_web options，不写任何东西；真跑写进去的就是这些。
func TestApplyWizardCloudWebDryRunPlansOptions(t *testing.T) {
	noHardware(t)
	noLocalHub(t)
	hub := fakeHub(t, "bot-secret")
	store := newMemStore()
	dir := t.TempDir()
	req := cloudWebReq(hub.URL, "bot-secret", WizardRole{Name: "Juzi"}, WizardRole{Name: "龙娘"})
	req.DryRun = true
	res := ApplyWizard(context.Background(), WizardEnv{Store: store, DataDir: dir, CLIs: testCLIs}, req)
	if !res.OK || res.Saved || store.batches != 0 || len(store.projects) != 0 || res.CloudWeb == nil || !res.CloudWeb.OK {
		t.Fatalf("dry run: ok=%v saved=%v batches=%d %+v", res.OK, res.Saved, store.batches, res)
	}
	if res.Roles[0].Project != "juzi" || res.Roles[1].Project != "role-2" || res.Roles[1].Name != "龙娘" || res.FallbackRole != "juzi" {
		t.Fatalf("roles: %+v", res.Roles)
	}
	if _, err := os.Stat(filepath.Join(dir, setupStateFile)); err == nil {
		t.Fatal("dry run wrote the wizard state")
	}
	// the plan the dry run made (white box: what a real run hands to the store)
	snap, _ := store.Snapshot()
	ids := []wizardIdentity{{WizardRole: WizardRole{Name: "Juzi", ReplyLanguage: "zh"}}, {WizardRole: WizardRole{Name: "龙娘", ReplyLanguage: "zh"}}}
	base, _ := cloudWebWSBase(hub.URL)
	pl, err := planWizard(snap, ids, req, SetupState{}, dir, "claudecode", testCLIs, "", "", "", false, base, "bot-secret")
	if err != nil {
		t.Fatal(err)
	}
	o := pl.writes[0].Options
	wantWS := "ws" + strings.TrimPrefix(hub.URL, "http") + "/cloud-web/ws?bot=juzi&name=Juzi"
	if pl.writes[0].PlatformType != "cloud_web" || o["transport"] != "websocket" || o["ws_url"] != wantWS || o["token"] != "bot-secret" ||
		o["allow_from"] != "user" || o["share_session_in_channel"] != true || o["group_reply_all"] != true {
		t.Fatalf("options = %v", o)
	}
	// two roles = a team: lead answers when no gate decides, the other only when named
	if *pl.writes[0].GateFallback != "reply" || *pl.writes[1].GateFallback != "context" || *pl.writes[1].GateURL != defaultGateURL+"?bot=role-2" {
		t.Fatalf("team gate: %v %v %v", *pl.writes[0].GateFallback, *pl.writes[1].GateFallback, *pl.writes[1].GateURL)
	}
	// the real run writes exactly that, in one batch, and remembers the cloud_web team separately
	req.DryRun = false
	res = ApplyWizard(context.Background(), WizardEnv{Store: store, DataDir: dir, CLIs: testCLIs}, req)
	if !res.Saved || store.batches != 1 || store.projects["juzi"].Options["ws_url"] != wantWS {
		t.Fatalf("real run: %+v %v", res, store.projects["juzi"])
	}
	st := LoadSetupState(dir)
	if st.Platform != "" || len(st.Team) != 0 || st.CloudWebFallback != "juzi" || len(st.CloudWebTeam) != 2 || st.CloudWebURL != base {
		t.Fatalf("state = %+v", st)
	}
	if res.GateHint == "" { // a team without a gate is told how it decides who answers
		t.Errorf("no gate hint for a cloud_web team")
	}
}

// 缺项报错：没地址/token（本机也没有向导起的平台）、token 错（401）都什么都不写；平台暂时不在线就存下并阻塞提醒。
func TestApplyWizardCloudWebMissingAndErrors(t *testing.T) {
	noHardware(t)
	noLocalHub(t)
	hub := fakeHub(t, "bot-secret")
	env := func(s *memStore) WizardEnv { return WizardEnv{Store: s, DataDir: t.TempDir(), CLIs: testCLIs} }
	s := newMemStore()
	res := ApplyWizard(context.Background(), env(s), WizardRequest{Platform: "cloud_web", Language: "en", Roles: []WizardRole{{Name: "a"}}})
	if res.Saved || !strings.Contains(res.Error, "url and token") || s.batches != 0 {
		t.Fatalf("missing: %+v", res)
	}
	res = ApplyWizard(context.Background(), env(s), cloudWebReq(hub.URL, "", WizardRole{Name: "a"}))
	if res.Saved || res.Error == "" {
		t.Fatalf("missing token: %+v", res)
	}
	res = ApplyWizard(context.Background(), env(s), cloudWebReq(hub.URL, "wrong", WizardRole{Name: "a"}))
	if res.Saved || s.batches != 0 || !strings.Contains(res.Error, "token") || res.CloudWeb == nil || res.CloudWeb.Code != "unauthorized" {
		t.Fatalf("401: %+v", res)
	}
	res = ApplyWizard(context.Background(), env(s), cloudWebReq(hub.URL, "bot-secret", WizardRole{Name: "a"}, WizardRole{Name: "a"}))
	if res.Saved || !strings.Contains(res.Error, "used twice") {
		t.Fatalf("same name twice: %+v", res)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	addr := closed.URL
	closed.Close()
	res = ApplyWizard(context.Background(), env(s), cloudWebReq(addr, "bot-secret", WizardRole{Name: "a"}))
	if !res.Saved || res.OK || !res.NeedsUserAction || len(res.Warnings) == 0 || res.Warnings[0].Code != "cloud_web_unreachable" {
		t.Fatalf("unreachable: %+v", res)
	}
}

// 同名 bot 不重复：再跑一次同一个名字，更新原来的项目，不会出现 juzi-2。
func TestApplyWizardCloudWebSameNameNotDuplicated(t *testing.T) {
	noHardware(t)
	noLocalHub(t)
	hub := fakeHub(t, "bot-secret")
	store := newMemStore()
	store.extra = []string{"juzi"} // a project named juzi on another platform already exists
	dir := t.TempDir()
	run := func(roles ...WizardRole) *WizardResult {
		return ApplyWizard(context.Background(), WizardEnv{Store: store, DataDir: dir, CLIs: testCLIs}, cloudWebReq(hub.URL, "bot-secret", roles...))
	}
	if res := run(WizardRole{Name: "Juzi"}); !res.Saved || res.Roles[0].Project != "juzi-2" {
		t.Fatalf("first: %+v", res.Roles)
	}
	res := run(WizardRole{Name: "Juzi"}, WizardRole{Name: "橘"})
	if !res.Saved || res.Roles[0].Project != "juzi-2" || !res.Roles[0].Reused || res.Roles[1].Project != "role-2" {
		t.Fatalf("second: %+v", res.Roles)
	}
	res = run(WizardRole{Name: "橘"})
	if !res.Saved || res.Roles[0].Project != "role-2" || !res.Roles[0].Reused || len(store.order) != 3 {
		t.Fatalf("third: %+v %v", res.Roles, store.order)
	}
	// the earlier team stays the team: juzi-2 is still wired as lead on the third run
	if st := LoadSetupState(dir); len(st.CloudWebTeam) != 2 || st.CloudWebFallback != "juzi-2" {
		t.Fatalf("team = %+v", st)
	}
}

// 本机自动发现：向导起过的 hub（有 hub.json）地址和 token 都自动填；别人起在 8790 的只填地址。
func TestLocalHubDiscovery(t *testing.T) {
	noHardware(t)
	noLocalHub(t)
	hub := fakeHub(t, "bot-secret")
	dir := t.TempDir()
	ctx := context.Background()
	if st := GetLocalHubStatus(ctx, dir, "zh"); st.Running || st.Managed {
		t.Fatalf("nothing there: %+v", st)
	}
	// someone else's hub on the default port: address known, token not
	localHubPort = hub.port(t)
	st := GetLocalHubStatus(ctx, dir, "zh")
	if !st.Running || st.Managed || st.TokenKnown || st.URL != localHubURL(localHubPort) || st.OpenURL != "" {
		t.Fatalf("unmanaged: %+v", st)
	}
	// the hub this wizard started: <data_dir>/hub/hub.json (0600)
	localHubPort = 1
	if err := writeJSONFile(filepath.Join(dir, "hub", "hub.json"), localHubState{Port: hub.port(t), WebToken: "web-secret", BotToken: "bot-secret"}, 0o600); err != nil {
		t.Fatal(err)
	}
	st = GetLocalHubStatus(ctx, dir, "zh")
	if !st.Running || !st.Managed || !st.TokenKnown || st.OpenURL != localHubURL(hub.port(t))+"/#token=web-secret" {
		t.Fatalf("managed: %+v", st)
	}
	b, _ := json.Marshal(st)
	if strings.Contains(string(b), "bot-secret") {
		t.Fatalf("bot token leaked into the status: %s", b)
	}
	// apply with nothing filled in: url and bot token come from hub.json
	store := newMemStore()
	res := ApplyWizard(ctx, WizardEnv{Store: store, DataDir: dir, CLIs: testCLIs},
		WizardRequest{Platform: "cloud_web", Roles: []WizardRole{{Name: "Juzi"}}})
	if !res.OK || store.projects["juzi"] == nil || store.projects["juzi"].Options["token"] != "bot-secret" {
		t.Fatalf("apply from local hub: %+v", res)
	}
	// a different address does not get the local token
	if u, tok := resolveCloudWeb(dir, "http://10.0.0.9:8790", ""); u != "http://10.0.0.9:8790" || tok != "" {
		t.Fatalf("resolve other: %q %q", u, tok)
	}
}

func TestSetupCloudWebHTTP(t *testing.T) {
	noLocalHub(t)
	hub := fakeHub(t, "bot-secret")
	dir := t.TempDir()
	m := NewManagementServer(0, "tok", nil)
	m.SetWizard(newMemStore(), dir)
	ts := httptest.NewServer(m.buildHandler(http.NewServeMux()))
	defer ts.Close()
	post := func(path, body string) map[string]any {
		req, _ := http.NewRequest("POST", ts.URL+"/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out
	}
	d := post("/setup/cloudweb/test", `{"url":"`+hub.URL+`","token":"bot-secret","lang":"zh"}`)["data"].(map[string]any)
	if d["ok"] != true {
		t.Fatalf("test ok: %v", d)
	}
	d = post("/setup/cloudweb/test", `{"url":"`+hub.URL+`","token":"nope","lang":"zh"}`)["data"].(map[string]any)
	if d["code"] != "unauthorized" || d["fix"] == "" {
		t.Fatalf("test 401: %v", d)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/setup/cloudweb/local?lang=en", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("local: %v %v", resp, err)
	}
	resp.Body.Close()
}
