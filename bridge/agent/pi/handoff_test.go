package pi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var piSessionFixture = []string{
	`{"type":"session","version":3,"id":"sess-pi-uuid-001","timestamp":"2026-10-23T01:00:00.000Z"}`,
	`{"type":"message","id":"m1","parentId":"sess-pi-uuid-001","timestamp":"2026-10-23T01:01:00.000Z","message":{"role":"user","content":"请帮我看一下代码架构"}}`,
	`{"type":"message","id":"m2","parentId":"m1","timestamp":"2026-10-23T01:01:30.000Z","message":{"role":"assistant","content":[{"type":"text","text":"没问题，我正在分析项目结构"}],"usage":{"input":1200,"output":150,"cacheRead":300,"cacheWrite":0}}}`,
	`{"type":"message","id":"m3","parentId":"m2","timestamp":"2026-10-23T01:05:00.000Z","message":{"role":"user","content":[{"type":"text","text":"重点关注认证模块"}]}}`,
	`{"type":"message","id":"m4","parentId":"m3","timestamp":"2026-10-23T01:06:15.500Z","message":{"role":"assistant","content":"认证模块采用 JWT 实现，配置在 config.go 中","usage":{"input":2500,"output":400,"cacheRead":1000,"cacheWrite":0}}}`,
}

func TestParsePiHandoffNormal(t *testing.T) {
	h, err := parsePiHandoff(strings.NewReader(strings.Join(piSessionFixture, "\n")))
	if err != nil {
		t.Fatalf("parsePiHandoff failed: %v", err)
	}

	if h.CompactSummary != "" || !h.CompactedAt.IsZero() {
		t.Fatalf("expected empty compaction, got summary=%q at=%v", h.CompactSummary, h.CompactedAt)
	}

	// 2500 + 1000 = 3500 tokens
	if h.ContextTokens != 3500 {
		t.Fatalf("expected ContextTokens=3500, got %d", h.ContextTokens)
	}

	wantTime, _ := time.Parse(time.RFC3339Nano, "2026-10-23T01:06:15.500Z")
	if !h.LastActivity.Equal(wantTime) {
		t.Fatalf("expected LastActivity=%v, got %v", wantTime, h.LastActivity)
	}

	if len(h.Tail) != 4 {
		t.Fatalf("expected 4 tail entries, got %d", len(h.Tail))
	}

	if h.Tail[0].Role != "user" || h.Tail[0].Content != "请帮我看一下代码架构" {
		t.Fatalf("unexpected entry 0: %+v", h.Tail[0])
	}
	if h.Tail[1].Role != "assistant" || h.Tail[1].Content != "没问题，我正在分析项目结构" {
		t.Fatalf("unexpected entry 1: %+v", h.Tail[1])
	}
	if h.Tail[2].Role != "user" || h.Tail[2].Content != "重点关注认证模块" {
		t.Fatalf("unexpected entry 2: %+v", h.Tail[2])
	}
	if h.Tail[3].Role != "assistant" || h.Tail[3].Content != "认证模块采用 JWT 实现，配置在 config.go 中" {
		t.Fatalf("unexpected entry 3: %+v", h.Tail[3])
	}
}

func TestParsePiHandoffCompaction(t *testing.T) {
	lines := append([]string{}, piSessionFixture...)
	lines = append(lines,
		`{"type":"compaction","id":"c1","parentId":"m4","timestamp":"2026-10-23T01:10:00.000Z","summary":"用户询问项目架构并重点排查了 JWT 认证逻辑","firstKeptEntryId":"m5","tokensBefore":45000}`,
		`{"type":"message","id":"m5","parentId":"c1","timestamp":"2026-10-23T01:12:00.000Z","message":{"role":"user","content":"现在继续检查权限拦截"}}`,
		`{"type":"message","id":"m6","parentId":"m5","timestamp":"2026-10-23T01:13:20.000Z","message":{"role":"assistant","content":"已核查，RBAC 权限守卫已挂载完毕","usage":{"input":4200,"output":200,"cacheRead":0,"cacheWrite":0}}}`,
	)

	h, err := parsePiHandoff(strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatalf("parsePiHandoff failed: %v", err)
	}

	wantSummary := "用户询问项目架构并重点排查了 JWT 认证逻辑"
	if h.CompactSummary != wantSummary {
		t.Fatalf("expected summary %q, got %q", wantSummary, h.CompactSummary)
	}

	wantCompactedAt, _ := time.Parse(time.RFC3339Nano, "2026-10-23T01:10:00.000Z")
	if !h.CompactedAt.Equal(wantCompactedAt) {
		t.Fatalf("expected CompactedAt=%v, got %v", wantCompactedAt, h.CompactedAt)
	}

	if h.ContextTokens != 4200 {
		t.Fatalf("expected ContextTokens=4200, got %d", h.ContextTokens)
	}

	wantLastActivity, _ := time.Parse(time.RFC3339Nano, "2026-10-23T01:13:20.000Z")
	if !h.LastActivity.Equal(wantLastActivity) {
		t.Fatalf("expected LastActivity=%v, got %v", wantLastActivity, h.LastActivity)
	}

	// Earlier tail entries before compaction must be pruned; only m5 and m6 remain.
	if len(h.Tail) != 2 {
		t.Fatalf("expected 2 tail entries after compaction, got %d", len(h.Tail))
	}
	if h.Tail[0].Role != "user" || h.Tail[0].Content != "现在继续检查权限拦截" {
		t.Fatalf("unexpected entry 0: %+v", h.Tail[0])
	}
	if h.Tail[1].Role != "assistant" || h.Tail[1].Content != "已核查，RBAC 权限守卫已挂载完毕" {
		t.Fatalf("unexpected entry 1: %+v", h.Tail[1])
	}
}

func TestReadSessionHandoffFindsSession(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	workDir := filepath.Join(tempHome, "my-pi-project")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessDir := piSessionDir(workDir)
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}

	uuid := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	fileName := "2026-10-23T01-00-00_" + uuid + ".jsonl"
	filePath := filepath.Join(sessDir, fileName)

	if err := os.WriteFile(filePath, []byte(strings.Join(piSessionFixture, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	agent := &Agent{
		workDir: workDir,
	}

	h, err := agent.ReadSessionHandoff(uuid)
	if err != nil {
		t.Fatalf("ReadSessionHandoff(%q) failed: %v", uuid, err)
	}

	if h.Path != filePath {
		t.Fatalf("expected Path=%q, got %q", filePath, h.Path)
	}
	if h.ContextTokens != 3500 {
		t.Fatalf("expected ContextTokens=3500, got %d", h.ContextTokens)
	}
	if len(h.Tail) != 4 {
		t.Fatalf("expected 4 tail entries, got %d", len(h.Tail))
	}

	// Negative test: non-existent session
	if _, err := agent.ReadSessionHandoff("non-existent-session-id"); err == nil {
		t.Fatal("expected error for non-existent session, got nil")
	}
}
