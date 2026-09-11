package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// triageDecisionFile is the triage agent's companion deliverable (written
// beside triage-report.md per agents/evolve-triage.md); cycleoutcome,
// triagecap and inboxmover read the same name.
const triageDecisionFile = "triage-decision.json"

// readJSON decodes path into v. Absent ⇒ (false, nil): the ordinary sparse
// workspace shape, silent. Present but unreadable or unparsable ⇒ (false, err)
// so the caller can surface it in Warnings — a half-written
// failure-decision.json must never look like "no failure recorded yet".
func readJSON(path string, v any) (bool, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(buf, v); err != nil {
		return false, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return true, nil
}

// readJSONWarn is readJSON with the error rendered as a warning line.
func readJSONWarn(path string, v any, warnings *[]string) bool {
	ok, err := readJSON(path, v)
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("%s: %v", filepath.Base(filepath.Dir(path)), err))
	}
	return ok
}

// triageDecision is the slice of triage-decision.json the dashboard shows.
type triageDecision struct {
	TopN []struct {
		ID string `json:"id"`
	} `json:"top_n"`
}

// readCycle assembles one cycle from its run workspace (when present) and its
// committed dossier (when present). State is assigned afterwards by
// assignState, because it depends on the loop's live status.
func readCycle(root string, id int, d *dossier.Dossier) (CycleSummary, []string) {
	cs := CycleSummary{ID: id}
	var warnings []string
	if d != nil {
		cs.HasDossier = true
		cs.Verdict, cs.CommitSHA, cs.Goal = d.FinalVerdict, d.CommitSHA, d.Goal
		cs.StartedAt, cs.EndedAt = parseTime(d.StartedAt), parseTime(d.EndedAt)
	}
	ws := core.RunWorkspacePath(root, id)
	if info, err := os.Stat(ws); err == nil && info.IsDir() {
		cs.HasWorkspace = true
		warnings = append(warnings, readWorkspace(ws, &cs)...)
	}
	f, w := readFailure(ws, d)
	cs.Failure = f
	warnings = append(warnings, w...)
	return cs, warnings
}

// readWorkspace fills the workspace-derived fields of cs.
func readWorkspace(ws string, cs *CycleSummary) []string {
	var warnings []string
	var run cyclestate.CycleState
	if readJSONWarn(filepath.Join(ws, core.RunStateFile), &run, &warnings) {
		cs.AuditRounds = run.AuditDispatches
		if cs.AuditRounds == 0 {
			cs.AuditRounds = countPhase(run.CompletedPhases, string(core.PhaseAudit))
		}
		if cs.StartedAt.IsZero() {
			cs.StartedAt = parseTime(run.StartedAt)
		}
	}
	entries, err := phasetiming.Read(ws)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		warnings = append(warnings, fmt.Sprintf("cycle %d %s: %v", cs.ID, phasetiming.FileName, err))
	}
	cs.Phases = phaseRuns(entries, readLLMCalls(ws))
	for _, p := range cs.Phases {
		cs.Tokens += p.Tokens
		if cs.StartedAt.IsZero() || (!p.StartedAt.IsZero() && p.StartedAt.Before(cs.StartedAt)) {
			cs.StartedAt = p.StartedAt
		}
		if p.EndedAt.After(cs.EndedAt) {
			cs.EndedAt = p.EndedAt
		}
	}
	var td triageDecision
	if readJSONWarn(filepath.Join(ws, triageDecisionFile), &td, &warnings) {
		for _, t := range td.TopN {
			if t.ID != "" {
				cs.Tasks = append(cs.Tasks, t.ID)
			}
		}
	}
	return warnings
}

// phaseRuns turns timing entries into PhaseRuns in run order, numbers each
// phase's occurrence, and joins the final attempt whose terminal timestamp lies
// inside that phase window. This prevents retries in round one from shifting
// every later repeated phase's model attribution.
func phaseRuns(entries []phasetiming.Entry, calls []llmCall) []PhaseRun {
	sort.SliceStable(entries, func(i, j int) bool {
		return parseTime(entries[i].StartedAt).Before(parseTime(entries[j].StartedAt))
	})
	callsByPhase := map[string][]llmCall{}
	for _, c := range calls {
		callsByPhase[c.Phase] = append(callsByPhase[c.Phase], c)
	}
	windows := phaseCallWindows(entries)
	seen := map[string]int{}
	usedCalls := map[string]map[int]struct{}{}
	out := make([]PhaseRun, 0, len(entries))
	for index, e := range entries {
		seen[e.Phase]++
		p := PhaseRun{Phase: e.Phase, Verdict: e.Verdict, Archetype: e.Archetype,
			StartedAt: parseTime(e.StartedAt), EndedAt: parseTime(e.EndedAt),
			DurationMS: e.DurationMS, Attempt: e.AttemptCount, Round: seen[e.Phase],
			Tokens: e.Tokens.Input + e.Tokens.Output, Model: e.ResolvedModel}
		if usedCalls[e.Phase] == nil {
			usedCalls[e.Phase] = map[int]struct{}{}
		}
		if c, callIndex, ok := callForPhaseWindow(windows[index], callsByPhase[e.Phase], seen[e.Phase], usedCalls[e.Phase]); ok {
			p.CLI, p.Model = c.CLI, c.Model
			usedCalls[e.Phase][callIndex] = struct{}{}
		}
		out = append(out, p)
	}
	return out
}

type phaseCallWindow struct {
	start, end        time.Time
	previousEnd       time.Time
	nextStart         time.Time
	indistinguishable bool
}

func phaseCallWindows(entries []phasetiming.Entry) []phaseCallWindow {
	windows := make([]phaseCallWindow, len(entries))
	previousByPhase := map[string]int{}
	for index, entry := range entries {
		start, end := parseTime(entry.StartedAt), parseTime(entry.EndedAt)
		if !end.IsZero() && !strings.Contains(entry.EndedAt, ".") {
			end = end.Add(time.Second - time.Nanosecond)
		}
		windows[index] = phaseCallWindow{start: start, end: end}
		if previous, ok := previousByPhase[entry.Phase]; ok {
			windows[index].previousEnd = windows[previous].end
			windows[previous].nextStart = start
			if !start.IsZero() && start.Equal(windows[previous].start) {
				windows[index].indistinguishable = true
				windows[previous].indistinguishable = true
			}
		}
		previousByPhase[entry.Phase] = index
	}
	return windows
}

func callForPhaseWindow(
	window phaseCallWindow,
	calls []llmCall,
	occurrence int,
	used map[int]struct{},
) (llmCall, int, bool) {
	if !window.start.IsZero() && !window.end.IsZero() {
		if window.indistinguishable {
			return llmCall{}, -1, false
		}
		var selected llmCall
		var selectedAt time.Time
		selectedIndex := -1
		for index, call := range calls {
			if _, alreadyUsed := used[index]; alreadyUsed {
				continue
			}
			terminal := parseTime(call.EndedAt)
			if terminal.IsZero() {
				terminal = parseTime(call.TS)
			}
			if terminal.IsZero() || terminal.Before(window.start) || terminal.After(window.end) {
				continue
			}
			callStart := parseTime(call.StartedAt)
			if !callStart.IsZero() && (callStart.Before(window.start) || callStart.After(window.end)) {
				continue
			}
			// A legacy call with no start time inside the previous phase's
			// whole-second overlap could have terminated either round. Withhold
			// it from the later round just as nextStart withholds it from the
			// earlier round.
			if callStart.IsZero() && !window.previousEnd.IsZero() &&
				!terminal.Before(window.start) && !terminal.After(window.previousEnd) {
				continue
			}
			// Whole-second phase records can overlap at a repeated phase's
			// boundary. A call that starts in the later round belongs there; a
			// timestamp-only legacy call at or after that boundary is ambiguous
			// and is likewise withheld from the earlier round.
			if !window.nextStart.IsZero() && !terminal.Before(window.nextStart) &&
				(callStart.IsZero() || !callStart.Before(window.nextStart)) {
				continue
			}
			if selectedAt.IsZero() || terminal.After(selectedAt) {
				selected, selectedAt, selectedIndex = call, terminal, index
			}
		}
		if !selectedAt.IsZero() {
			return selected, selectedIndex, true
		}
		// A timestamped ledger that does not overlap this entry carries no safe
		// association; leave routing blank instead of borrowing another round.
		for _, call := range calls {
			if !parseTime(call.EndedAt).IsZero() || !parseTime(call.TS).IsZero() {
				return llmCall{}, -1, false
			}
		}
	}
	index := occurrence - 1
	if index >= 0 && index < len(calls) {
		if _, alreadyUsed := used[index]; !alreadyUsed {
			return calls[index], index, true
		}
	}
	return llmCall{}, -1, false
}

func countPhase(phases []string, name string) int {
	n := 0
	for _, p := range phases {
		if p == name {
			n++
		}
	}
	return n
}

// assignState derives the closed-vocabulary state type and its human name.
// "halted" is the ADR-0072 SYSTEM level (policy.LevelSystem), never a literal;
// the brake and the running cycle both come from the loop status.
func assignState(cs CycleSummary, loop LoopStatus) CycleSummary {
	switch {
	case loop.Running && loop.CycleID == cs.ID:
		cs.State, cs.CurrentPhase = StateRunning, loop.Phase
		cs.StateName = "running · " + loop.Phase
	case cs.Failure != nil && cs.Failure.Level == policy.LevelSystem:
		cs.State, cs.StateName = StateHalted, "halted · "+cs.Failure.Category
	case cs.HasDossier && cs.Verdict == dossier.VerdictPass:
		cs.State, cs.StateName = StatePass, "pass"
	case cs.HasDossier && cs.Verdict == dossier.VerdictWarn:
		cs.State, cs.StateName = StateWarn, "warn"
	case cs.HasDossier && cs.Verdict == dossier.VerdictFail:
		cs.State, cs.StateName = StateFail, "fail"
	default:
		cs.State, cs.StateName = StateIncomplete, "incomplete"
		if loop.BrakeEngaged {
			cs.StateName = "paused (brake)"
		}
	}
	return cs
}
