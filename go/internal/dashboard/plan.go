package dashboard

// plan.go — the per-cycle PhasePlan projection: the registry's mandatory set
// (config.mandatory_phases, the set the router's floor enforces), the phases
// that ran with their last verdict, rounds and contract-gate mark, the
// ongoing phase, and what remains. Pure reader over artifacts the pipeline
// already writes; every source it cannot read is said in the warnings.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	planFile   = "phase-plan.json"
	replanFile = "phase-replan.json"
	// Step statuses beyond the verdict classes (pass/warn/fail).
	stepOngoing   = "ongoing"   // the running cycle's current phase
	stepPending   = "pending"   // a mandatory phase the running cycle has not reached
	stepUnreached = "unreached" // a mandatory phase a sealed cycle never ran
	stepSkipped   = "skipped"   // a mandatory phase the cycle went past without running
)

// mandatorySet is the registry's answer to "which phases are required":
// config.mandatory_phases in order, the conditional-mandatory phases (required
// when their rule holds — the reader cannot evaluate the rule, so one that
// ran counts as required), and the spine order the walk follows.
type mandatorySet struct {
	mandatory   []string
	conditional map[string]bool
	order       []string
}

// readMandatory resolves the set through config, its owner, with the
// operator's environment injected so the same EVOLVE_* overrides the loop's
// floor honours shape the set the board prints: a missing registry is the
// compiled baseline (silent, as everywhere else); an unreadable or malformed
// one, or a weak spine, is said (config.IsRegistryFault).
func readMandatory(root string, env map[string]string) (mandatorySet, []string) {
	cfg, ws := config.Load(config.RegistryPath(root), env)
	var warnings []string
	for _, w := range ws {
		if config.IsRegistryFault(w) {
			warnings = append(warnings, w.Message)
		}
	}
	set := mandatorySet{mandatory: cfg.Mandatory, conditional: map[string]bool{}, order: cfg.SpineOrder}
	for phase := range cfg.Conditional {
		set.conditional[phase] = true
	}
	return set, warnings
}

// position is the phase's place in the walk (spine_order; the mandatory list
// when the registry declares no spine_order); -1 for a phase the walk does
// not order (an optional insertion, or a mandatory phase outside spine_order),
// which therefore never moves the frontier and is never marked skipped.
func (m mandatorySet) position(phase string) int {
	order := m.order
	if len(order) == 0 {
		order = m.mandatory // a registry without spine_order walks the mandatory list
	}
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return -1
}

// streamReader reads a cycle's Signal Center stream once per (mtime, size):
// a sealed cycle's stream never changes, and the board re-collects on every
// tick for up to defaultMaxCycles cycles.
type streamReader struct {
	mu    sync.Mutex
	cache map[string]streamEntry
}

type streamEntry struct {
	mtime    time.Time
	size     int64
	gates    map[string]bool
	outcomes []PhaseRun
	warn     string
}

func newStreamReader() *streamReader { return &streamReader{cache: map[string]streamEntry{}} }

// read returns the phases whose contract gate verified their deliverables
// (gatesignal.CodeVerified) and the phase outcomes recorded so far
// (signalcenter.KindPhaseOutcome: verdict and wall clock), in stream order. A
// missing stream yields nothing — a mark is evidence, never inferred; a torn
// line is skipped and a read that stops early is said (warn).
func (r *streamReader) read(ws string) (gates map[string]bool, outcomes []PhaseRun, warn string) {
	path := filepath.Join(ws, signalcenter.StreamFileName)
	info, err := os.Stat(path)
	if err != nil {
		return map[string]bool{}, nil, ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.cache[path]; ok && e.mtime.Equal(info.ModTime()) && e.size == info.Size() {
		return e.gates, e.outcomes, e.warn
	}
	e := streamEntry{mtime: info.ModTime(), size: info.Size(), gates: map[string]bool{}}
	e.outcomes, e.warn = scanStream(path, e.gates)
	r.cache[path] = e
	return e.gates, e.outcomes, e.warn
}

func scanStream(path string, gates map[string]bool) (outcomes []PhaseRun, warn string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev signalcenter.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil || ev.Phase == "" {
			continue
		}
		switch {
		case ev.Code == gatesignal.CodeVerified:
			gates[ev.Phase] = true
		case ev.Kind == signalcenter.KindPhaseOutcome:
			ms, _ := strconv.ParseInt(ev.Fields["duration_ms"], 10, 64)
			outcomes = append(outcomes, PhaseRun{Phase: ev.Phase, Verdict: ev.Fields["verdict"], DurationMS: ms, Attempt: ev.Attempt})
		}
	}
	if err := sc.Err(); err != nil {
		warn = fmt.Sprintf("%s: %v — the plan may miss later phases", signalcenter.StreamFileName, err)
	}
	return outcomes, warn
}

// phaseHistory is the ONE precedence rule for "what ran": a running cycle
// reads the Signal Center stream (phase-timing.json is flushed at closeout,
// so the stream is the live record); a sealed cycle reads phase-timing.json
// (the durable record, joined with the model attribution); either falls back
// to the other when its own record is empty.
func phaseHistory(cs CycleSummary, outcomes []PhaseRun) []PhaseRun {
	if cs.State == StateRunning {
		if len(outcomes) > 0 {
			return outcomes
		}
		return cs.Phases
	}
	if len(cs.Phases) > 0 {
		return cs.Phases
	}
	return outcomes
}

// readPlan projects cs (already state-assigned) onto its PhasePlan, plus the
// warnings its sources raised. nil when the cycle has no run workspace.
func readPlan(set mandatorySet, ws string, cs CycleSummary, loop LoopStatus, streams *streamReader) (*PhasePlan, []string) {
	if !cs.HasWorkspace {
		return nil, nil
	}
	var warnings []string
	warn := func(w string) {
		if w != "" {
			warnings = append(warnings, fmt.Sprintf("cycle %d %s", cs.ID, w))
		}
	}
	gates, outcomes, streamWarn := streams.read(ws)
	warn(streamWarn)
	running := cs.State == StateRunning
	current := cs.CurrentPhase
	plan := &PhasePlan{Mandatory: set.mandatory}
	last, rounds, order := runOrder(phaseHistory(cs, outcomes), current, running)
	frontier := -1 // walk position of the furthest ordered phase the cycle reached
	for _, p := range order {
		if pos := set.position(p); pos > frontier {
			frontier = pos
		}
	}
	required := map[string]bool{}
	for _, p := range set.mandatory {
		required[p] = true
		if _, ran := last[p]; !ran && p != current {
			order = append(order, p)
		}
	}
	held := map[string]bool{} // everything the sequence holds: ran, ongoing, or scheduled by the floor
	for _, p := range order {
		held[p] = true
	}
	for _, p := range order {
		step := PlanStep{Phase: p, Optional: !required[p] && !set.conditional[p], Conditional: set.conditional[p], GateVerified: gates[p]}
		run, ran := last[p]
		switch {
		case running && p == current:
			step.Status, step.Rounds = stepOngoing, rounds[p]
			plan.Ongoing, plan.OngoingSince = p, loop.PhaseStartedAt
		case ran:
			step.Status, step.DurationMS, step.Rounds = verdictStatus(run.Verdict), run.DurationMS, rounds[p]
		case set.position(p) >= 0 && set.position(p) < frontier:
			step.Status = stepSkipped
		case running:
			step.Status = stepPending
			plan.Remaining = append(plan.Remaining, p)
		default:
			step.Status = stepUnreached
		}
		plan.count(step)
		plan.Steps = append(plan.Steps, step)
	}
	plan.Total = len(plan.Steps)
	var advisorWarn string
	plan.AdvisorProposed, plan.AdvisorSkips, plan.AdvisorOverridden, advisorWarn = advisorProposal(ws, held, last)
	warn(advisorWarn)
	return plan, warnings
}

// runOrder folds the phase history into its last run per phase, its round
// count, and the phases in first-run order (the current phase appended when
// it has not run yet).
func runOrder(history []PhaseRun, current string, running bool) (last map[string]PhaseRun, rounds map[string]int, order []string) {
	last, rounds = map[string]PhaseRun{}, map[string]int{}
	for _, p := range history {
		if _, seen := last[p.Phase]; !seen {
			order = append(order, p.Phase)
		}
		last[p.Phase] = p
		rounds[p.Phase]++
	}
	if running && current != "" {
		if _, seen := last[current]; !seen {
			order = append(order, current)
		}
	}
	return last, rounds, order
}

// count tallies a step into the plan's headline numbers: a required step is
// a mandatory one or a conditional-mandatory one that ran.
func (p *PhasePlan) count(step PlanStep) {
	isRequired := !step.Optional && (!step.Conditional || step.Status != stepUnreached)
	if isRequired {
		p.Required++
	}
	if step.Status == StatePass {
		p.Passed++
		if isRequired {
			p.PassedRequired++
		}
	}
}

func verdictStatus(verdict string) string {
	switch verdict {
	case "PASS":
		return StatePass
	case "WARN":
		return StateWarn
	case "FAIL":
		return StateFail
	}
	return StateIncomplete
}

// advisorProposal reads the advisor's newest plan (phase-replan.json, else
// phase-plan.json — a router.PhasePlanEntry array) and sorts its entries
// against the sequence: proposed to run and not held by it (not run, not
// ongoing, not scheduled by the floor); proposed to skip and did not run;
// proposed to skip but ran anyway (the mandatory floor overrode the
// proposal). Absent: nothing proposed. A newer file that is present but torn
// is said (warn) and yields nothing — never the older file mislabelled as
// current.
func advisorProposal(ws string, held map[string]bool, ran map[string]PhaseRun) (proposed, skips, overridden []string, warn string) {
	var entries []router.PhasePlanEntry
	for _, name := range []string{replanFile, planFile} {
		ok, err := readJSON(filepath.Join(ws, name), &entries)
		if err != nil {
			return nil, nil, nil, fmt.Sprintf("%s: %v — advisor proposal not shown", name, err)
		}
		if ok {
			break
		}
	}
	for _, e := range entries {
		_, did := ran[e.Phase]
		switch {
		case e.Run && !held[e.Phase]:
			proposed = append(proposed, e.Phase)
		case !e.Run && did:
			overridden = append(overridden, e.Phase)
		case !e.Run:
			skips = append(skips, e.Phase)
		}
	}
	return proposed, skips, overridden, ""
}
