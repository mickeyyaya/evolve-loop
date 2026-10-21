package recoveryretro

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const MinFailures = 2

type PhaseModel struct {
	Phase string
	Model string
}

type Try struct {
	Cycle       int
	Verdict     string
	Class       string
	Fingerprint string
	Reason      string
	CommitSHA   string
	Models      []PhaseModel
}

type Recovery struct {
	TaskID   string
	Failures []Try
	Pass     Try
	Rungs    []string
}

func Recoveries(history []*dossier.Dossier, pass *dossier.Dossier) []Recovery {
	if pass == nil || pass.FinalVerdict != dossier.VerdictPass || pass.Tasks == nil {
		return nil
	}
	ordered := slices.Clone(history)
	slices.SortStableFunc(ordered, func(a, b *dossier.Dossier) int { return cycleOf(a) - cycleOf(b) })
	var out []Recovery
	for _, task := range *pass.Tasks {
		if failures := failuresSinceLastPass(ordered, task, pass.Cycle); len(failures) >= MinFailures {
			out = append(out, Recovery{TaskID: task, Failures: failures, Pass: tryOf(pass)})
		}
	}
	return out
}

func failuresSinceLastPass(ordered []*dossier.Dossier, task string, before int) []Try {
	var failures []Try
	seen := map[int]bool{}
	for _, d := range ordered {
		if d == nil || d.Cycle >= before || seen[d.Cycle] || !names(d, task) {
			continue
		}
		seen[d.Cycle] = true
		switch d.FinalVerdict {
		case dossier.VerdictPass:
			failures = nil
		case dossier.VerdictFail:
			failures = append(failures, tryOf(d))
		}
	}
	return failures
}

func cycleOf(d *dossier.Dossier) int {
	if d == nil {
		return 0
	}
	return d.Cycle
}

func names(d *dossier.Dossier, task string) bool {
	return d.Tasks != nil && slices.Contains(*d.Tasks, task)
}

func tryOf(d *dossier.Dossier) Try {
	t := Try{Cycle: d.Cycle, Verdict: d.FinalVerdict, CommitSHA: d.CommitSHA, Models: modelsOf(d.Phases)}
	if d.Failure != nil {
		t.Class, t.Fingerprint = d.Failure.PreClass, d.Failure.Fingerprint
		if len(d.Failure.Reasons) > 0 {
			t.Reason = d.Failure.Reasons[0]
		}
	}
	return t
}

func modelsOf(phases []dossier.PhaseRecord) []PhaseModel {
	out := make([]PhaseModel, 0, len(phases))
	for _, p := range phases {
		out = append(out, PhaseModel{Phase: p.Name, Model: p.ResolvedModel})
	}
	return out
}
