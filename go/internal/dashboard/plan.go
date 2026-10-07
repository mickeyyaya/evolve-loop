package dashboard

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
	planFile      = "phase-plan.json"
	replanFile    = "phase-replan.json"
	stepOngoing   = "ongoing"
	stepPending   = "pending"
	stepUnreached = "unreached"
	stepSkipped   = "skipped"
)

const notInWalk = -1

type mandatorySet struct {
	mandatory   []string
	conditional map[string]bool
	order       []string
}

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

func (m mandatorySet) position(phase string) int {
	order := m.order
	if len(order) == 0 {
		order = m.mandatory
	}
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return notInWalk
}

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
		case ev.Kind == signalcenter.KindPhaseOutcome && ev.Fields["verdict"] != "":
			ms, _ := strconv.ParseInt(ev.Fields["duration_ms"], 10, 64)
			outcomes = append(outcomes, PhaseRun{Phase: ev.Phase, Verdict: ev.Fields["verdict"], DurationMS: ms, Attempt: ev.Attempt})
		}
	}
	if err := sc.Err(); err != nil {
		warn = fmt.Sprintf("%s: %v — the plan may miss later phases", signalcenter.StreamFileName, err)
	}
	return outcomes, warn
}

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
	frontier := walkFrontier(set, order)
	order, required := scheduleMandatory(set, last, current, order)
	inSequence := map[string]bool{}
	for _, p := range order {
		inSequence[p] = true
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
	plan.AdvisorProposed, plan.AdvisorSkips, plan.AdvisorOverridden, advisorWarn = advisorProposal(ws, inSequence, last)
	warn(advisorWarn)
	return plan, warnings
}

func walkFrontier(set mandatorySet, order []string) int {
	frontier := notInWalk
	for _, p := range order {
		if pos := set.position(p); pos > frontier {
			frontier = pos
		}
	}
	return frontier
}

func scheduleMandatory(set mandatorySet, last map[string]PhaseRun, current string, order []string) ([]string, map[string]bool) {
	required := map[string]bool{}
	for _, p := range set.mandatory {
		required[p] = true
		if _, ran := last[p]; !ran && p != current {
			order = append(order, p)
		}
	}
	return order, required
}

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

func advisorProposal(ws string, inSequence map[string]bool, ran map[string]PhaseRun) (proposed, skips, overridden []string, warn string) {
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
		_, didRun := ran[e.Phase]
		switch {
		case e.Run && !inSequence[e.Phase]:
			proposed = append(proposed, e.Phase)
		case !e.Run && didRun:
			overridden = append(overridden, e.Phase)
		case !e.Run:
			skips = append(skips, e.Phase)
		}
	}
	return proposed, skips, overridden, ""
}
