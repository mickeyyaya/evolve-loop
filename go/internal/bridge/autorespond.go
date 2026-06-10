package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
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
	suppressedOnce := "" // a fire-once prompt that matched but was already handled
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
		// A fire-once prompt (boot-time trust dialog) is handled a single time.
		// On later ticks the dismissed dialog lingers in the captured scrollback
		// and still matches; skip it rather than re-firing (which would trip the
		// loop guard and abandon the run). It does not count toward the guard.
		// Keep scanning so a genuinely-new prompt (e.g. per-edit approval) on the
		// same pane still fires; only if nothing else matches do we surface the
		// suppression (rc 0) so the caller can WARN once — never a silent skip.
		if p.Once && counts[p.Name] >= 1 {
			if suppressedOnce == "" {
				suppressedOnce = p.Name
			}
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
	// Nothing fired, but a fire-once prompt is still matching (its dismissed text
	// lingering in scrollback). rc 0 = no action, like noop; the distinct action
	// lets the caller WARN once so a genuinely-stuck dialog is diagnosable instead
	// of silently timing out.
	if suppressedOnce != "" {
		return "suppress_once:" + suppressedOnce, 0
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
	// scrollback is the capture-pane depth: 0 for visible-pane CLIs (claude),
	// >0 for alt-screen CLIs (codex/agy) whose bare visible pane is blank.
	scrollback int
	// suppressLogged tracks fire-once prompts we have already WARNed about, so a
	// lingering-in-scrollback once-prompt is surfaced exactly once, not every poll.
	suppressLogged map[string]bool
	// Interaction telemetry (ADR-0045 I1): rec records every send with its
	// deterministically-resolved outcome ("prompt-pattern cleared on next
	// capture"). nil = no telemetry (the recipe adapter's capability runs are
	// outside the phase-interaction surface). phase/cycle stamp the events;
	// pending is the one in-flight send awaiting resolution.
	rec     *interaction.Recorder
	phase   string
	cycle   int
	pending *pendingAutoRespond
}

// pendingAutoRespond is one auto-respond send awaiting its outcome: the rule
// that fired, its compiled pattern (re-checked against the NEXT capture), and
// the send timestamp for latency.
type pendingAutoRespond struct {
	rule string
	re   *regexp.Regexp
	keys string
	at   time.Time
}

// newAutoResponder builds the responder from the CLI's embedded manifest.
// A missing/unreadable manifest yields an empty rule set (tick → noop).
// human engages the keystroke-plausibility send path.
func newAutoResponder(cli, workspace string, deps Deps, human bool, scrollback int) *autoResponder {
	var prompts []ManifestPrompt
	if m, err := LoadManifest(cli); err == nil {
		prompts = m.InteractivePrompts
	}
	return &autoResponder{prompts: prompts, workspace: workspace, cli: cli, counts: map[string]int{}, deps: deps, human: human, scrollback: scrollback, suppressLogged: map[string]bool{}}
}

// tick captures the pane, decides, and applies the effect (send-keys or
// escalation-report). Returns (action, rc) for runTmuxREPL's loop.
func (ar *autoResponder) tick(ctx context.Context, session string) (string, int) {
	pane, _ := ar.deps.Tmux.CapturePane(ctx, session, ar.scrollback)
	// ADR-0045 I1: resolve the in-flight send against THIS capture before
	// deciding — the pattern no longer matching is the deterministic
	// "it worked" signal ("prompt-pattern cleared on next capture").
	ar.resolvePending(pane)
	var prevCounts map[string]int
	if ar.rec != nil {
		prevCounts = make(map[string]int, len(ar.counts))
		for k, v := range ar.counts {
			prevCounts[k] = v
		}
	}
	action, rc := decideAutoRespond(pane, ar.prompts, ar.counts)
	switch rc {
	case 1:
		keysCSV := strings.TrimPrefix(action, "send:")
		if ar.human {
			humanReadingPause(ar.deps, pane)
			humanSendKeysCSV(ctx, ar.deps, session, keysCSV)
		} else {
			sendKeySequence(ctx, ar.deps, session, keysCSV)
			fmt.Fprintf(ar.deps.Stderr, "[auto-respond] sent keys: %s\n", keysCSV)
		}
		ar.openPending(prevCounts, keysCSV)
		return "", 1
	case 2:
		fmt.Fprintf(ar.deps.Stderr, "[auto-respond] extend_timeout signal: %s\n", action)
		return action, 2
	case 85:
		ar.writeEscalation(pane, strings.TrimPrefix(action, "escalate:"), "escalate", session)
		return "", 85
	case 86:
		// The guard trips because the SAME pattern kept matching — the
		// pending send demonstrably did not clear it.
		name := strings.TrimPrefix(action, "loop_guard:")
		if ar.pending != nil && ar.pending.rule == name {
			ar.record(ar.pending, interaction.ResultNoEffect)
			ar.pending = nil
		}
		ar.writeEscalation(pane, name, "loop_guard", session)
		return "", 86
	default:
		// A fire-once prompt is still matching after its single response (its
		// dismissed text lingering in scrollback). No action — but WARN once so a
		// genuinely-unanswered dialog is diagnosable rather than a silent timeout.
		if name, ok := strings.CutPrefix(action, "suppress_once:"); ok {
			// Lingering-in-scrollback is genuinely indistinguishable from an
			// unanswered dialog at this layer — record the honest bucket, not
			// a guessed success (mirrors the WARN below).
			if ar.pending != nil && ar.pending.rule == name {
				ar.record(ar.pending, interaction.ResultSuppressedLingering)
				ar.pending = nil
			}
			if !ar.suppressLogged[name] {
				ar.suppressLogged[name] = true
				fmt.Fprintf(ar.deps.Stderr, "[auto-respond] %s already handled; suppressing re-fire (dialog lingering in scrollback). "+
					"If the agent stalls, the dialog may still be unanswered.\n", name)
			}
		}
		return "", 0
	}
}

// resolvePending re-checks the in-flight send against the current capture and
// records prompt_cleared when its pattern no longer matches. A still-matching
// pattern stays pending — decide() may re-fire it (→ no_effect) or suppress
// it (→ suppressed_lingering); a nil re (recompile failed — unreachable for a
// pattern that just matched) is left for flushPending so nothing is fabricated.
func (ar *autoResponder) resolvePending(pane string) {
	if ar.pending == nil || ar.pending.re == nil {
		return
	}
	if !ar.pending.re.MatchString(pane) {
		ar.record(ar.pending, interaction.ResultPromptCleared)
		ar.pending = nil
	}
}

// openPending resolves a still-matching predecessor honestly (its pattern was
// still on screen when another send was needed ⇒ no_effect) and opens the
// outcome window for the send that just fired. prevCounts identifies the rule
// decideAutoRespond matched — the only counter that moved — without changing
// decideAutoRespond's pinned signature. No-op without a recorder, so the
// recipe adapter's capability runs pay zero tracking cost.
func (ar *autoResponder) openPending(prevCounts map[string]int, keysCSV string) {
	if ar.rec == nil {
		return
	}
	if ar.pending != nil {
		ar.record(ar.pending, interaction.ResultNoEffect)
		ar.pending = nil
	}
	name := ""
	for n, c := range ar.counts {
		if c > prevCounts[n] {
			name = n
			break
		}
	}
	if name == "" {
		return // defensive: a send without a counted rule is unreachable
	}
	var re *regexp.Regexp
	for _, p := range ar.prompts {
		if p.Name == name {
			re, _ = regexp.Compile(p.Regex) // compiled in decide already (it matched)
			break
		}
	}
	ar.pending = &pendingAutoRespond{rule: name, re: re, keys: keysCSV, at: ar.deps.Now()}
}

// flushPending resolves an in-flight auto-respond send the run is ending on
// (no further capture will arrive): record run_ended, never silence.
func (ar *autoResponder) flushPending() {
	if ar.pending == nil {
		return
	}
	ar.record(ar.pending, interaction.ResultRunEnded)
	ar.pending = nil
}

// record emits one resolved auto-respond outcome through the I1 chokepoint.
func (ar *autoResponder) record(p *pendingAutoRespond, result string) {
	ar.rec.Record(interaction.Outcome{
		Event: interaction.Event{
			Kind:    interaction.KindAutoRespond,
			Phase:   ar.phase,
			Cycle:   ar.cycle,
			Trigger: "prompt_matched",
			Payload: p.keys,
			RuleID:  p.rule,
		},
		Result:    result,
		LatencyMS: ar.deps.Now().Sub(p.at).Milliseconds(),
	})
}

// autoRespondInterKeyPause spaces out the keystrokes of a multi-step response
// so the inner CLI's TUI gets a render frame between them. claude's multi-
// select navigation (toggle → Right to Submit → Enter) is unreliable when the
// three keys arrive as one rapid burst — the cursor move lands before the
// toggle has re-rendered — but reliable once paced. Verified 2026-05-26: a
// zero-gap burst intermittently failed to submit; a 500 ms gap submitted on
// every run. The pause is delivered via Deps.Sleep, so the deterministic tests
// (no-op / scaled Sleep) stay fast and only a real launch waits.
const autoRespondInterKeyPause = 500 * time.Millisecond

// sendKeySequence sends each comma-separated key token to the REPL as its own
// keystroke, in order, pausing between them — so a multi-step response like
// "Enter,Right,Enter" (claude's multi-select: toggle the highlighted checkbox
// → Right to the Submit tab → Enter to submit) arrives as three distinct,
// paced keypresses instead of being collapsed or bursted. An "Enter" token
// sends a bare Enter; any other non-empty token sends that tmux key/text with
// no trailing Enter.
//
// The old parseSendKeysCSV collapsed every Enter into a single trailing Enter,
// which would submit a multi-select with nothing selected.
func sendKeySequence(ctx context.Context, deps Deps, session, csv string) {
	first := true
	for _, tok := range strings.Split(csv, ",") {
		if tok == "" {
			continue // empty token (e.g. "y,,Enter") → no keystroke
		}
		if !first {
			deps.Sleep(autoRespondInterKeyPause)
		}
		if tok == "Enter" {
			_ = deps.Tmux.SendKeys(ctx, session, "", true)
		} else {
			_ = deps.Tmux.SendKeys(ctx, session, tok, false)
		}
		first = false
	}
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
