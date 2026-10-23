package pi

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

var _ core.SessionHandoffReader = (*Agent)(nil)

// ReadSessionHandoff reads what a fresh context needs from a pi session's
// .jsonl file: the last model reply's time and prompt token size, the newest
// compaction summary, and the user/assistant dialogue after it.
func (a *Agent) ReadSessionHandoff(sessionID string) (*core.SessionHandoff, error) {
	a.mu.Lock()
	workDir := a.workDir
	a.mu.Unlock()

	var path string
	if filepath.IsAbs(sessionID) && strings.HasSuffix(sessionID, ".jsonl") {
		if _, err := os.Stat(sessionID); err == nil {
			path = sessionID
		}
	}
	if path == "" {
		sessDir := piSessionDir(workDir)
		if sessDir != "" {
			path = findSessionFile(sessDir, sessionID)
		}
	}
	if path == "" {
		return nil, fmt.Errorf("pi: session file not found for %s", sessionID)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pi: open session file: %w", err)
	}
	defer f.Close()

	h, err := parsePiHandoff(f)
	if err != nil {
		return nil, fmt.Errorf("pi: read session file: %w", err)
	}
	h.Path = path
	return h, nil
}

type piJSONLLine struct {
	Type         string          `json:"type"` // "session" | "message" | "compaction"
	ID           string          `json:"id"`
	ParentID     string          `json:"parentId"`
	Timestamp    string          `json:"timestamp"`
	Summary      string          `json:"summary"` // compaction entry
	TokensBefore int             `json:"tokensBefore"`
	Message      *piMessageEntry `json:"message"` // message entry
}

type piMessageEntry struct {
	Role      string          `json:"role"` // "user" | "assistant" | "tool"
	Content   json.RawMessage `json:"content"`
	Usage     *piUsageEntry   `json:"usage"`
	Timestamp string          `json:"timestamp"`
}

type piUsageEntry struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
}

func parsePiHandoff(r io.Reader) (*core.SessionHandoff, error) {
	var h core.SessionHandoff
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}
		var line piJSONLLine
		if err := json.Unmarshal(lineBytes, &line); err != nil {
			continue
		}

		switch line.Type {
		case "compaction":
			h.CompactSummary = line.Summary
			if ts, err := time.Parse(time.RFC3339Nano, line.Timestamp); err == nil {
				h.CompactedAt = ts
			} else if ts, err := time.Parse(time.RFC3339, line.Timestamp); err == nil {
				h.CompactedAt = ts
			}
			// When compaction happens, active context is rebuilt: earlier tail is pruned.
			h.Tail = nil
			h.ImageBytes = 0

		case "message":
			if line.Message == nil {
				continue
			}
			msg := line.Message
			role := msg.Role
			if role != "user" && role != "assistant" {
				continue
			}

			tsStr := line.Timestamp
			if tsStr == "" {
				tsStr = msg.Timestamp
			}
			var entryTS time.Time
			if tsStr != "" {
				if parsed, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
					entryTS = parsed
				} else if parsed, err := time.Parse(time.RFC3339, tsStr); err == nil {
					entryTS = parsed
				}
			}

			text := extractPiContentText(msg.Content)
			if text == "" {
				continue
			}

			if role == "assistant" {
				if !entryTS.IsZero() {
					h.LastActivity = entryTS
				}
				if msg.Usage != nil {
					tokens := msg.Usage.Input + msg.Usage.CacheRead
					if tokens > 0 {
						h.ContextTokens = tokens
					}
				}
			}

			h.Tail = append(h.Tail, core.HistoryEntry{
				Role:      role,
				Content:   text,
				Timestamp: entryTS,
			})
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return &h, nil
}

func extractPiContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Case 1: Plain string
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	// Case 2: Array of content parts
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var parts []string
		for _, part := range arr {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}
