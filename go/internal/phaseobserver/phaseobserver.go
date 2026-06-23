// Package phaseobserver ports the core behavior of
// legacy/scripts/dispatch/phase-observer.sh (v10.18.0+ default stall detector).
//
// SCOPE NOTE: this v11.6.x port covers the primary use case — stall detection
// + NDJSON event tracking + report writing on shutdown. The 4 secondary
// detection rules (infinite_loop, error_spike, cost_anomaly, rate_limit) emit
// best-effort events but do not yet fire as INCIDENT severity. Operators
// The full rule set remains deferred until the rule-engine port lands.
//
// Tails the subagent's stream-json stdout.log, maintains in-memory state,
// runs deterministic detection rules periodically, and emits observations to:
//   - {agent}-observer-events.ndjson  (live, append-only, one envelope/line)
//   - {agent}-observer-report.json    (atomic write at shutdown)
package phaseobserver

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolveloop/go/internal/recovery"
)

const (
	ExitOK          = 0
	ExitInvalidArgs = 10
)

// Scope is "phase" (default) or "cycle". Cycle-scope runs only stall_no_output.
type Scope string

const (
	ScopePhase Scope = "phase"
	ScopeCycle Scope = "cycle"
)

// Config wires the observer.
type Config struct {
	Workspace    string
	SubagentPGID int
	Cycle        int
	Phase        string
	Agent        string
	CycleState   string
	Scope        Scope
	Enforce      bool

	PollS          int
	StallS         int
	NudgeS         int    // soft-stall nudge threshold; 0 = disabled (opt-in)
	NudgeBody      string // text appended to the agent inbox on soft stall
	LoopN          int
	LoopWindowS    int
	ErrorRate      float64
	CostSigma      float64
	ThrottleN      int
	EOFGraceS      int
	HeartbeatEvery int

	// MaxNoProgressS is Workstream E1's "babbling agent" backstop. The existing
	// StallS check resets on ANY valid JSON line — including pure assistant_text
	// token streaming — so a livelocked agent that emits forever never trips it.
	// MaxNoProgressS tracks "last MEANINGFUL progress" (tool_use / tool_result /
	// progress events only) and emits stuck_no_progress when exceeded. 0 = the
	// feature is disabled (legacy posture, byte-identical to pre-E1).
	MaxNoProgressS int

	// StallPolicy (ADR-0044 C4) maps a stall INCIDENT to a recovery action
	// (extend | kill_retry | escalate), decoupling action from detection.
	// nil — the default until the C3 composition slice wires a real policy —
	// preserves the legacy inline behavior byte-for-byte: Enforce → SIGTERM,
	// unenriched INCIDENT envelope. With a policy injected, its verdict
	// outranks Enforce (extend/escalate suppress the kill; kill_retry kills
	// even without Enforce) and the decision + justification are recorded
	// inside the INCIDENT envelope (action / action_reason).
	StallPolicy recovery.StallPolicy

	// ProcessAlive probes whether the observed agent's process group still
	// exists (R3.4, cycles 274/277: pane/log echo is NOT liveness — a wedged
	// shell read as alive for 25+ min after the CLI died). A dead group
	// fires a "process_dead" INCIDENT once, resolved by the stall policy
	// (chain: kill_retry) regardless of idle budgets. nil DISABLES the probe
	// (legacy byte-identical — the same nil-seam convention as StallPolicy);
	// the composition root wires DefaultProcessAlive, so fixture Configs
	// with fake pgids never see a false "dead".
	ProcessAlive func(pgid int) bool

	// Testing seams.
	Now         func() time.Time
	KillPgrp    func(pgid int, sig syscall.Signal) error
	ShutdownSig <-chan struct{} // SIGUSR1-equivalent
	StopAfterMS int             // testing: force shutdown after this many ms
}

// DefaultProcessAlive is the production R3.4 liveness probe: signal-0 to the
// process group — ESRCH (an error) means the group is gone. EPERM (also an
// error) is treated as DEAD here, which is conservative-but-safe only
// because observer and agent share a UID; if cross-UID subagents are ever
// introduced this probe must distinguish ESRCH from EPERM. Wired by the
// composition root (cmd_phase_observer); Run treats nil as probe-off so
// fixture Configs with fake pgids never see a false "dead".
func DefaultProcessAlive(pgid int) bool {
	return syscall.Kill(-pgid, 0) == nil
}

// Observer holds runtime state.
type Observer struct {
	cfg Config

	traceID      string
	startedAt    time.Time
	startedAtISO string
	lastEventTS  time.Time
	// lastProgressTS is updated ONLY on meaningful-progress events (tool_use,
	// tool_result). Pure assistant_text streaming does NOT reset it. WS-E1.
	lastProgressTS time.Time
	lastByteOff    int64
	eventCount     int
	toolCallCount  int
	errorCount     int
	toolResultCnt  int
	rateLimitCnt   int
	cumulativeCost float64
	cacheReadTok   int
	cacheCreateTok int
	nudged         bool
	// processDeadFired guards the R3.4 dead-process INCIDENT to exactly one
	// emission — the group cannot come back; re-firing per tick is spam.
	processDeadFired bool
	incidents        []map[string]any
	loopHistory      []loopEntry
	rateLimitHist    []time.Time

	mu sync.Mutex
}

type loopEntry struct {
	ts       time.Time
	inputSHA string
	tool     string
}

// Run drives the observer until shutdown. Returns the bash-compatible exit code.
func Run(cfg Config, stdoutPath string, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[phase-observer] "+format+"\n", args...)
	}

	// validation
	if cfg.Workspace == "" || cfg.Phase == "" || cfg.Agent == "" {
		logf("usage: phase-observer <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
		return ExitInvalidArgs
	}
	if info, err := os.Stat(cfg.Workspace); err != nil || !info.IsDir() {
		logf("workspace not a directory: %s", cfg.Workspace)
		return ExitInvalidArgs
	}
	if cfg.Cycle <= 0 {
		logf("cycle must be integer")
		return ExitInvalidArgs
	}

	// defaults
	if cfg.PollS == 0 {
		cfg.PollS = 5
	}
	if cfg.StallS == 0 {
		cfg.StallS = 600
	}
	// NudgeS is opt-in: 0 leaves the soft-stall nudge disabled. NudgeBody
	// gets a default only when nudging is enabled.
	if cfg.NudgeS > 0 && cfg.NudgeBody == "" {
		cfg.NudgeBody = "You appear stalled. Summarize your current state, then either continue or finalize your artifact."
	}
	if cfg.LoopN == 0 {
		cfg.LoopN = 6
	}
	if cfg.LoopWindowS == 0 {
		cfg.LoopWindowS = 120
	}
	if cfg.ErrorRate == 0 {
		cfg.ErrorRate = 0.3
	}
	if cfg.CostSigma == 0 {
		cfg.CostSigma = 2
	}
	if cfg.ThrottleN == 0 {
		cfg.ThrottleN = 3
	}
	if cfg.EOFGraceS == 0 {
		cfg.EOFGraceS = 10
	}
	if cfg.HeartbeatEvery == 0 {
		cfg.HeartbeatEvery = 12
	}
	if cfg.Scope == "" {
		cfg.Scope = ScopePhase
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.KillPgrp == nil {
		cfg.KillPgrp = func(pgid int, sig syscall.Signal) error {
			return syscall.Kill(-pgid, sig)
		}
	}
	// cfg.ProcessAlive deliberately has NO default here: nil = probe off
	// (legacy byte-identical). DefaultProcessAlive is wired at the
	// composition root (cmd_phase_observer), never inside Run.

	now := cfg.Now()
	obs := &Observer{
		cfg:            cfg,
		traceID:        fmt.Sprintf("cycle-%d-%s-%d", cfg.Cycle, cfg.Phase, now.Unix()),
		startedAt:      now,
		startedAtISO:   now.UTC().Format("2006-01-02T15:04:05Z"),
		lastEventTS:    now,
		lastProgressTS: now, // WS-E1: starts at now; only meaningful-progress events bump it
	}

	eventsPath := filepath.Join(cfg.Workspace, cfg.Agent+"-observer-events.ndjson")
	reportPath := filepath.Join(cfg.Workspace, cfg.Agent+"-observer-report.json")
	if stdoutPath == "" {
		stdoutPath = filepath.Join(cfg.Workspace, cfg.Agent+"-stdout.log")
	}

	// Emit start event.
	obs.emit(eventsPath, "observer_started", "INFO", map[string]any{
		"scope":   string(cfg.Scope),
		"enforce": cfg.Enforce,
		"stall_s": cfg.StallS,
		"poll_s":  cfg.PollS,
	})

	// handleStallIncident (ADR-0044 C4) is the single emit+act path for both
	// stall INCIDENT sites. nil StallPolicy ⇒ byte-identical legacy behavior:
	// unenriched envelope + the Enforce→SIGTERM branch. With a policy, its
	// verdict outranks Enforce and is recorded inside the envelope — every
	// recovery decision is justified, never silent.
	type stallIncident struct {
		kind    string
		payload map[string]any
		event   recovery.StallEvent
	}
	handleStallIncident := func(in stallIncident) {
		if cfg.StallPolicy == nil {
			obs.emit(eventsPath, in.kind, "INCIDENT", in.payload)
			if cfg.Enforce && cfg.SubagentPGID > 0 {
				logf("ENFORCE: killing pgid %d due to %s", cfg.SubagentPGID, in.kind)
				_ = cfg.KillPgrp(cfg.SubagentPGID, syscall.SIGTERM)
			}
			return
		}
		action, reason := cfg.StallPolicy.Decide(in.event)
		// The envelope records what will ACTUALLY happen, never the intent
		// alone: a kill_retry with no pgid to kill must not claim a kill
		// (the record-reflects-reality invariant this whole ADR exists for).
		willKill := action == recovery.StallKillRetry && cfg.SubagentPGID > 0
		effective := string(action)
		if action == recovery.StallKillRetry && !willKill {
			effective = "kill_retry_skipped_no_pgid"
		}
		in.payload["action"] = effective
		in.payload["action_reason"] = reason
		obs.emit(eventsPath, in.kind, "INCIDENT", in.payload)
		if willKill {
			logf("stall-policy: killing pgid %d due to %s (%s)", cfg.SubagentPGID, in.kind, reason)
			_ = cfg.KillPgrp(cfg.SubagentPGID, syscall.SIGTERM)
		}
	}

	// Poll loop.
	tickerInterval := time.Duration(cfg.PollS) * time.Second
	if cfg.StopAfterMS > 0 {
		tickerInterval = time.Duration(cfg.StopAfterMS) * time.Millisecond / 4
	}
	pollTicker := time.NewTicker(tickerInterval)
	defer pollTicker.Stop()

	var stopTimer *time.Timer
	if cfg.StopAfterMS > 0 {
		stopTimer = time.NewTimer(time.Duration(cfg.StopAfterMS) * time.Millisecond)
	}

	eofQuietCount := 0
	pollCounter := 0

OUTER:
	for {
		select {
		case <-cfg.ShutdownSig:
			obs.emit(eventsPath, "observer_shutdown", "INFO", map[string]any{"reason": "sigusr1"})
			break OUTER
		case <-func() <-chan time.Time {
			if stopTimer != nil {
				return stopTimer.C
			}
			ch := make(chan time.Time)
			return ch // never fires
		}():
			obs.emit(eventsPath, "observer_shutdown", "INFO", map[string]any{"reason": "stop-timer"})
			break OUTER
		case <-pollTicker.C:
			pollCounter++
			// R3.4 process-liveness probe — BEFORE the log tail, because a
			// dead agent's residual log lines must not refresh the liveness
			// clocks one last time. Fires the INCIDENT exactly once; the
			// stall policy (chain: process-dead-kill, above busy-extend)
			// resolves it to kill_retry regardless of idle budgets — pane/
			// log echo is not liveness (cycles 274/277). nil probe = off.
			if cfg.ProcessAlive != nil && !obs.processDeadFired && cfg.SubagentPGID > 0 && !cfg.ProcessAlive(cfg.SubagentPGID) {
				obs.processDeadFired = true
				handleStallIncident(stallIncident{
					kind:    "process_dead",
					payload: map[string]any{"pgid": cfg.SubagentPGID},
					event:   recovery.StallEvent{Kind: "process_dead", Phase: cfg.Phase},
				})
			}
			newLines, newOffset := obs.tail(stdoutPath)
			if len(newLines) == 0 {
				eofQuietCount++
			} else {
				eofQuietCount = 0
				obs.lastByteOff = newOffset
				for _, ln := range newLines {
					obs.processLine(ln)
				}
			}

			// run stall rule
			if obs.cfg.Scope == ScopeCycle || obs.cfg.Scope == ScopePhase {
				stall := cfg.Now().Sub(obs.lastEventTS).Seconds()
				// Soft-stall nudge (once) before the hard SIGTERM: append a
				// nudge envelope a draining *-tmux driver can inject to prompt
				// the agent to continue or finalize. Opt-in (NudgeS > 0).
				if cfg.NudgeS > 0 && int(stall) >= cfg.NudgeS && !obs.nudged {
					obs.nudged = true
					if err := inbox.Append(cfg.Workspace, cfg.Agent, inbox.Envelope{
						Kind:   inbox.KindNudge,
						Body:   cfg.NudgeBody,
						Source: "observer",
					}, cfg.Now); err != nil {
						logf("WARN soft-stall nudge append failed: %v", err)
					}
					obs.emit(eventsPath, "soft_stall_nudge", "WARN", map[string]any{
						"idle_s":      int(stall),
						"threshold_s": cfg.NudgeS,
					})
				}
				if int(stall) >= cfg.StallS {
					handleStallIncident(stallIncident{
						kind:    "stuck_no_output",
						payload: map[string]any{"idle_s": int(stall), "threshold_s": cfg.StallS},
						event: recovery.StallEvent{
							Kind: "stuck_no_output", Phase: cfg.Phase,
							IdleS: int(stall), ThresholdS: cfg.StallS,
						},
					})
				}
				// WS-E1: babbling-but-livelocked backstop. The idle clock above
				// resets on EVERY valid JSON line including pure assistant_text,
				// so an agent that produces tokens forever without doing real
				// work (no tool_use / tool_result) never trips it. The progress
				// clock only resets on meaningful events, so we catch that case.
				// Opt-in via cfg.MaxNoProgressS > 0; 0 preserves legacy posture.
				if cfg.MaxNoProgressS > 0 {
					noProgress := cfg.Now().Sub(obs.lastProgressTS).Seconds()
					if int(noProgress) >= cfg.MaxNoProgressS {
						handleStallIncident(stallIncident{
							kind: "stuck_no_progress",
							payload: map[string]any{
								"no_progress_s": int(noProgress),
								"threshold_s":   cfg.MaxNoProgressS,
								"tool_calls":    obs.toolCallCount,
								"tool_results":  obs.toolResultCnt,
							},
							event: recovery.StallEvent{
								Kind: "stuck_no_progress", Phase: cfg.Phase,
								IdleS: int(noProgress), ThresholdS: cfg.MaxNoProgressS,
								ToolCalls: obs.toolCallCount, ToolResults: obs.toolResultCnt,
							},
						})
					}
				}
			}

			// heartbeat
			if pollCounter%cfg.HeartbeatEvery == 0 {
				obs.emit(eventsPath, "heartbeat", "INFO", map[string]any{
					"event_count":     obs.eventCount,
					"tool_call_count": obs.toolCallCount,
					"error_count":     obs.errorCount,
					"cumulative_cost": obs.cumulativeCost,
				})
			}

			// EOF detection (stdout stopped growing for grace period)
			if eofQuietCount*cfg.PollS >= cfg.EOFGraceS && obs.eventCount > 0 {
				obs.emit(eventsPath, "observer_shutdown", "INFO", map[string]any{"reason": "eof_grace"})
				break OUTER
			}
		}
	}

	// write final report
	if err := obs.writeReport(reportPath); err != nil {
		logf("WARN: failed to write report: %v", err)
	}
	return ExitOK
}

// tail reads new bytes since last offset. Returns parsed lines + new offset.
func (o *Observer) tail(stdoutPath string) ([]string, int64) {
	info, err := os.Stat(stdoutPath)
	if err != nil {
		return nil, o.lastByteOff
	}
	if info.Size() < o.lastByteOff {
		// rotation; restart from 0
		o.lastByteOff = 0
	}
	f, err := os.Open(stdoutPath)
	if err != nil {
		return nil, o.lastByteOff
	}
	defer func() { _ = f.Close() }() // read-only handle; close error carries no signal
	if _, err := f.Seek(o.lastByteOff, 0); err != nil {
		return nil, o.lastByteOff
	}
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, info.Size()
}

// processLine parses a single stream-json event and updates state.
func (o *Observer) processLine(line string) {
	if line == "" {
		return
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		return // malformed JSON — skip
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.eventCount++
	o.lastEventTS = o.cfg.Now()

	t, _ := doc["type"].(string)
	switch t {
	case "assistant":
		msg, _ := doc["message"].(map[string]any)
		content, _ := msg["content"].([]any)
		if len(content) == 0 {
			return
		}
		block, _ := content[0].(map[string]any)
		btype, _ := block["type"].(string)
		if btype == "tool_use" {
			name, _ := block["name"].(string)
			if name == "" {
				name = "?"
			}
			o.toolCallCount++
			o.lastProgressTS = o.lastEventTS // WS-E1: tool dispatch is meaningful progress
			input, _ := block["input"].(map[string]any)
			inputJSON, _ := json.Marshal(input)
			sum := sha256.Sum256(inputJSON)
			o.loopHistory = append(o.loopHistory, loopEntry{
				ts: o.lastEventTS, inputSHA: hex.EncodeToString(sum[:]), tool: name,
			})
		}
	case "user":
		msg, _ := doc["message"].(map[string]any)
		content, _ := msg["content"].([]any)
		if len(content) == 0 {
			return
		}
		block, _ := content[0].(map[string]any)
		rtype, _ := block["type"].(string)
		if rtype == "tool_result" {
			isErr, _ := block["is_error"].(bool)
			o.toolResultCnt++
			o.lastProgressTS = o.lastEventTS // WS-E1: tool return is meaningful progress
			if isErr {
				o.errorCount++
			}
		}
	case "result":
		if cost, ok := doc["total_cost_usd"].(float64); ok {
			o.cumulativeCost += cost
		}
		if usage, ok := doc["usage"].(map[string]any); ok {
			if cr, ok := usage["cache_read_input_tokens"].(float64); ok {
				o.cacheReadTok += int(cr)
			}
			if cc, ok := usage["cache_creation_input_tokens"].(float64); ok {
				o.cacheCreateTok += int(cc)
			}
		}
	case "rate_limit_event":
		o.rateLimitCnt++
		o.rateLimitHist = append(o.rateLimitHist, o.lastEventTS)
	}
}

// emit writes one event envelope to the events.ndjson file.
func (o *Observer) emit(eventsPath, eventType, severity string, data map[string]any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.cfg.Now()
	envelope := map[string]any{
		"id":             fmt.Sprintf("obs_%d_%d_%d", now.UnixNano(), os.Getpid(), o.eventCount),
		"schema_version": "1.0",
		"ts":             now.UTC().Format("2006-01-02T15:04:05Z"),
		"trace_id":       o.traceID,
		"source": map[string]any{
			"component":    "phase-observer",
			"cycle":        o.cfg.Cycle,
			"phase":        o.cfg.Phase,
			"agent":        o.cfg.Agent,
			"observer_pid": os.Getpid(),
		},
		"type":     eventType,
		"severity": severity,
		"data":     data,
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }() // best-effort event append; write below is already discarded
	_, _ = f.Write(append(b, '\n'))
	if severity == "INCIDENT" {
		o.incidents = append(o.incidents, envelope)
	}
}

// writeReport persists the phase summary atomically.
func (o *Observer) writeReport(reportPath string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.cfg.Now()
	report := map[string]any{
		"schema_version":        "1.0",
		"trace_id":              o.traceID,
		"started_at":            o.startedAtISO,
		"finished_at":           now.UTC().Format("2006-01-02T15:04:05Z"),
		"duration_s":            int(now.Sub(o.startedAt).Seconds()),
		"cycle":                 o.cfg.Cycle,
		"phase":                 o.cfg.Phase,
		"agent":                 o.cfg.Agent,
		"event_count":           o.eventCount,
		"tool_call_count":       o.toolCallCount,
		"tool_result_count":     o.toolResultCnt,
		"error_count":           o.errorCount,
		"rate_limit_count":      o.rateLimitCnt,
		"cumulative_cost":       o.cumulativeCost,
		"cache_read_tokens":     o.cacheReadTok,
		"cache_creation_tokens": o.cacheCreateTok,
		"incident_count":        len(o.incidents),
		"incidents":             o.incidents,
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return err
	}
	tmp := reportPath + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, reportPath)
}
