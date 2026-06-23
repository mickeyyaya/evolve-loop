// Package soakreport is the R8.3 soak evidence reporter — the read-only
// instrument the EVOLVE_PHASE_RECOVERY enforce flip (plan R8.5) is gated on.
//
// Every ADR-0044/0045 component already leaves DURABLE shadow records:
// interaction ledgers carry C2 fatal_pane_shadow, I2 salvage-ladder, I3
// kernel_answer, and I4 rule_shadow_fire outcomes; the phase observer's
// events ndjson carries the C4/C3 INCIDENT envelopes with their
// policy-decided action/action_reason. This package aggregates them per
// batch and renders the per-component evidence table against the plan §6
// bars. It never mutates anything — the sweep (cmd_loop) and the flip
// (operator) act; the reporter only accounts.
package soakreport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/cyclehealth"
)

// CycleRow is one cycle's R6 outcome classification.
type CycleRow struct {
	Cycle   int
	Outcome string
	Detail  string
}

// ComponentEvidence is one component's aggregated soak evidence.
type ComponentEvidence struct {
	Component string         // "C2", "C4/C3", "I2", "I3", "I4"
	Bar       string         // the plan §6 evidence bar, verbatim
	Counts    map[string]int // evidence counters (stable keys)
	Notes     []string       // caveats the human reviewer must weigh
}

// Report is the per-batch soak evidence bundle.
type Report struct {
	Cycles     []CycleRow
	Components []ComponentEvidence
}

// bars is the plan §6 evidence-bar text, the single rendered source.
var bars = map[string]string{
	"C2":    "≥10 observations over ≥3 batches; 0 would-fires on phases that later PASSed; ≥1 true positive",
	"C4/C3": "0 would-kill decisions on phases that completed within the next 2 intervals",
	"I2":    "every would-salvage candidate verified offline (exists + passes contract); 0 would-acts on approved phases",
	"I3":    "answers ⊂ closed vocabulary; 0 answers contradicting the eventual resolution",
	"I4":    "promoted signatures ≥12 chars; 0 healthy-corpus hits (re-checked at flip)",
}

// componentOrder fixes the rendered section order (stable diffs across soaks).
var componentOrder = []string{"C2", "C4/C3", "I2", "I3", "I4"}

// Collect aggregates the durable soak evidence for the given cycles under
// projectRoot. Missing files are absent evidence, never errors — the report
// must render for any historical batch.
func Collect(projectRoot string, cycles []int) Report {
	r := Report{}
	comp := map[string]*ComponentEvidence{}
	for _, name := range componentOrder {
		comp[name] = &ComponentEvidence{Component: name, Bar: bars[name], Counts: map[string]int{}}
	}

	for _, c := range cycles {
		ws := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", c))
		oc, detail := cyclehealth.ClassifyOutcome(ws)
		r.Cycles = append(r.Cycles, CycleRow{Cycle: c, Outcome: string(oc), Detail: detail})

		entries, err := os.ReadDir(ws)
		if err != nil {
			comp["C2"].Notes = appendOnce(comp["C2"].Notes, fmt.Sprintf("cycle-%d workspace unreadable — no evidence collected", c))
			continue
		}
		for _, e := range entries {
			name := e.Name()
			switch {
			case strings.HasSuffix(name, "-interactions.ndjson"):
				collectInteractions(filepath.Join(ws, name), comp)
			case strings.HasSuffix(name, "-observer-events.ndjson"):
				collectObserverIncidents(filepath.Join(ws, name), comp["C4/C3"])
			}
		}
	}

	for _, name := range componentOrder {
		r.Components = append(r.Components, *comp[name])
	}
	return r
}

// collectInteractions folds one interaction ledger into the component set.
func collectInteractions(path string, comp map[string]*ComponentEvidence) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var out struct {
			Kind   string `json:"kind"`
			Result string `json:"result"`
			RuleID string `json:"rule_id"`
		}
		if json.Unmarshal([]byte(line), &out) != nil {
			continue
		}
		switch out.Kind {
		case "fatal_pane_shadow":
			comp["C2"].Counts[out.Result]++
		case "salvage":
			comp["I2"].Counts[out.Result]++
		case "kernel_answer":
			comp["I3"].Counts["kernel_answer"]++
			comp["I3"].Counts["result:"+out.Result]++
		case "rule_shadow_fire":
			if out.RuleID != "" {
				comp["I4"].Counts[out.Result+":"+out.RuleID]++
			}
		}
	}
}

// collectObserverIncidents folds one observer events ndjson — INCIDENT
// severity only — into the C4/C3 evidence.
func collectObserverIncidents(path string, c *ComponentEvidence) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var env struct {
			Type     string `json:"type"`
			Severity string `json:"severity"`
			Data     struct {
				Action       string `json:"action"`
				ActionReason string `json:"action_reason"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(line), &env) != nil || env.Severity != "INCIDENT" {
			continue
		}
		c.Counts["incident:"+env.Type]++
		if env.Data.Action != "" {
			c.Counts["action:"+env.Data.Action]++
		}
	}
}

// Render produces the markdown soak report: per-cycle outcomes, then one
// section per component with its §6 bar and sorted evidence counters.
func (r Report) Render() string {
	var b strings.Builder
	b.WriteString("# Soak evidence report (EVOLVE_PHASE_RECOVERY promotion program)\n\n")
	b.WriteString("## Cycle outcomes (R6)\n\n| Cycle | Outcome | Detail |\n|---|---|---|\n")
	for _, c := range r.Cycles {
		fmt.Fprintf(&b, "| cycle-%d | %s | %s |\n", c.Cycle, c.Outcome, c.Detail)
	}
	for _, comp := range r.Components {
		fmt.Fprintf(&b, "\n## %s\n\nBar: %s\n\n", comp.Component, comp.Bar)
		if len(comp.Counts) == 0 {
			b.WriteString("_no evidence recorded this batch_\n")
		} else {
			b.WriteString("| Evidence | Count |\n|---|---|\n")
			keys := make([]string, 0, len(comp.Counts))
			for k := range comp.Counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&b, "| %s | %d |\n", k, comp.Counts[k])
			}
		}
		for _, n := range comp.Notes {
			fmt.Fprintf(&b, "\n> NOTE: %s\n", n)
		}
	}
	return b.String()
}

func appendOnce(notes []string, n string) []string {
	for _, x := range notes {
		if x == n {
			return notes
		}
	}
	return append(notes, n)
}
