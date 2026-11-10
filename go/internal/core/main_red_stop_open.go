package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	mainRedFixSource = "main-red-stop"
	mainRedFixKind   = "fix"
)

type mainRedStopOpener struct {
	now func() time.Time
}

type mainRedFixItem struct {
	ID            string   `json:"id"`
	Weight        float64  `json:"weight"`
	Kind          string   `json:"kind"`
	Priority      string   `json:"priority"`
	PriorityClass string   `json:"priority_class"`
	Origin        string   `json:"origin"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Fix           string   `json:"fix"`
	Files         []string `json:"files"`
	Acceptance    []string `json:"acceptance"`
	Source        string   `json:"source"`
}

func OpenMainRedStop(projectRoot string, v baseattr.Verdict) (bool, error) {
	return mainRedStopOpener{now: time.Now}.open(projectRoot, v)
}

func (o mainRedStopOpener) open(projectRoot string, v baseattr.Verdict) (bool, error) {
	evolveDir := filepath.Join(projectRoot, ".evolve")
	opened := false
	stop, err := mainstop.Update(evolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		opened = !s.Holds()
		return s.Opened(v, mainstop.FixItemID(v.Main), o.now()), nil
	})
	if err != nil {
		return false, fmt.Errorf("main-red stop: write the stop: %w", err)
	}
	raw, _ := json.Marshal(fixItemOf(stop))
	if _, err := inboxfile.Mint(filepath.Join(evolveDir, "inbox"), raw, o.now); err != nil {
		return opened, fmt.Errorf("main-red stop: file the fix item %s: %w", stop.FixItem, err)
	}
	return opened, nil
}

func fixItemOf(stop mainstop.Stop) mainRedFixItem {
	keys := baseattr.Names(stop.Keys)
	return mainRedFixItem{
		ID:            stop.FixItem,
		Weight:        1.0,
		Kind:          mainRedFixKind,
		Priority:      "critical",
		PriorityClass: inboxbatch.ClassCorrectness,
		Origin:        inboxbatch.OriginCycle,
		Title:         inboxbatch.Fit("Fix first: main is red at "+stop.MainSHA, inboxbatch.MaxFieldLen),
		Summary:       fmt.Sprintf("A lane found %s red on its base and on main at %s. No lane caused it, so each lane is refused or parked until main is green. The keys of the stop are in .evolve/main-red-stop.json.", keys, stop.MainSHA),
		Fix:           fmt.Sprintf("Reproduce %s on main, fix each test at its source, and ship the fix. The ship releases the stop when each key is green.", keys),
		Files:         keyPackages(stop.Keys),
		Acceptance: []string{
			inboxbatch.Fit("Each key is green on main: "+keys, inboxbatch.MaxAcceptanceLen),
			"A regression test fails before the fix and passes after it",
		},
		Source: mainRedFixSource,
	}
}

func keyPackages(keys []baseattr.Key) []string {
	packages := make([]string, 0, len(keys))
	for _, k := range keys {
		if !slices.Contains(packages, k.Package) {
			packages = append(packages, k.Package)
		}
	}
	return packages
}

type stopOpening struct {
	projectRoot, runID, phase, origin string
	cycle                             int
	verdict                           baseattr.Verdict
}

func (s stopOpening) through(sink MainRedSink, signals *signalcenter.Center) error {
	if len(s.verdict.MainRed) == 0 {
		return nil
	}
	opened, err := sink(s.projectRoot, s.verdict)
	if opened {
		signals.Emit(s.openedEvent(err))
	}
	return err
}

func (s stopOpening) openedEvent(fileErr error) signalcenter.Event {
	keys, fixItem := baseattr.Names(s.verdict.MainRed), mainstop.FixItemID(s.verdict.Main)
	e := signalcenter.Event{
		Cycle: s.cycle, RunID: s.runID, Phase: s.phase,
		Module: signalcenter.ModuleGateRepo, Origin: s.origin, Kind: signalcenter.KindLoopWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeMainRedStopOpened,
		Reason: fmt.Sprintf("main is red at %s (%s), so the main-red stop is open; next: build the inbox item %s, which leads the rank while the stop holds", s.verdict.Main, keys, fixItem),
		Fields: map[string]string{"keys": keys, "main": s.verdict.Main, "fix_item": fixItem, "fix_item_error": errText(fileErr)},
	}
	if fileErr != nil {
		e.Severity = signalcenter.SeverityIncident
		e.Reason = fmt.Sprintf("main is red at %s (%s), so the main-red stop is open, but its fix item %s is not in the inbox (%v); next: repair the inbox write, and the next red lane files the item again", s.verdict.Main, keys, fixItem, fileErr)
	}
	return e
}
