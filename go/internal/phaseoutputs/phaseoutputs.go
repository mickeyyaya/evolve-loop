// Package phaseoutputs accounts, per cycle and per completed phase, for the
// data a reviewer needs: the report, the dispatched prompt, the event stream,
// and the usage record.
//
// The question it answers is the operator's, verbatim: "can the system
// correctly track the output, and did each phase generate enough data for
// review?" Until now nobody could answer it without hand-listing a workspace —
// which is how the pipeline ran for weeks with orchestrator-report.md having
// zero writers, dossiers 89% stubs, and a shadow record whose absence
// conflated "didn't comply" with "never ran". Absence only means something
// when somebody is looking for the presence — this package is the looker.
//
// Pure: input is the completed-phase list and a name→size listing of the
// workspace; output is rows and gaps. The CLI (`evolve cycle outputs`) and the
// wave monitor do the I/O.
package phaseoutputs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Artifact is one expected output's observed state.
type Artifact struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	// Empty marks a zero-byte file: it satisfies a stat and informs nobody,
	// so presence alone is not data.
	Empty bool  `json:"empty,omitempty"`
	Bytes int64 `json:"bytes"`
}

// Row is one completed phase's accounting.
type Row struct {
	Phase  string   `json:"phase"`
	Agent  string   `json:"agent"`
	Report Artifact `json:"report"`
	Prompt Artifact `json:"prompt"`
	Events Artifact `json:"events"`
	Usage  Artifact `json:"usage"`
	// Exempt marks a bookkeeping phase that dispatches no agent and so owes no
	// report — exempted explicitly, or every such phase would spam the gap
	// list until the signal drowned.
	Exempt bool `json:"exempt,omitempty"`
}

// Result is Survey's output: one Row per completed phase.
type Result struct {
	Rows []Row `json:"rows"`
}

// noDispatchPhases is the exemption register: a phase listed here owes no
// review outputs, so its gaps can never fire. It is EMPTY by evidence: the two
// candidates guessed during development (inherited-defect-reconcile,
// coverage-gate) both turn out to be fully-dispatched phases with complete
// outputs in live workspaces (cycles 1432/1442) — the guess was refuted by
// exactly the kind of look this package institutionalises. An entry added here
// must cite the live cycle that shows the phase legitimately producing
// nothing.
var noDispatchPhases = map[string]bool{}

// agentFor resolves the artifact/stream basename stem for a phase from the
// registry — the SSOT — falling back to the phase name for minted phases the
// registry does not know.
func agentFor(phase string) string {
	if c, ok := phasecontract.For(phase); ok && c.AgentName != "" {
		return c.AgentName
	}
	return phase
}

// Survey accounts for every completed phase against the workspace listing
// (name → size in bytes).
func Survey(completedPhases []string, listing map[string]int64) Result {
	res := Result{}
	for _, phase := range completedPhases {
		agent := agentFor(phase)
		row := Row{
			Phase:  phase,
			Agent:  agent,
			Exempt: noDispatchPhases[phase],
			// Live naming truth (cycles 1432/1441/1442, corrected by review):
			// the REPORT follows the registry (retrospective-report.md,
			// test-report.md). Usage is PHASE-named everywhere — the C1
			// chokepoint writes %s-usage.json from the phase. Prompt and
			// events are PHASE-named for runner-dispatched phases (the runner
			// sets Agent = phase), but retro dispatches outside that path and
			// its prompt is AGENT-named (retrospective-prompt.txt) — the
			// fallback candidate's one live consumer. Retro has NO events
			// stream under either name live (retro-observer-events.ndjson is
			// the observer's, a different mechanism): that gap is REAL and
			// intentionally reported — fix queued as retro-events-stream item.
			Report: observe(listing, phasecontract.ArtifactFilename(phase)),
			Prompt: observe(listing, phase+"-prompt.txt", agent+"-prompt.txt"),
			Events: observe(listing, phase+"-events.ndjson", agent+"-events.ndjson"),
			Usage:  observe(listing, phase+"-usage.json", agent+"-usage.json"),
		}
		res.Rows = append(res.Rows, row)
	}
	return res
}

// observe checks the first candidate name that exists; candidates exist
// because stream/usage files are named after the AGENT for dispatched phases
// but a few writers key on the phase name.
func observe(listing map[string]int64, candidates ...string) Artifact {
	for _, name := range candidates {
		if size, ok := listing[name]; ok {
			return Artifact{Name: name, Present: true, Empty: size == 0, Bytes: size}
		}
	}
	return Artifact{Name: candidates[0]}
}

// reviewable reports whether an artifact carries data a reviewer can read.
func (a Artifact) reviewable() bool { return a.Present && !a.Empty }

// Gaps names every completed phase whose review data is missing or empty —
// the phase and the artifact, because an operator holding a gap list needs to
// know which file to go looking for, not that "something" was lost.
func (r Result) Gaps() []string {
	var out []string
	for _, row := range r.Rows {
		if row.Exempt {
			continue
		}
		for _, a := range []Artifact{row.Report, row.Prompt, row.Events, row.Usage} {
			if !a.reviewable() {
				state := "missing"
				if a.Present {
					state = "empty"
				}
				out = append(out, fmt.Sprintf("%s: %s %s", row.Phase, a.Name, state))
			}
		}
	}
	sort.Strings(out)
	return out
}

// SummaryLine is the one line the wave monitor prints per cycle.
func (r Result) SummaryLine() string {
	surveyed, complete := 0, 0
	for _, row := range r.Rows {
		if row.Exempt {
			continue
		}
		surveyed++
		if row.Report.reviewable() && row.Prompt.reviewable() && row.Events.reviewable() && row.Usage.reviewable() {
			complete++
		}
	}
	if surveyed == 0 {
		return "phase outputs: nothing to survey"
	}
	line := fmt.Sprintf("phase outputs: %d/%d complete", complete, surveyed)
	if gaps := r.Gaps(); len(gaps) > 0 {
		line += " — gaps: " + strings.Join(gaps, "; ")
	}
	return line
}
