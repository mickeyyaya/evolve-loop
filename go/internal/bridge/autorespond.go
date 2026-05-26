package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// autorespond.go — the fallback prompt-detection engine for interactive
// REPLs (Go port of lib/auto-respond.sh). --dangerously-skip-permissions
// is the default permission strategy; this is the safety net for prompts
// that escape the bypass (auth-recheck, rate-limit, model-deprecation,
// terminal-resize, trust prompts). Two layers, mirroring the bash:
//
//	decideAutoRespond — PURE: pane + manifest prompts + counts → (action, rc).
//	autoResponder.tick — EFFECTFUL: capture-pane → decide → send-keys /
//	                     escalation-report.
//
// Action/rc contract (consumed by runTmuxREPL):
//
//	"noop"            0   nothing matched
//	"send:<csv>"      1   caller already sent the keys (responded)
//	"extend:<secs>"   2   bump the artifact-poll deadline
//	"escalate:<name>" 85  policy=escalate / missing keys → abandon
//	"loop_guard:<n>"  86  same pattern matched >5× → abandon
const autoRespondLoopGuardLimit = 5

// decideAutoRespond is the pure decision: first interactive_prompts regex
// to match the pane wins; counts tracks per-pattern match frequency for
// the loop guard. Mirrors auto_respond_decide.
func decideAutoRespond(pane string, prompts []ManifestPrompt, counts map[string]int) (string, int) {
	for _, p := range prompts {
		if p.Regex == "" {
			continue
		}
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			continue
		}
		if !re.MatchString(pane) {
			continue
		}
		counts[p.Name]++
		if counts[p.Name] > autoRespondLoopGuardLimit {
			return "loop_guard:" + p.Name, 86
		}
		switch p.Policy {
		case "auto_respond":
			if p.ResponseKeys == "" {
				return "escalate:" + p.Name, 85
			}
			return "send:" + p.ResponseKeys, 1
		case "extend_timeout":
			if !allDigits(p.ResponseKeys) {
				return "escalate:" + p.Name, 85
			}
			return "extend:" + p.ResponseKeys, 2
		default: // escalate
			return "escalate:" + p.Name, 85
		}
	}
	return "noop", 0
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// autoResponder is the per-launch effectful wrapper: it owns the manifest
// prompt set + match counts + workspace for one *-tmux run.
type autoResponder struct {
	prompts   []ManifestPrompt
	workspace string
	cli       string
	counts    map[string]int
	deps      Deps
	human     bool // when true, deliver keys with human-input cadence
}

// newAutoResponder builds the responder from the CLI's embedded manifest.
// A missing/unreadable manifest yields an empty rule set (tick → noop).
// human engages the keystroke-plausibility send path.
func newAutoResponder(cli, workspace string, deps Deps, human bool) *autoResponder {
	var prompts []ManifestPrompt
	if m, err := LoadManifest(cli); err == nil {
		prompts = m.InteractivePrompts
	}
	return &autoResponder{prompts: prompts, workspace: workspace, cli: cli, counts: map[string]int{}, deps: deps, human: human}
}

// tick captures the pane, decides, and applies the effect (send-keys or
// escalation-report). Returns (action, rc) for runTmuxREPL's loop.
func (ar *autoResponder) tick(ctx context.Context, session string) (string, int) {
	pane, _ := ar.deps.Tmux.CapturePane(ctx, session, 0)
	action, rc := decideAutoRespond(pane, ar.prompts, ar.counts)
	switch rc {
	case 1:
		keysCSV := strings.TrimPrefix(action, "send:")
		if ar.human {
			humanReadingPause(ar.deps, pane)
			humanSendKeysCSV(ctx, ar.deps, session, keysCSV)
		} else {
			keys, enter := parseSendKeysCSV(keysCSV)
			_ = ar.deps.Tmux.SendKeys(ctx, session, keys, enter)
			fmt.Fprintf(ar.deps.Stderr, "[auto-respond] sent keys: %s\n", keysCSV)
		}
		return "", 1
	case 2:
		fmt.Fprintf(ar.deps.Stderr, "[auto-respond] extend_timeout signal: %s\n", action)
		return action, 2
	case 85:
		ar.writeEscalation(pane, strings.TrimPrefix(action, "escalate:"), "escalate", session)
		return "", 85
	case 86:
		ar.writeEscalation(pane, strings.TrimPrefix(action, "loop_guard:"), "loop_guard", session)
		return "", 86
	default:
		return "", 0
	}
}

// parseSendKeysCSV maps a comma-separated tmux key list to the
// (keys, enter) form of TmuxController.SendKeys. A trailing "Enter" token
// becomes enter=true; the remaining tokens join with spaces (the manifest
// patterns are single-key + Enter: "y,Enter", "Enter", "3,Enter").
func parseSendKeysCSV(csv string) (keys string, enter bool) {
	toks := strings.Split(csv, ",")
	var keyToks []string
	for _, tk := range toks {
		if tk == "Enter" {
			enter = true
			continue
		}
		keyToks = append(keyToks, tk)
	}
	return strings.Join(keyToks, " "), enter
}

// writeEscalation writes escalation-report.json from the final pane, the
// operator's repair trail (Go port of auto_respond_write_escalation_report).
func (ar *autoResponder) writeEscalation(pane, patternName, reason, session string) {
	report := struct {
		SchemaVersion int      `json:"schema_version"`
		CapturedAt    string   `json:"captured_at"`
		CLI           string   `json:"cli"`
		PatternName   string   `json:"pattern_name"`
		Reason        string   `json:"reason"`
		Session       string   `json:"session"`
		PaneTail      string   `json:"pane_tail"`
		NextSteps     []string `json:"next_steps"`
	}{
		SchemaVersion: 1,
		CapturedAt:    ar.deps.Now().UTC().Format("2006-01-02T15:04:05Z"),
		CLI:           ar.cli,
		PatternName:   patternName,
		Reason:        reason,
		Session:       session,
		PaneTail:      lastLines(pane, 30),
		NextSteps: []string{
			"Read pane_tail above; identify the prompt the agent is stuck on",
			"Run: evolve bridge add-rule --escalation=<this-file> --regex=R --response=KEYS",
			"Re-run the workflow; the bridge should now auto-respond to this prompt",
		},
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(filepath.Join(ar.workspace, "escalation-report.json"), b, 0o644)
	fmt.Fprintf(ar.deps.Stderr, "[auto-respond] escalation report written (pattern=%s reason=%s)\n", patternName, reason)
}

// lastLines returns the last n lines of s.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
