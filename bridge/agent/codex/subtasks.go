package codex

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/chenhg5/cc-connect/core"
)

var _ core.SubTaskReporter = (*codexSession)(nil)
var _ core.SubTaskReporter = (*appServerSession)(nil)

// Both transports carry the same collaboration record, with different casing.
// Only receiver IDs and native agent states are public; prompts, messages and
// the collaboration tool's own ID/status never become child rows.
func updateCollaborationSubTasks(table *core.SubTaskTracker, item map[string]any) {
	tool, _ := item["tool"].(string)
	tool = strings.ToLower(strings.ReplaceAll(tool, "_", ""))
	switch tool {
	case "spawnagent", "wait", "waitagent", "sendinput", "resumeagent", "closeagent", "sendmessage", "followuptask", "interruptagent", "listagents":
	default:
		return
	}
	states, _ := item["agentsStates"].(map[string]any)
	if states == nil {
		states, _ = item["agents_states"].(map[string]any)
	}
	ids, _ := item["receiverThreadIds"].([]any)
	if ids == nil {
		ids, _ = item["receiver_thread_ids"].([]any)
	}
	// listAgents may report identities only as state-map keys.
	seen := map[string]bool{}
	for _, raw := range ids {
		if id, ok := raw.(string); ok {
			seen[id] = true
		}
	}
	var extra []string
	for id := range states {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		ids = append(ids, id)
	}
	sender, _ := item["senderThreadId"].(string)
	if sender == "" {
		sender, _ = item["sender_thread_id"].(string)
	}
	for _, raw := range ids {
		id, _ := raw.(string)
		if id == "" || id == sender {
			continue
		}
		state, _ := states[id].(map[string]any)
		status, _ := state["status"].(string)
		mapped, ok := codexSubTaskStatus(status)
		if !ok {
			// A successful spawn with a real child ID establishes a running
			// child, but a returned wait/send/close does not prove its state.
			callStatus, _ := item["status"].(string)
			if tool != "spawnagent" || callStatus != "completed" || state != nil {
				continue
			}
			mapped = core.SubTaskRunning
		}
		table.UpdateSubTask(core.SubTask{ID: id, Status: mapped, Background: true}, tool == "resumeagent" && mapped == core.SubTaskRunning)
	}
}

func codexSubTaskStatus(status string) (core.SubTaskStatus, bool) {
	switch strings.ToLower(strings.ReplaceAll(status, "_", "")) {
	case "pendinginit", "running":
		return core.SubTaskRunning, true
	case "completed":
		return core.SubTaskCompleted, true
	case "errored":
		return core.SubTaskFailed, true
	case "interrupted", "shutdown", "notfound":
		return core.SubTaskStopped, true
	default:
		return "", false
	}
}

// Observe known children's turns before the parent-only output guard. Never
// forward child text or let a child turn finish the parent's turn.
func (s *appServerSession) handleChildNotification(method string, raw json.RawMessage) bool {
	var n struct {
		ThreadID string `json:"threadId"`
		Turn     struct {
			Status string `json:"status"`
		} `json:"turn"`
	}
	if json.Unmarshal(raw, &n) != nil || n.ThreadID == "" || n.ThreadID == s.CurrentSessionID() || !s.HasSubTask(n.ThreadID) {
		return false
	}
	var status core.SubTaskStatus
	switch method {
	case "turn/started":
		status = core.SubTaskRunning
	case "turn/completed":
		switch n.Turn.Status {
		case "completed":
			status = core.SubTaskCompleted
		case "failed":
			status = core.SubTaskFailed
		case "interrupted":
			status = core.SubTaskStopped
		default:
			return true
		}
	default:
		return true
	}
	s.UpdateSubTask(core.SubTask{ID: n.ThreadID, Status: status, Background: true}, method == "turn/started")
	return true
}
