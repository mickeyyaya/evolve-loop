package llmcalls

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	TurnsFilename = "llm-turns.ndjson"
	MaxTurnLines  = 500
)

var marshalTurn = json.Marshal

type Turn struct {
	Index        int
	TS           string
	Tokens       cyclestate.TokenUsage
	PromptTokens int
	Tools        []string
}

type turnRecord struct {
	CallID       string                `json:"call_id"`
	Turn         int                   `json:"turn,omitempty"`
	TS           string                `json:"ts,omitempty"`
	Tokens       cyclestate.TokenUsage `json:"tokens"`
	PromptTokens int                   `json:"prompt_tokens"`
	Tools        []string              `json:"tools,omitempty"`
	Truncated    int                   `json:"truncated,omitempty"`
}

func TurnsPath(workspace string) string { return filepath.Join(workspace, TurnsFilename) }

func AppendTurns(workspace, callID string, turns []Turn) error {
	if len(turns) == 0 {
		return nil
	}
	kept := turns[:min(len(turns), MaxTurnLines)]
	lines := make([]turnRecord, 0, len(kept)+1)
	for _, t := range kept {
		lines = append(lines, turnRecord{CallID: callID, Turn: t.Index, TS: t.TS, Tokens: t.Tokens, PromptTokens: t.PromptTokens, Tools: t.Tools})
	}
	if dropped := len(turns) - len(kept); dropped > 0 {
		lines = append(lines, turnRecord{CallID: callID, Truncated: dropped})
	}
	var payload []byte
	for i, ln := range lines {
		raw, err := marshalTurn(ln)
		if err != nil {
			return fmt.Errorf("marshal turn line: %w", err)
		}
		if i > 0 {
			payload = append(payload, '\n')
		}
		payload = append(payload, raw...)
	}
	path := TurnsPath(workspace)
	if err := flock.WithPathLock(path, func() error { return appendLineLocked(path, payload) }); err != nil {
		return fmt.Errorf("append turn sidecar %q: %w", path, err)
	}
	return nil
}
