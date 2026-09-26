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
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// autoRespondLoopGuardLimit caps the matches of one pattern; the next match abandons the run with rc 86.
const autoRespondLoopGuardLimit = 5

// agentDiffLineRE matches an agent-authored diff line, numbered or bare. CLI chrome is never
// diff-prefixed, so such a line is the agent's content and must not drive prompt matching.
var agentDiffLineRE = regexp.MustCompile(`^[ \t]*(?:\d+[ \t]+)?[+-]`)

func isAgentDiffLine(ln string) bool {
	trimmed := strings.TrimLeft(ln, " \t")
	if strings.HasPrefix(trimmed, "+++") || strings.HasPrefix(trimmed, "---") {
		return false
	}
	return agentDiffLineRE.MatchString(ln)
}

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

// stripPromptEchoLines drops each line whose trimmed text is a substring of the injected prompt.
// An empty prompt strips nothing: fail open rather than suppress a genuine signal.
func stripPromptEchoLines(pane, injectedPrompt string) string {
	if strings.TrimSpace(injectedPrompt) == "" {
		return pane
	}
	lines := strings.Split(pane, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed != "" && strings.Contains(injectedPrompt, trimmed) {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

// strippedForExhaustionScan is the one pane treatment both exhaustion scans (tick and stop review) use.
// Stripping only reduces the surface; the persistence gate is what spares a working agent.
func strippedForExhaustionScan(pane, injectedPrompt string) string {
	return stripAgentDiffLines(stripPromptEchoLines(pane, injectedPrompt))
}

// strippedForFatalPaneScan delegates to recovery.StripAgentContent, which core's advise hook shares.
// It differs from strippedForExhaustionScan on purpose: it blanks lines in place and honors a protect list.
func strippedForFatalPaneScan(pane, injectedPrompt string, protected []string) string {
	return recovery.StripAgentContent(pane, injectedPrompt, protected)
}

// decideAutoRespond returns the action and rc of the first manifest prompt that matches the
// diff-stripped pane; counts feeds the loop guard.
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
		// Live modals sit at the bottom of the capture, so TailLines means "on screen now".
		subject := pane
		if p.TailLines > 0 {
			subject = lastLines(subject, p.TailLines)
		}
		if !re.MatchString(subject) {
			continue
		}
		// A busy CLI cannot be blocked on an escalate prompt, so a match is the agent quoting a banner.
		// Only escalate is gated: menus and approvals legitimately render beside "esc to cancel".
		if paneBusy && p.Policy == "escalate" {
			continue
		}
		// A handled fire-once prompt lingers in scrollback: skip it without counting toward the loop
		// guard, and keep scanning so a new prompt on the same pane still fires.
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
	// A lingering fire-once prompt reports suppress_once (rc 0) so the caller can warn once.
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

// autoResponder holds one tmux launch's prompt rules, match counts and in-flight send.
type autoResponder struct {
	prompts []ManifestPrompt
	// transientRegex is the manifest's temporary-upstream signature, compiled once per launch.
	transientRegex   string
	transientPattern *regexp.Regexp
	// transientGate measures a 60s dwell on the wait loop's 2s cadence. Only the wait loop
	// enables it, because other tick callers discard rc.
	transientGate         *exhaustionGate
	transientDwellEnabled bool
	transientFired        bool
	// exhaustedRegex is the manifest's quota-wall pattern, checked every tick so a wall
	// escalates without waiting for the stop-review checkpoint.
	exhaustedRegex string
	// exhaustGate requires the wall on consecutive ticks before rc 85, so a wall-shaped frame a
	// working agent renders never kills it.
	exhaustGate *exhaustionGate
	// injectedPrompt is stripped from the scans so the agent quoting its own instructions never escalates.
	injectedPrompt string
	// wallScanSuppressed disables the exhaustion scan for the phase once a live probe finds the CLI healthy.
	wallScanSuppressed bool
	// wallProbed and wallConfirmed latch the one corroboration per responder: callers that discard
	// rc would otherwise re-probe a confirmed wall every tick.
	wallProbed    bool
	wallConfirmed bool
	workspace     string
	cli           string
	counts        map[string]int
	deps          Deps
	human         bool // when true, deliver keys with human-input cadence
	// scrollback is the capture depth: 0 for visible-pane CLIs, >0 for alt-screen CLIs whose visible pane is blank.
	scrollback int
	// suppressLogged holds the fire-once prompts already warned about, so each warns once.
	suppressLogged map[string]bool
	// rec records every send with its resolved outcome; nil disables telemetry (recipe capability
	// runs). pending is the one send awaiting resolution.
	// See ADR-0045.
	rec     *interaction.Recorder
	phase   string
	cycle   int
	pending *pendingAutoRespond
	// broker answers a blocking question the kernel knows, once per launch (brokerTried), instead of
	// escalating. A nil broker, a non-enforce stage or a miss falls through to rc 85.
	broker      *interaction.KernelAnswerer
	brokerStage string
	brokerTried bool
	// shadowRules match observe-only and record would_fire once per rule (shadowFired); they send nothing.
	shadowRules []shadowObserver
	shadowFired map[string]bool
	// firedOnceThisTick tells the boot loop that the rc-1 send dismissed a fire-once dialog, whose
	// selection cursor can look like the REPL marker, so boot re-polls.
	firedOnceThisTick bool
}

// firedRuleOnce reports whether the rule that just fired is fire-once. Only a boot dialog renders a
// cursor that can pass for the REPL marker, so only it warrants a boot-loop re-poll.
func (ar *autoResponder) firedRuleOnce(prevCounts map[string]int) bool {
	for _, p := range ar.prompts {
		if ar.counts[p.Name] > prevCounts[p.Name] {
			return p.Once
		}
	}
	return false
}

// firedRuleName names the rule whose count just advanced, so the send log attributes the keystroke.
func (ar *autoResponder) firedRuleName(prevCounts map[string]int) string {
	for _, p := range ar.prompts {
		if ar.counts[p.Name] > prevCounts[p.Name] {
			return p.Name
		}
	}
	return "unknown"
}

// pendingAutoRespond is one injection awaiting its outcome on the next capture. An auto-respond
// send and a kernel answer resolve the same way, so they share it.
type pendingAutoRespond struct {
	rule    string
	re      *regexp.Regexp
	keys    string
	at      time.Time
	kind    string // interaction.Kind* (default KindAutoRespond)
	trigger string
}

// newAutoResponder loads the CLI's manifest rules; a missing manifest yields none (tick is a noop).
func newAutoResponder(cli, workspace string, deps Deps, human bool, scrollback int) *autoResponder {
	var prompts []ManifestPrompt
	var exhaustedRegex string
	var transientRegex string
	var transientPattern *regexp.Regexp
	if m, err := LoadManifest(cli); err == nil {
		prompts = m.InteractivePrompts
		exhaustedRegex = manifestExhaustedPattern(m)
		transientRegex = m.TransientRegex
		if transientRegex != "" {
			transientPattern, err = regexp.Compile(transientRegex)
			if err != nil && deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[%s] WARN: invalid manifest transient_regex: %v\n", cli, err)
			}
		}
	}
	return &autoResponder{prompts: prompts, transientRegex: transientRegex, transientPattern: transientPattern, transientGate: &exhaustionGate{threshold: transientDwellObservations}, exhaustedRegex: exhaustedRegex, exhaustGate: newExhaustionGate(), workspace: workspace, cli: cli, counts: map[string]int{}, deps: deps, human: human, scrollback: scrollback, suppressLogged: map[string]bool{}, shadowFired: map[string]bool{}}
}

// tick captures the pane and applies one observation, returning (action, rc) for runTmuxREPL.
func (ar *autoResponder) tick(ctx context.Context, session string) (string, int) {
	pane, err := ar.deps.Tmux.CapturePane(ctx, session, ar.scrollback)
	return ar.tickPane(ctx, session, pane, err == nil)
}

// transientDwellObservations is the 60s dwell at the wait loop's 2s cadence. Change it together
// with exhaustionPersistObservations, or say why not.
const transientDwellObservations = 30

// tickPane applies one observation to a pane the wait loop already captured. A failed capture
// (captureOK false) is no observation, never a recovered pane, so it cannot reset the dwell.
func (ar *autoResponder) tickPane(ctx context.Context, session, pane string, captureOK bool) (string, int) {
	// Resolve the in-flight send first: its pattern no longer matching is the proof it worked.
	ar.resolvePending(pane)
	// Shadow rules record a would-fire once per rule and change nothing.
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
	// decideAutoRespond advances the fired rule's counter; the snapshot identifies that rule.
	prevCounts := make(map[string]int, len(ar.counts))
	for k, v := range ar.counts {
		prevCounts[k] = v
	}
	ar.firedOnceThisTick = false
	// pane stays raw for resolvePending, shadow rules and writeEscalation; only the decision strips it.
	// BusyOf is nil-safe and stateless, so this read never disturbs the checkpoint's Observe baseline.
	paneBusy := ar.deps.LivenessCenter.BusyOf(pane, panestream.Profiles[strings.TrimSuffix(ar.cli, "-tmux")])
	// scanPane drops prompt echoes so neither scan fires on the agent's own instructions.
	scanPane := stripPromptEchoLines(pane, ar.injectedPrompt)
	action, rc := decideAutoRespond(scanPane, ar.prompts, ar.counts, paneBusy)
	if ar.transientDwellEnabled && ar.transientPattern != nil && captureOK {
		if ar.transientGate == nil {
			// A nil gate from direct struct construction degrades to a disabled dwell, never a panic.
			ar.transientGate = &exhaustionGate{threshold: transientDwellObservations}
		}
		matched := !paneBusy && ar.transientPattern.MatchString(strippedForExhaustionScan(pane, ar.injectedPrompt))
		if !matched {
			ar.transientFired = false
		}
		if ar.transientGate.observe(matched) && !ar.transientFired {
			ar.transientFired = true
			stage := recoveryStageFromEnv(ar.deps)
			if stage != "off" && ar.rec != nil {
				result := "would_fast_fail"
				if stage == "enforce" {
					result = "fast_failed"
				}
				ar.rec.Record(interaction.Outcome{Event: interaction.Event{
					Kind:    "transient_dwell",
					Phase:   ar.phase,
					Cycle:   ar.cycle,
					Trigger: "transient_upstream_60s",
				}, Result: result})
			}
			if stage == "enforce" {
				return "transient_dwell", 3
			}
		}
	}
	// A quota wall escalates regardless of busy and overrides a lesser verdict: the artifact will never
	// come. It scans the stripped pane and is persistence-gated, so wall text a working agent renders
	// never fast-fails it.
	if rc != 85 && ar.exhaustedRegex != "" && !ar.wallScanSuppressed {
		if ar.exhaustGate == nil { // bulletproof against a direct struct construction
			ar.exhaustGate = newExhaustionGate()
		}
		walled := ar.deps.LivenessCenter.ExhaustedOf(strippedForExhaustionScan(pane, ar.injectedPrompt), panestream.PaneProfile{ExhaustedRegex: ar.exhaustedRegex})
		if ar.exhaustGate.observe(walled) {
			// Exactly one live probe per responder, because work content can hold wall vocabulary
			// on-pane. Both answers latch: callers that discard rc would re-probe every tick.
			if !ar.wallProbed {
				ar.wallProbed = true
				ar.wallConfirmed = wallCorroborated(ctx, ar.deps.CorroborateWall, ar.cli)
				if !ar.wallConfirmed {
					ar.wallScanSuppressed = true
					fmt.Fprintf(ar.deps.Stderr, "[%s] EXHAUSTION-SUPPRESSED: pane matched wall vocabulary but a live probe (cheapest tier) answered — treating as content-induced; a tier-scoped wall would surface via artifact-timeout fallback instead\n", ar.cli)
				}
			}
			if ar.wallConfirmed {
				action, rc = "escalate:exhausted", 85
			}
		}
	}
	switch rc {
	case 1:
		ar.firedOnceThisTick = ar.firedRuleOnce(prevCounts)
		keysCSV := strings.TrimPrefix(action, "send:")
		if ar.human {
			humanReadingPause(ar.deps, pane)
			humanSendKeysCSV(ctx, ar.deps, session, keysCSV)
		} else {
			sendKeySequence(ctx, ar.deps, session, keysCSV)
			fmt.Fprintf(ar.deps.Stderr, "[auto-respond] sent keys: %s (rule=%s)\n", keysCSV, ar.firedRuleName(prevCounts))
		}
		ar.openPending(prevCounts, keysCSV)
		return "", 1
	case 2:
		fmt.Fprintf(ar.deps.Stderr, "[auto-respond] extend_timeout signal: %s\n", action)
		return action, 2
	case 85:
		// Before escalating, let the kernel answer a blocking question it knows; a miss still escalates.
		if ar.tryKernelAnswer(ctx, session, pane) {
			return "", 1
		}
		ar.writeEscalation(pane, strings.TrimPrefix(action, "escalate:"), "escalate", session)
		return "", 85
	case 86:
		// The same pattern kept matching, so the pending send demonstrably did not clear it.
		name := strings.TrimPrefix(action, "loop_guard:")
		if ar.pending != nil && ar.pending.rule == name {
			ar.record(ar.pending, interaction.ResultNoEffect)
			ar.pending = nil
		}
		ar.writeEscalation(pane, name, "loop_guard", session)
		return "", 86
	default:
		// A fire-once prompt still matches after its response. That is indistinguishable from an
		// unanswered dialog here, so record suppressed_lingering and warn once.
		if name, ok := strings.CutPrefix(action, "suppress_once:"); ok {
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

// resolvePending records prompt_cleared once the pending pattern stops matching. A nil pattern is
// left for flushPending, so no outcome is fabricated.
func (ar *autoResponder) resolvePending(pane string) {
	if ar.pending == nil || ar.pending.re == nil {
		return
	}
	if !ar.pending.re.MatchString(pane) {
		ar.record(ar.pending, interaction.ResultPromptCleared)
		ar.pending = nil
	}
}

// openPending closes a still-pending predecessor as no_effect and opens the outcome window for the
// send that just fired. No-op without a recorder.
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

// flushPending records run_ended for a send the run ends on, so no outcome is dropped.
func (ar *autoResponder) flushPending() {
	if ar.pending == nil {
		return
	}
	ar.record(ar.pending, interaction.ResultRunEnded)
	ar.pending = nil
}

// record emits one resolved injection outcome under the pending send's own kind and trigger.
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

// tryKernelAnswer extracts the blocking question through the panetrust boundary (never raw pane) and
// injects the kernel's answer once in enforce stage. Every other path returns false, so the caller escalates.
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
		// Shadow records what it would answer. brokerTried bounds injection, not recording; the
		// escalate rule's loop guard caps how often this path runs.
		fmt.Fprintf(ar.deps.Stderr, "[ask-broker] shadow: would answer %q with %q (EVOLVE_PHASE_RECOVERY=%s)\n", q.Value, answer, ar.brokerStage)
		if ar.rec != nil {
			ar.rec.Record(interaction.Outcome{
				Event:  interaction.Event{Kind: interaction.KindKernelAnswer, Phase: ar.phase, Cycle: ar.cycle, Trigger: "unknown_prompt", Payload: answer, RuleID: "would_act"},
				Result: interaction.ResultWouldAct,
			})
		}
		return false
	}
	// The agent is blocked at a prompt, so it is idle by construction and needs no busy guard.
	ar.brokerTried = true
	if ar.human {
		humanReadingPause(ar.deps, pane)
	}
	_ = ar.deps.Tmux.SendKeys(ctx, session, answer, true)
	fmt.Fprintf(ar.deps.Stderr, "[ask-broker] answered %q with kernel fact %q\n", q.Value, answer)
	// Resolve like an auto-respond send: prompt_cleared if the question clears, else run_ended.
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

// autoRespondInterKeyPause gives the TUI a render frame between keys: claude's multi-select fails
// to submit on a burst. It runs through Deps.Sleep, so tests stay fast.
const autoRespondInterKeyPause = 500 * time.Millisecond

// sendKeySequence sends each comma-separated token as its own paced keystroke; an "Enter" token is
// a bare Enter. Collapsing the Enters would submit a multi-select with nothing selected.
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

// writeEscalation writes escalation-report.json from the final pane, the operator's repair trail.
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

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
