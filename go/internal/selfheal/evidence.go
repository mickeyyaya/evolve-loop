package selfheal

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	EvidenceIncident      = "incident"
	EvidenceSignal        = "signal"
	EvidenceDigest        = "digest"
	EvidenceRetrospective = "retrospective"

	kindIncidentSignals  = "incident-signals"
	kindCuratorProposals = "inbox-curator-proposal"
	signalsFile          = "incident-signals.json"
	curatorProposalsFile = "curator-proposals.json"
	evidenceDirPrefix    = "selfheal-"
	cycleSourcePrefix    = "cycle:"
	ciSourcePrefix       = "ci:"
)

var evidencePrefixes = map[string]string{EvidenceIncident: "i", EvidenceSignal: "s", EvidenceDigest: "d", EvidenceRetrospective: "r"}

var cycleRecords = []struct{ name, class string }{
	{"failure-digest.json", EvidenceDigest},
	{"retrospective-report.md", EvidenceRetrospective},
}

type evidenceSource struct {
	path, class, kind, why string
	cycle                  int
}

func EvidenceWorkspace(evolveDir, incidentID string) string {
	return filepath.Join(evolveDir, "runs", evidenceDirPrefix+incidentID)
}

func WriteEvidence(root string, inc Incident, signals []signalcenter.Event) (inputindex.Index, error) {
	return writeEvidenceAt(root, EvidenceWorkspace(paths.EvolveDirOf(root), inc.ID), inc, signals)
}

func EntryIDs(idx inputindex.Index) []string {
	out := make([]string, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		out = append(out, e.ID)
	}
	return out
}

func writeEvidenceAt(root, ws string, inc Incident, signals []signalcenter.Event) (inputindex.Index, error) {
	if err := os.RemoveAll(ws); err != nil {
		return inputindex.Index{}, fmt.Errorf("evidence of %s: %w", inc.ID, err)
	}
	sources, err := evidenceSources(root, ws, inc, signals)
	if err != nil {
		return inputindex.Index{}, err
	}
	idx := inputindex.Index{Schema: inputindex.Schema, Phase: phasecontract.FlowSeatSelfhealOrchestrator, ItemIDs: []string{}, Lineage: []int{},
		Required: []string{}, Entries: []inputindex.Entry{}, Notes: evidenceNotes(inc)}
	counts := map[string]int{}
	for _, s := range sources {
		e, ok, err := inputindex.FileEntry(root, s.path, s.class, s.kind, s.why)
		if err != nil {
			return inputindex.Index{}, err
		}
		if !ok {
			continue
		}
		counts[s.class]++
		e.ID, e.Cycle, e.Required = evidencePrefixes[s.class]+strconv.Itoa(counts[s.class]), s.cycle, s.class == EvidenceIncident
		if e.Required {
			idx.Required = append(idx.Required, e.ID)
		}
		idx.Entries = append(idx.Entries, e)
	}
	return idx, inputindex.Write(ws, idx)
}

func evidenceSources(root, ws string, inc Incident, signals []signalcenter.Event) ([]evidenceSource, error) {
	out := []evidenceSource{{path: filepath.Join(Dir(paths.EvolveDirOf(root)), inc.ID+".json"), class: EvidenceIncident, kind: EvidenceIncident, why: "the incident record that the detector wrote"}}
	cuts := []struct {
		name, kind, why string
		events          []signalcenter.Event
	}{
		{signalsFile, kindIncidentSignals, "the signals that opened the incident", sourceSignals(inc, signals)},
		{curatorProposalsFile, kindCuratorProposals, "the inbox curator proposals that name the subject of the incident", curatorProposals(inc, signals)},
	}
	for _, c := range cuts {
		if len(c.events) == 0 {
			continue
		}
		path, err := writeCut(ws, c.name, c.events)
		if err != nil {
			return nil, err
		}
		out = append(out, evidenceSource{path: path, class: EvidenceSignal, kind: c.kind, why: c.why})
	}
	return append(out, cycleSources(root, inc)...), nil
}

func sourceSignals(inc Incident, signals []signalcenter.Event) []signalcenter.Event {
	return slices.DeleteFunc(slices.Clone(signals), func(e signalcenter.Event) bool {
		return !slices.Contains(inc.Sources, signalEvidenceID(e))
	})
}

func curatorProposals(inc Incident, signals []signalcenter.Event) []signalcenter.Event {
	return slices.DeleteFunc(slices.Clone(signals), func(e signalcenter.Event) bool {
		return e.Code != inboxcurator.CodeProposal || (e.Fields["id"] != inc.Subject && e.Fields["target"] != inc.Subject)
	})
}

func cycleSources(root string, inc Incident) []evidenceSource {
	var out []evidenceSource
	for _, src := range inc.Sources {
		n, ok := strings.CutPrefix(src, cycleSourcePrefix)
		cycle, err := strconv.Atoi(n)
		if !ok || err != nil {
			continue
		}
		for _, r := range cycleRecords {
			path := filepath.Join(paths.EvolveDirOf(root), "runs", "cycle-"+n, r.name)
			out = append(out, evidenceSource{path: path, class: r.class, kind: strings.TrimSuffix(r.name, filepath.Ext(r.name)), why: "a record of cycle " + n + ", a source of the incident", cycle: cycle})
		}
	}
	return out
}

func writeCut(ws, name string, events []signalcenter.Event) (string, error) {
	path := filepath.Join(ws, name)
	if err := atomicwrite.JSON(path, events); err != nil {
		return "", fmt.Errorf("evidence cut: %w", err)
	}
	return path, nil
}

func evidenceNotes(inc Incident) []string {
	var runs []string
	for _, src := range inc.Sources {
		if id, ok := strings.CutPrefix(src, ciSourcePrefix); ok {
			runs = append(runs, id)
		}
	}
	var notes []string
	if len(runs) > 0 {
		notes = append(notes, "no ci-log entry: the pipeline keeps no cut of a CI log yet; the incident names the CI runs "+strings.Join(runs, ", "))
	}
	return append(notes, "no diff entry: the pipeline does not cut a diff for an incident yet")
}
