package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

const autoRespondLoopGuardLimit = 5

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

func strippedForExhaustionScan(pane, injectedPrompt string) string {
	return stripAgentDiffLines(stripPromptEchoLines(pane, injectedPrompt))
}

func strippedForFatalPaneScan(pane, injectedPrompt string, protected []string) string {
	return recovery.StripAgentContent(pane, injectedPrompt, protected)
}

func decideAutoRespond(pane string, prompts []ManifestPrompt, counts map[string]int, paneBusy bool) (string, int) {
	pane = stripAgentDiffLines(pane)
	suppressedOnce := ""
	for _, p := range prompts {
		if !promptMatches(p, pane) {
			continue
		}
		if paneBusy && (p.Policy == "escalate" || p.Policy == "hold") {
			continue
		}
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
		case "hold":
			return "hold:" + p.Name, 4
		default:
			return "escalate:" + p.Name, 85
		}
	}
	if suppressedOnce != "" {
		return "suppress_once:" + suppressedOnce, 0
	}
	return "noop", 0
}

func promptMatches(p ManifestPrompt, pane string) bool {
	if p.Regex == "" {
		return false
	}
	re, err := regexp.Compile(p.Regex)
	if err != nil {
		return false
	}
	subject := pane
	if p.TailLines > 0 {
		subject = lastLines(subject, p.TailLines)
	}
	return re.MatchString(subject)
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

type autoResponder struct {
	prompts               []ManifestPrompt
	transientRegex        string
	transientPattern      *regexp.Regexp
	transientGate         *exhaustionGate
	transientDwellEnabled bool
	transientFired        bool
	exhaustedRegex        string
	exhaustGate           *exhaustionGate
	injectedPrompt        string
	wallScanSuppressed    bool
	wallProbed            bool
	wallConfirmed         bool
	workspace             string
	cli                   string
	counts                map[string]int
	deps                  Deps
	human                 bool
	scrollback            int
	suppressLogged        map[string]bool
	rec                   *interaction.Recorder
	phase                 string
	cycle                 int
	pending               *pendingAutoRespond
	broker                *interaction.KernelAnswerer
	brokerStage           string
	brokerTried           bool
	shadowRules           []shadowObserver
	shadowFired           map[string]bool
	paneProfile           panestream.PaneProfile
}

func (ar *autoResponder) firedRuleName(prevCounts map[string]int) string {
	for _, p := range ar.prompts {
		if ar.counts[p.Name] > prevCounts[p.Name] {
			return p.Name
		}
	}
	return "unknown"
}

type pendingAutoRespond struct {
	rule    string
	re      *regexp.Regexp
	keys    string
	at      time.Time
	kind    string
	trigger string
}

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
	return &autoResponder{paneProfile: cliPaneProfile(cli), prompts: prompts, transientRegex: transientRegex, transientPattern: transientPattern, transientGate: &exhaustionGate{threshold: transientDwellObservations}, exhaustedRegex: exhaustedRegex, exhaustGate: newExhaustionGate(), workspace: workspace, cli: cli, counts: map[string]int{}, deps: deps, human: human, scrollback: scrollback, suppressLogged: map[string]bool{}, shadowFired: map[string]bool{}}
}

func (ar *autoResponder) tick(ctx context.Context, session string) (string, int) {
	pane, err := ar.deps.Tmux.CapturePane(ctx, session, ar.scrollback)
	return ar.tickPane(ctx, session, pane, err == nil)
}

func (ar *autoResponder) bootTick(ctx context.Context, session, capturedPane string) (repoll bool, err error) {
	action, rc := ar.tickPane(ctx, session, capturedPane, true)
	switch rc {
	case 1, 4:
		return true, nil
	case 86:
		rule := strings.TrimPrefix(action, "loop_guard:")
		return false, fmt.Errorf("auto-respond loop guard: rule %s matched more than %d times", rule, autoRespondLoopGuardLimit)
	}
	return false, nil
}

func (ar *autoResponder) endBoot() {
	if ar == nil {
		return
	}
	ar.prompts = slices.DeleteFunc(slices.Clone(ar.prompts), func(p ManifestPrompt) bool { return p.Policy == "hold" })
}

const transientDwellObservations = 30

func (ar *autoResponder) tickPane(ctx context.Context, session, pane string, captureOK bool) (string, int) {
	ar.resolvePending(pane)
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
	prevCounts := make(map[string]int, len(ar.counts))
	for k, v := range ar.counts {
		prevCounts[k] = v
	}
	paneBusy := ar.deps.LivenessCenter.BusyOf(pane, ar.paneProfile)
	scanPane := stripPromptEchoLines(pane, ar.injectedPrompt)
	action, rc := decideAutoRespond(scanPane, ar.prompts, ar.counts, paneBusy)
	if ar.transientDwellEnabled && ar.transientPattern != nil && captureOK {
		if ar.transientGate == nil {
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
	if rc != 85 && ar.exhaustedRegex != "" && !ar.wallScanSuppressed {
		if ar.exhaustGate == nil {
			ar.exhaustGate = newExhaustionGate()
		}
		walled := ar.deps.LivenessCenter.ExhaustedOf(strippedForExhaustionScan(pane, ar.injectedPrompt), panestream.PaneProfile{ExhaustedRegex: ar.exhaustedRegex})
		if ar.exhaustGate.observe(walled) {
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
		if ar.tryKernelAnswer(ctx, session, pane) {
			return "", 1
		}
		ar.writeEscalation(pane, strings.TrimPrefix(action, "escalate:"), "escalate", session)
		return "", 85
	case 86:
		name := strings.TrimPrefix(action, "loop_guard:")
		if ar.pending != nil && ar.pending.rule == name {
			ar.record(ar.pending, interaction.ResultNoEffect)
			ar.pending = nil
		}
		ar.writeEscalation(pane, name, "loop_guard", session)
		return action, 86
	default:
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
		return "", rc
	}
}

func (ar *autoResponder) resolvePending(pane string) {
	if ar.pending == nil || ar.pending.re == nil {
		return
	}
	if !ar.pending.re.MatchString(pane) {
		ar.record(ar.pending, interaction.ResultPromptCleared)
		ar.pending = nil
	}
}

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
		return
	}
	var re *regexp.Regexp
	for _, p := range ar.prompts {
		if p.Name == name {
			re, _ = regexp.Compile(p.Regex)
			break
		}
	}
	ar.pending = &pendingAutoRespond{rule: name, re: re, keys: keysCSV, at: ar.deps.Now()}
}

func (ar *autoResponder) flushPending() {
	if ar.pending == nil {
		return
	}
	ar.record(ar.pending, interaction.ResultRunEnded)
	ar.pending = nil
}

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

func (ar *autoResponder) tryKernelAnswer(ctx context.Context, session, pane string) bool {
	if ar.broker == nil || ar.brokerTried || ar.brokerStage == "off" || ar.brokerStage == "" {
		return false
	}
	q, err := panetrust.Extract(pane, panetrust.ExtractSpec{Kind: panetrust.ExtractQuestion})
	if err != nil {
		return false
	}
	answer, ok := ar.broker.Answer(q.Value)
	if !ok {
		return false
	}
	if ar.brokerStage != "enforce" {
		fmt.Fprintf(ar.deps.Stderr, "[ask-broker] shadow: would answer %q with %q (EVOLVE_PHASE_RECOVERY=%s)\n", q.Value, answer, ar.brokerStage)
		if ar.rec != nil {
			ar.rec.Record(interaction.Outcome{
				Event:  interaction.Event{Kind: interaction.KindKernelAnswer, Phase: ar.phase, Cycle: ar.cycle, Trigger: "unknown_prompt", Payload: answer, RuleID: "would_act"},
				Result: interaction.ResultWouldAct,
			})
		}
		return false
	}
	ar.brokerTried = true
	if ar.human {
		humanReadingPause(ar.deps, pane)
	}
	_ = ar.deps.Tmux.SendKeys(ctx, session, answer, true)
	fmt.Fprintf(ar.deps.Stderr, "[ask-broker] answered %q with kernel fact %q\n", q.Value, answer)
	if ar.rec != nil {
		ar.flushPending()
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

const autoRespondInterKeyPause = 500 * time.Millisecond

func sendKeySequence(ctx context.Context, deps Deps, session, csv string) {
	first := true
	for _, tok := range strings.Split(csv, ",") {
		if tok == "" {
			continue
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

func newLaunchAutoResponder(workspace string, deps Deps, lp tmuxLaunch, human bool) *autoResponder {
	ar := newAutoResponder(lp.name, workspace, deps, human, lp.bootScrollback)
	ar.paneProfile = paneProfileFor(lp)
	return ar
}

func cliPaneProfile(cli string) panestream.PaneProfile {
	var lp tmuxLaunch
	lp.name = cli
	return paneProfileFor(lp)
}
