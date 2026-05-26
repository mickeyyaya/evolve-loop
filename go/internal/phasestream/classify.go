package phasestream

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	toolUseInputBytes   = 200
	toolResultHeadBytes = 200
	toolResultTailBytes = 100
	unknownExcerptBytes = 500
)

// Classifier turns raw CLI output lines into normalized envelopes. It is
// stateful: stream_event deltas accumulate and are coalesced into a
// single progress tick via FlushProgress (called per tail poll-batch),
// and seq is monotonic across all emitted envelopes.
//
// Not safe for concurrent use — the normalizer owns one Classifier per
// phase and feeds it from a single goroutine.
type Classifier struct {
	src     Source
	traceID string
	now     func() time.Time

	seq             int64
	pendingDeltas   int
	cumOutputTokens int64
	lastFlush       time.Time
}

// NewClassifier builds a Classifier. now defaults to time.Now.
func NewClassifier(src Source, traceID string, now func() time.Time) *Classifier {
	if now == nil {
		now = time.Now
	}
	return &Classifier{src: src, traceID: traceID, now: now, lastFlush: now()}
}

// Emit builds an envelope for a normalizer-originated event (e.g. a stall
// incident) using the same monotonic seq + source as line events, so the
// unified stream stays gap-free across both line- and rule-events.
func (c *Classifier) Emit(kind Kind, sev Severity, data map[string]any) Envelope {
	return c.newEnvelope(kind, sev, data)
}

func (c *Classifier) newEnvelope(kind Kind, sev Severity, data map[string]any) Envelope {
	c.seq++
	return Envelope{
		SchemaVersion: SchemaVersion,
		Seq:           c.seq,
		TS:            c.now().UTC().Format(time.RFC3339),
		TraceID:       c.traceID,
		Source:        c.src,
		Kind:          kind,
		Severity:      sev,
		Data:          data,
	}
}

// Line classifies one stdout line into zero or more envelopes. Returns
// nil for noise (stream_event deltas, system:init, blank/spinner/border
// lines) — stream_event growth surfaces later via FlushProgress.
func (c *Classifier) Line(raw []byte) []Envelope {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	if trimmed[0] == '{' {
		var probe struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(trimmed), &probe); err == nil && probe.Type != "" {
			return c.classifyJSON(probe.Type, []byte(trimmed))
		}
		// Looks like JSON but won't parse / has no type — fall through to
		// plaintext so we never silently lose a line.
	}
	return c.classifyPlain(trimmed)
}

// Stderr classifies one stderr line. Emits an infra_failure envelope
// when an infrastructure marker is present, otherwise drops the line
// (stderr noise carries no signal for the consumers).
func (c *Classifier) Stderr(raw []byte) []Envelope {
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return nil
	}
	if marker := detectInfraMarker(line); marker != "" {
		return []Envelope{c.newEnvelope(KindInfraFailure, SeverityIncident, map[string]any{
			"marker":  marker,
			"source":  "stderr",
			"excerpt": truncateInline(line, 400),
		})}
	}
	return nil
}

// FlushProgress emits one coalesced progress envelope when stream_event
// deltas have accumulated since the last flush, and resets the counter.
// Returns ok=false when nothing is pending (no-op). The normalizer calls
// this once per tail poll-batch so liveness stays fresh without the
// per-delta payload noise.
func (c *Classifier) FlushProgress() (Envelope, bool) {
	if c.pendingDeltas == 0 {
		return Envelope{}, false
	}
	now := c.now()
	env := c.newEnvelope(KindProgress, SeverityInfo, map[string]any{
		"delta_count":       int64(c.pendingDeltas),
		"cum_output_tokens": c.cumOutputTokens,
		"since_ms":          now.Sub(c.lastFlush).Milliseconds(),
	})
	c.pendingDeltas = 0
	c.lastFlush = now
	return env, true
}

func (c *Classifier) classifyJSON(typ string, raw []byte) []Envelope {
	switch typ {
	case "stream_event":
		// The dominant noise source. Coalesced into a progress tick
		// rather than dropped, so the observer keeps a liveness signal.
		c.pendingDeltas++
		return nil
	case "result":
		return []Envelope{c.formatResult(raw)}
	case "assistant":
		return c.formatAssistant(raw)
	case "user":
		return c.formatUser(raw)
	case "rate_limit_event":
		return []Envelope{c.newEnvelope(KindRateLimit, SeverityWarn, map[string]any{
			"raw": truncateInline(string(raw), 400),
		})}
	case "system":
		return c.formatSystem(raw)
	default:
		return []Envelope{c.newEnvelope(KindUnknown, SeverityInfo, map[string]any{
			"type": typ,
			"raw":  truncateInline(string(raw), unknownExcerptBytes),
		})}
	}
}

type resultEvent struct {
	IsError      bool    `json:"is_error"`
	DurationMS   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func (c *Classifier) formatResult(raw []byte) Envelope {
	var r resultEvent
	_ = json.Unmarshal(raw, &r)
	return c.newEnvelope(KindResult, SeverityInfo, map[string]any{
		"cost_usd": r.TotalCostUSD,
		"tokens": map[string]any{
			"in":      r.Usage.InputTokens,
			"out":     r.Usage.OutputTokens,
			"cache_r": r.Usage.CacheReadInputTokens,
			"cache_c": r.Usage.CacheCreationInputTokens,
		},
		"num_turns":   int64(r.NumTurns),
		"duration_ms": int64(r.DurationMS),
		"is_error":    r.IsError,
	})
}

type assistantEvent struct {
	Message struct {
		Content []assistantContent `json:"content"`
	} `json:"message"`
}

type assistantContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
}

func (c *Classifier) formatAssistant(raw []byte) []Envelope {
	var a assistantEvent
	if err := json.Unmarshal(raw, &a); err != nil {
		return []Envelope{c.newEnvelope(KindAssistantText, SeverityInfo, map[string]any{
			"text": truncateInline(string(raw), unknownExcerptBytes),
		})}
	}
	var out []Envelope
	for _, blk := range a.Message.Content {
		switch blk.Type {
		case "text":
			if blk.Text == "" {
				continue
			}
			out = append(out, c.newEnvelope(KindAssistantText, SeverityInfo, map[string]any{"text": blk.Text}))
		case "thinking":
			if blk.Thinking == "" {
				continue
			}
			out = append(out, c.newEnvelope(KindThinking, SeverityInfo, map[string]any{"text": blk.Thinking}))
		case "tool_use":
			out = append(out, c.formatToolUse(blk))
		}
	}
	return out
}

// formatToolUse special-cases interactive tools BEFORE the generic
// 200-byte input clamp: AskUserQuestion / ExitPlanMode carry the
// question, options, recommendation, or plan — that IS the signal, so
// it must survive at full fidelity (ADR-0020).
func (c *Classifier) formatToolUse(blk assistantContent) Envelope {
	switch blk.Name {
	case "AskUserQuestion":
		return c.newEnvelope(KindInteraction, SeverityInfo, map[string]any{
			"mode":        "ask_user_question",
			"tool_use_id": blk.ID,
			"input":       decodeFull(blk.Input),
		})
	case "ExitPlanMode":
		return c.newEnvelope(KindInteraction, SeverityInfo, map[string]any{
			"mode":        "exit_plan_mode",
			"tool_use_id": blk.ID,
			"input":       decodeFull(blk.Input),
		})
	default:
		return c.newEnvelope(KindToolUse, SeverityInfo, map[string]any{
			"name":          blk.Name,
			"id":            blk.ID,
			"input_excerpt": truncateInline(string(blk.Input), toolUseInputBytes),
		})
	}
}

type userEvent struct {
	Message struct {
		Content []userContent `json:"content"`
	} `json:"message"`
}

type userContent struct {
	Type      string          `json:"type"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

func (c *Classifier) formatUser(raw []byte) []Envelope {
	var u userEvent
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil
	}
	var out []Envelope
	for _, blk := range u.Message.Content {
		if blk.Type != "tool_result" {
			continue
		}
		sev := SeverityInfo
		if blk.IsError {
			sev = SeverityWarn
		}
		var asStr string
		if err := json.Unmarshal(blk.Content, &asStr); err != nil {
			asStr = string(blk.Content)
		}
		out = append(out, c.newEnvelope(KindToolResult, sev, map[string]any{
			"tool_use_id": blk.ToolUseID,
			"is_error":    blk.IsError,
			"excerpt":     truncateMiddle(asStr, toolResultHeadBytes, toolResultTailBytes),
		}))
	}
	return out
}

type systemEvent struct {
	Subtype   string `json:"subtype"`
	HookName  string `json:"hook_name"`
	HookEvent string `json:"hook_event"`
	ExitCode  *int   `json:"exit_code"`
	Outcome   string `json:"outcome"`
}

func (c *Classifier) formatSystem(raw []byte) []Envelope {
	var s systemEvent
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	if s.Subtype == "init" {
		return nil // session bootstrap — pure noise
	}
	data := map[string]any{"subtype": s.Subtype}
	if s.HookName != "" {
		data["hook_name"] = s.HookName
		data["hook_event"] = s.HookEvent
		data["outcome"] = s.Outcome
	}
	if s.ExitCode != nil {
		data["exit_code"] = int64(*s.ExitCode)
	}
	return []Envelope{c.newEnvelope(KindSystemHook, SeverityInfo, data)}
}

func (c *Classifier) classifyPlain(line string) []Envelope {
	if isNoise(line) {
		return nil
	}
	return []Envelope{c.newEnvelope(KindAssistantText, SeverityInfo, map[string]any{"text": line})}
}

// decodeFull parses a tool-use input blob into a generic structure with
// no truncation. Falls back to the raw string if it isn't valid JSON.
func decodeFull(raw json.RawMessage) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

// ---- noise detection (ported from logfilter/plaintext.go) ----

func isNoise(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	allSpinner, allBorder := true, true
	for _, r := range trimmed {
		if !isSpinnerRune(r) && !unicode.IsSpace(r) {
			allSpinner = false
		}
		if !isBorderRune(r) && !unicode.IsSpace(r) {
			allBorder = false
		}
		if !allSpinner && !allBorder {
			return false
		}
	}
	return allSpinner || allBorder
}

func isSpinnerRune(r rune) bool {
	switch r {
	case '⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏':
		return true
	case '|', '/', '-', '\\':
		return true
	}
	return false
}

func isBorderRune(r rune) bool {
	switch r {
	case '╭', '╮', '╰', '╯', '─', '│', '┌', '┐', '└', '┘', '═', '║':
		return true
	}
	return false
}

// ---- truncation helpers (ported from logfilter/streamjson.go) ----

func truncateInline(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("… (%d bytes elided)", len(s)-n)
}

func truncateMiddle(s string, head, tail int) string {
	if len(s) <= head+tail+32 {
		return s
	}
	elided := len(s) - head - tail
	return s[:head] + fmt.Sprintf("… (%d bytes elided) …", elided) + s[len(s)-tail:]
}
