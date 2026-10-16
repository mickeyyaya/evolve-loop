// Package phaseoutputs accounts, per cycle and per completed phase, for the
// data a reviewer needs: the report, the dispatched prompt, the event
// stream, and the usage record.
package phaseoutputs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type Artifact struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Empty   bool   `json:"empty,omitempty"`
	Bytes   int64  `json:"bytes"`
	NotOwed bool   `json:"not_owed,omitempty"`
}

type Row struct {
	Phase  string   `json:"phase"`
	Agent  string   `json:"agent"`
	Report Artifact `json:"report"`
	Prompt Artifact `json:"prompt"`
	Events Artifact `json:"events"`
	Usage  Artifact `json:"usage"`
}

type Result struct {
	Rows []Row `json:"rows"`
}

var nativePhases = map[string]bool{"ship": true}

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
			Phase:  phase,
			Agent:  agent,
			Report: observe(listing, report),
			Prompt: observe(listing, phase+"-prompt.txt", agent+"-prompt.txt"),
			Events: observe(listing, phase+"-events.ndjson", agent+"-events.ndjson"),
			Usage:  observe(listing, phase+"-usage.json", agent+"-usage.json"),
		}
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

func observe(listing map[string]int64, candidates ...string) Artifact {
	for _, name := range candidates {
		if size, ok := listing[name]; ok {
			return Artifact{Name: name, Present: true, Empty: size == 0, Bytes: size}
		}
	}
	return Artifact{Name: candidates[0]}
}

func (a Artifact) satisfied() bool { return a.NotOwed || (a.Present && !a.Empty) }

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
