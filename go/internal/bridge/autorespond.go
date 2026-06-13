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

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
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

// agentDiffLineRE marks a captured line as agent-authored edit content: a
// numbered diff line ("   224 +\tpane := ...") as rendered in the codex/claude
// editor view, or a bare unified-diff content line ("+text" / "-text") from
// lingering patch scrollback. CLI chrome — prompts, dialogs, rate-limit banners
// — is never diff-prefixed, so these lines carry the agent's content (which can
// contain prompt-shaped text the agent is writing ABOUT, e.g. a clihealth
// rate-limit fixture) and must not drive interactive-prompt matching.
// soak #4 cycle 314: an agent editing the clihealth parser typed "You've hit
// your usage limit" into a test fixture and the escalate rule benched codex.
var agentDiffLineRE = regexp.MustCompile(`^[ \t]*(?:\d+[ \t]+)?[+-]`)

func isAgentDiffLine(ln string) bool {
	trimmed := strings.TrimLeft(ln, " \t")
	if strings.HasPrefix(trimmed, "+++") || strings.HasPrefix(trimmed, "---") {
		return false
	}
	return agentDiffLineRE.MatchString(ln)
}

// stripAgentDiffLines removes diff content lines so prompt matching sees only
// CLI chrome. Non-diff lines — including the CLI's real banners and prompts —
// pass through unchanged.
func stripAgentDiffLines(pane string) string {
	lines := strings.Split(pane, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		if isAgentDiffLine(ln) {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

// decideAutoRespond is the pure decision: first interactive_prompts regex
// to match the pane wins; counts tracks per-pattern match frequency for
// the loop guard. Mirrors auto_respond_decide. Agent edit-diff lines are
// stripped first (stripAgentDiffLines) so prompt-shaped text the agent is
// merely WRITING never drives a send/escalate decision.
func decideAutoRespond(pane string, prompts []ManifestPrompt, counts map[string]int, paneBusy bool) (string, int) {
	pane = stripAgentDiffLines(pane)
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
		// ADR-0047 state-gate: a policy=escalate prompt (rate_limit/quota/auth)
		// means the CLI is BLOCKED needing intervention — mutually exclusive with
		// the CLI actively generating. If the pane is BUSY, an escalate match is
		// the agent QUOTING the banner in its own output, not the CLI's chrome —
		// skip it (don't count toward the loop guard) and keep scanning. Cycle-314:
		// a clihealth coverage cycle wrote "You've hit your usage limit" into a
		// test fixture while codex showed "Working… esc to interrupt"; the bridge
		// benched the codex family 30min on the agent's own content. Scoped to
		// escalate only — auto_respond prompts (menus/approvals) legitimately
		// co-occur with an "esc to cancel" affordance and must still fire.
		if paneBusy && p.Policy == "escalate" {
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
	// I3 AskBroker (ADR-0045): when an escalation would fire (rc 85) and the
	// kernel KNOWS the answer to the blocking question, inject it ONCE and buy
	// one more interval instead of failing the whole phase to a cross-family
	// re-dispatch. nil broker / non-enforce stage / a miss all fall through to
	// the unchanged 85 → fallback chain (the unconditional floor). brokerTried
	// bounds it to once per launch.
	broker      *interaction.KernelAnswerer
	brokerStage string
	brokerTried bool
	// shadowRules are the SHADOW-stage promoted rules, matched observe-only
	// per tick (R8.2): a match records a rule_shadow_fire/would_fire outcome
	// — the soak evidence the I4 measured auto-enforce sweep reads — and
	// sends NOTHING. shadowFired dedups to one signal per rule per launch.
	shadowRules []shadowObserver
	shadowFired map[string]bool
}

// pendingAutoRespond is one injection awaiting its outcome: the rule/source
// that fired, its compiled pattern (re-checked against the NEXT capture), the
// payload, the send timestamp, and the I1 Kind/Trigger to record under (an
// auto-respond send vs an I3 kernel answer resolve identically — pattern
// cleared on next capture — so they share this struct).
type pendingAutoRespond struct {
	rule    string
	re      *regexp.Regexp
	keys    string
	at      time.Time
	kind    string // interaction.Kind* (default KindAutoRespond)
	trigger string
}

// newAutoResponder builds the responder from the CLI's embedded manifest.
// A missing/unreadable manifest yields an empty rule set (tick → noop).
// human engages the keystroke-plausibility send path.
func newAutoResponder(cli, workspace string, deps Deps, human bool, scrollback int) *autoResponder {
	var prompts []ManifestPrompt
	if m, err := LoadManifest(cli); err == nil {
		prompts = m.InteractivePrompts
	}
	return &autoResponder{prompts: prompts, workspace: workspace, cli: cli, counts: map[string]int{}, deps: deps, human: human, scrollback: scrollback, suppressLogged: map[string]bool{}, shadowFired: map[string]bool{}}
}

// tick captures the pane, decides, and applies the effect (send-keys or
// escalation-report). Returns (action, rc) for runTmuxREPL's loop.
func (ar *autoResponder) tick(ctx context.Context, session string) (string, int) {
	pane, _ := ar.deps.Tmux.CapturePane(ctx, session, ar.scrollback)
	// ADR-0045 I1: resolve the in-flight send against THIS capture before
	// deciding — the pattern no longer matching is the deterministic
	// "it worked" signal ("prompt-pattern cleared on next capture").
	ar.resolvePending(pane)
	// R8.2 / I4 soak signal: shadow-stage promoted rules observe the pane
	// and record a would-fire ONCE per rule per launch — no keys, no
	// control-flow change. The batch-end sweep flips measured-clean rules
	// to enforce on this evidence.
	if ar.rec != nil {
		for _, so := range ar.shadowRules {
			if !ar.shadowFired[so.id] && so.re.MatchString(pane) {
				ar.shadowFired[so.id] = true
				ar.rec.Record(interaction.Outcome{Event: interaction.Event{
					Kind:    "rule_shadow_fire",
					Phase:   ar.phase,
					Cycle:   ar.cycle,
					Trigger: "shadow_rule_matched",
					RuleID:  so.id,
				}, Result: "would_fire"})
			}
		}
	}
	var prevCounts map[string]int
	if ar.rec != nil {
		prevCounts = make(map[string]int, len(ar.counts))
		for k, v := range ar.counts {
			prevCounts[k] = v
		}
	}
	// pane here stays RAW: resolvePending, shadow matching, and writeEscalation
	// (above/below) need the real terminal. stripAgentDiffLines runs only
	// inside decideAutoRespond, scoped to the prompt-matching decision.
	paneBusy := panestream.PaneBusy(pane, panestream.Profiles[strings.TrimSuffix(ar.cli, "-tmux")])
	action, rc := decideAutoRespond(pane, ar.prompts, ar.counts, paneBusy)
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
		// ADR-0045 I3: the pre-85 rung. If the kernel can answer the blocking
		// question, inject it once and return "responded" (rc 1) to buy one
		// more interval. Any miss / non-enforce / no-typed-question falls
		// through to today's escalation — I3 never suppresses the 85 chain.
		if ar.tryKernelAnswer(ctx, session, pane) {
			return "", 1
		}
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

// record emits one resolved injection outcome through the I1 chokepoint. The
// pending carries its own Kind/Trigger so an auto-respond send and an I3
// kernel answer record under the right vocabulary (both resolve the same way:
// pattern cleared on the next capture).
func (ar *autoResponder) record(p *pendingAutoRespond, result string) {
	kind := p.kind
	if kind == "" {
		kind = interaction.KindAutoRespond
	}
	trigger := p.trigger
	if trigger == "" {
		trigger = "prompt_matched"
	}
	ar.rec.Record(interaction.Outcome{
		Event: interaction.Event{
			Kind:    kind,
			Phase:   ar.phase,
			Cycle:   ar.cycle,
			Trigger: trigger,
			Payload: p.keys,
			RuleID:  p.rule,
		},
		Result:    result,
		LatencyMS: ar.deps.Now().Sub(p.at).Milliseconds(),
	})
}

// tryKernelAnswer is the I3 pre-85 rung: extract the blocking question via the
// panetrust trust boundary (typed extraction — the privileged path never
// branches on raw pane), ask the KernelAnswerer, and on a hit inject the
// answer ONCE. Returns true only when an answer was actually injected (enforce
// stage). Shadow records a would-act soak signal but injects nothing; off,
// a nil broker, a non-extractable pane, or a kernel MISS all return false so
// the caller escalates exactly as today.
func (ar *autoResponder) tryKernelAnswer(ctx context.Context, session, pane string) bool {
	if ar.broker == nil || ar.brokerTried || ar.brokerStage == "off" || ar.brokerStage == "" {
		return false
	}
	q, err := panetrust.Extract(pane, panetrust.ExtractSpec{Kind: panetrust.ExtractQuestion})
	if err != nil {
		return false // no typed question → fall through to the chain, never guess
	}
	answer, ok := ar.broker.Answer(q.Value)
	if !ok {
		return false // kernel doesn't know → the 85 chain is the floor
	}
	if ar.brokerStage != "enforce" {
		// Shadow soak: record what we WOULD have answered, change nothing.
		// brokerTried is NOT consumed here — the once-budget bounds INJECTION
		// (enforce), not soak recording; the escalate rule's own loop guard
		// caps how many ticks reach this path.
		fmt.Fprintf(ar.deps.Stderr, "[ask-broker] shadow: would answer %q with %q (EVOLVE_PHASE_RECOVERY=%s)\n", q.Value, answer, ar.brokerStage)
		if ar.rec != nil {
			ar.rec.Record(interaction.Outcome{
				Event:  interaction.Event{Kind: interaction.KindKernelAnswer, Phase: ar.phase, Cycle: ar.cycle, Trigger: "unknown_prompt", Payload: answer, RuleID: "would_act"},
				Result: interaction.ResultWouldAct,
			})
		}
		return false
	}
	// Enforce: inject the answer once. The agent is blocked AT a prompt
	// (that is why an escalation fired), so it is idle by construction — no
	// Busy guard needed here. brokerTried is consumed on THIS path only.
	ar.brokerTried = true
	if ar.human {
		humanReadingPause(ar.deps, pane)
	}
	_ = ar.deps.Tmux.SendKeys(ctx, session, answer, true)
	fmt.Fprintf(ar.deps.Stderr, "[ask-broker] answered %q with kernel fact %q\n", q.Value, answer)
	// Resolve like an auto-respond send: if the question text clears on the
	// next capture it worked (prompt_cleared); otherwise the run ends with it
	// pending (run_ended) — never a fabricated success. QuoteMeta output always
	// compiles, so a nil re (handled by resolvePending) is unreachable here.
	if ar.rec != nil {
		ar.flushPending() // resolve any prior auto-respond send first
		re, _ := regexp.Compile(regexp.QuoteMeta(strings.TrimSpace(q.Value)))
		ar.pending = &pendingAutoRespond{
			rule:    "kernel_answer",
			re:      re,
			keys:    answer,
			at:      ar.deps.Now(),
			kind:    interaction.KindKernelAnswer,
			trigger: "unknown_prompt",
		}
	}
	return true
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
