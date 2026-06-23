package logfilter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/textutil"
)

const (
	toolResultHeadBytes = 200
	toolResultTailBytes = 100
	toolUseInputBytes   = 200
)

// classifyJSON inspects one line. Returns (handled=true) when the line
// is recognized as a stream-json envelope and writes its filtered form
// into the second return. (handled=false) means the line is not JSON;
// caller routes to the plain-text path.
//
// formatted="" with handled=true is legal — it means "drop this line".
func classifyJSON(line []byte) (handled bool, formatted string) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		// Blank line — neither JSON nor signal; drop.
		return true, ""
	}
	if trimmed[0] != '{' {
		return false, ""
	}
	var probe envelope
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		// Looks like JSON but won't parse — pass through to plain-text
		// so we never silently lose a malformed line.
		return false, ""
	}
	if probe.Type == "" {
		return false, ""
	}
	return true, formatEvent(probe.Type, []byte(trimmed))
}

// envelope is the minimal probe used to route by .type.
type envelope struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}

func formatEvent(eventType string, raw []byte) string {
	switch eventType {
	case "stream_event":
		// Entirely redundant with the assembled assistant event.
		return ""
	case "system":
		return formatSystem(raw)
	case "assistant":
		return formatAssistant(raw)
	case "user":
		return formatUser(raw)
	case "result":
		return formatResult(raw)
	case "rate_limit_event":
		return formatRateLimit(raw)
	default:
		// Unknown — keep raw line so we never silently drop signal.
		return "[unknown:" + eventType + "] " + textutil.TruncateInline(string(raw), 500)
	}
}

// ----- system events -----

type systemEvent struct {
	Subtype   string `json:"subtype"`
	HookName  string `json:"hook_name"`
	HookEvent string `json:"hook_event"`
	ExitCode  *int   `json:"exit_code"`
	Outcome   string `json:"outcome"`
	Status    string `json:"status"`
}

func formatSystem(raw []byte) string {
	var s systemEvent
	if err := json.Unmarshal(raw, &s); err != nil {
		return "[system] " + textutil.TruncateInline(string(raw), 500)
	}
	switch s.Subtype {
	case "init":
		return ""
	case "hook_started":
		return fmt.Sprintf("[hook] started name=%s event=%s", s.HookName, s.HookEvent)
	case "hook_response":
		ec := "?"
		if s.ExitCode != nil {
			ec = fmt.Sprintf("%d", *s.ExitCode)
		}
		return fmt.Sprintf("[hook] response name=%s event=%s exit=%s outcome=%s",
			s.HookName, s.HookEvent, ec, s.Outcome)
	case "status":
		return "[status] " + s.Status
	default:
		// Unknown system subtype — keep one compressed line.
		return "[system:" + s.Subtype + "] " + textutil.TruncateInline(string(raw), 400)
	}
}

// ----- assistant events -----

type assistantEvent struct {
	Message struct {
		ID      string             `json:"id"`
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

func formatAssistant(raw []byte) string {
	var a assistantEvent
	if err := json.Unmarshal(raw, &a); err != nil {
		return "[assistant] " + textutil.TruncateInline(string(raw), 500)
	}
	var parts []string
	for _, c := range a.Message.Content {
		switch c.Type {
		case "text":
			if c.Text == "" {
				continue
			}
			parts = append(parts, "[assistant] "+c.Text)
		case "thinking":
			if c.Thinking == "" {
				continue
			}
			parts = append(parts, "[thinking] "+c.Thinking)
		case "tool_use":
			parts = append(parts, fmt.Sprintf("[tool_use name=%s id=%s] %s",
				c.Name, c.ID, textutil.TruncateInline(string(c.Input), toolUseInputBytes)))
		default:
			parts = append(parts, "[assistant:"+c.Type+"]")
		}
	}
	return strings.Join(parts, "\n")
}

// ----- user / tool_result events -----

type userEvent struct {
	Message struct {
		Content []userContent `json:"content"`
	} `json:"message"`
}

type userContent struct {
	Type      string          `json:"type"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

func formatUser(raw []byte) string {
	var u userEvent
	if err := json.Unmarshal(raw, &u); err != nil {
		return "[user] " + textutil.TruncateInline(string(raw), 500)
	}
	var parts []string
	for _, c := range u.Message.Content {
		if c.Type != "tool_result" {
			parts = append(parts, "[user:"+c.Type+"]")
			continue
		}
		// Content may be a JSON string or a nested array/object — render
		// as string and truncate either way.
		var asStr string
		if err := json.Unmarshal(c.Content, &asStr); err != nil {
			asStr = string(c.Content)
		}
		parts = append(parts, fmt.Sprintf("[tool_result id=%s] %s",
			c.ToolUseID, textutil.TruncateMiddle(asStr, toolResultHeadBytes, toolResultTailBytes)))
	}
	return strings.Join(parts, "\n")
}

// ----- result + rate_limit -----

type resultEvent struct {
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	DurationMS   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Result       string  `json:"result"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func formatResult(raw []byte) string {
	var r resultEvent
	if err := json.Unmarshal(raw, &r); err != nil {
		return "[result] " + textutil.TruncateInline(string(raw), 500)
	}
	header := fmt.Sprintf("[result] subtype=%s error=%t turns=%d cost=$%g duration=%dms tokens(in=%d out=%d cache_r=%d cache_c=%d)",
		r.Subtype, r.IsError, r.NumTurns, r.TotalCostUSD, r.DurationMS,
		r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheReadInputTokens, r.Usage.CacheCreationInputTokens)
	if r.Result != "" {
		return header + "\n" + r.Result
	}
	return header
}

func formatRateLimit(raw []byte) string {
	// Keep one compact line — rate_limit events are tiny but useful.
	return "[rate_limit] " + textutil.TruncateInline(string(raw), 400)
}
