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
	// NotOwed marks an artifact this phase legitimately does not produce (a
	// native phase's prompt, a NoArtifact contract's report). Marked ON the
	// row rather than only skipped in Gaps() so a consumer reading rows
	// cannot re-derive the absence as a gap the summary disagrees with.
	NotOwed bool `json:"not_owed,omitempty"`
}

// Row is one completed phase's accounting.
type Row struct {
	Phase  string   `json:"phase"`
	Agent  string   `json:"agent"`
	Report Artifact `json:"report"`
	Prompt Artifact `json:"prompt"`
	Events Artifact `json:"events"`
	Usage  Artifact `json:"usage"`
}

// Result is Survey's output: one Row per completed phase.
type Result struct {
	Rows []Row `json:"rows"`
}

// nativePhases lists phases that complete natively in Go with no agent
// dispatch: they owe no prompt and no events (nothing was dispatched to
// stream), and their no-report fact comes from the registry's NoArtifact
// contract. They ALWAYS owe usage — the C1 chokepoint writes a sidecar for
// every recorded phase, native included, so a missing one is a real recording
// failure. Admission requires a cited live cycle: ship enters on cycles
// 1452/1453 (in completed_phases with ship-usage.json and nothing else; the
// earlier whole-row exemption guesses — inherited-defect-reconcile,
// coverage-gate — were refuted by cycles 1432/1442 and stay OUT).
var nativePhases = map[string]bool{"ship": true}

// Survey accounts for every completed phase against the workspace listing
// (name → size in bytes). Duplicate completed_phases entries (a re-dispatched
// phase records once per completion — cycle-1452 carried audit twice) collapse
// to one row, or the completeness denominator counts the same files twice.
//
// resolver is the SAME contract-resolution vocabulary the contract gate and
// bridge use — callers pass a catalog-aware phasecontract.CatalogResolver so
// spec-derived phases (memo, minted phases) resolve their real report names.
// Cycle-1452's false `memo-report.md missing` gap came from resolving through
// the compiled map alone; the tempting alternative — adding a builtin memo
// entry — was caught in review as WORSE: builtins shadow spec-derived
// contracts, so it would have silently dropped memo's required sections from
// the live gate. A nil resolver degrades to builtin-only.
func Survey(completedPhases []string, listing map[string]int64, resolver phasecontract.Resolver) Result {
	if resolver == nil {
		resolver = phasecontract.BuiltinResolver{}
	}
	res := Result{}
	seen := map[string]bool{}
	for _, phase := range completedPhases {
		if seen[phase] {
			continue
		}
		seen[phase] = true
		c, known := resolver.Resolve(phase)
		agent := phase
		if known && c.AgentName != "" {
			agent = c.AgentName
		}
		report := phase + "-report.md"
		if known && c.ArtifactName != "" {
			report = c.ArtifactName
		}
		row := Row{
			Phase: phase,
			Agent: agent,
			// Live naming truth (cycles 1432/1441/1442/1452, corrected by
			// review + first monitored wave): the REPORT follows the resolved
			// contract (retrospective-report.md, test-report.md, memo.md).
			// Usage is PHASE-named everywhere — the C1 chokepoint writes
			// %s-usage.json from the phase. Prompt and events are PHASE-named
			// for runner-dispatched phases (the runner sets Agent = phase),
			// but retro dispatches outside that path and its prompt is
			// AGENT-named (retrospective-prompt.txt) — the fallback
			// candidate's one live consumer. Retro has NO events stream under
			// either name live (retro-observer-events.ndjson is the
			// observer's, a different mechanism): that gap is REAL and
			// intentionally reported — fix queued as retro-events-stream item.
			Report: observe(listing, report),
			Prompt: observe(listing, phase+"-prompt.txt", agent+"-prompt.txt"),
			Events: observe(listing, phase+"-events.ndjson", agent+"-events.ndjson"),
			Usage:  observe(listing, phase+"-usage.json", agent+"-usage.json"),
		}
		// Owed-ness. The no-report fact is the resolved contract's (NoArtifact
		// — ship's result is the pushed commit, not a file); no-prompt/
		// no-events is the native register's. Usage is never trimmed.
		if known && c.NoArtifact {
			row.Report.NotOwed = true
		}
		if nativePhases[phase] {
			row.Prompt.NotOwed = true
			row.Events.NotOwed = true
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

// satisfied reports whether this artifact's obligation is met: either the
// phase does not owe it, or it is present with actual content.
func (a Artifact) satisfied() bool { return a.NotOwed || (a.Present && !a.Empty) }

// Gaps names every completed phase whose OWED review data is missing or empty
// — the phase and the artifact, because an operator holding a gap list needs
// to know which file to go looking for, not that "something" was lost.
func (r Result) Gaps() []string {
	var out []string
	for _, row := range r.Rows {
		for _, a := range []Artifact{row.Report, row.Prompt, row.Events, row.Usage} {
			if !a.satisfied() {
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
		surveyed++
		if row.Report.satisfied() && row.Prompt.satisfied() && row.Events.satisfied() && row.Usage.satisfied() {
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
