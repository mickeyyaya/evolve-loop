package selfheal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	liveIncidents  = "testdata/incidents-2026-10-27"
	stallIncident  = "inc-2026-10-27-56437896"
	churnIncident  = "inc-2026-10-27-408110e5"
	ciHangIncident = "inc-2026-10-27-235078cd"
	churnItem      = "retro-minting-files-through-inbox-add-lane"
)

func liveRecords(t *testing.T) []Incident {
	t.Helper()
	incidents, err := LoadIncidents(liveIncidents)
	if err != nil || len(incidents) != 8 {
		t.Fatalf("the live pass of #1186 has 8 records: %d %v", len(incidents), err)
	}
	return incidents
}

func liveIncident(t *testing.T, id string) Incident {
	t.Helper()
	for _, inc := range liveRecords(t) {
		if inc.ID == id {
			return inc
		}
	}
	t.Fatalf("no live record %s", id)
	return Incident{}
}

func evidenceRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, inc := range liveRecords(t) {
		if err := WriteIncident(Dir(filepath.Join(root, ".evolve")), inc); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func signalAt(pid int, seq uint64, code signalcenter.Code, fields map[string]string) signalcenter.Event {
	return signalcenter.Event{PID: pid, Seq: seq, TS: t0.Format(time.RFC3339Nano), Code: code, Fields: fields}
}

func writeRunFile(t *testing.T, root string, cycle, name, body string) {
	t.Helper()
	path := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsonLines(t *testing.T, path string) []signalcenter.Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []signalcenter.Event
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func entryOf(idx inputindex.Index, kind string) (inputindex.Entry, bool) {
	i := slices.IndexFunc(idx.Entries, func(e inputindex.Entry) bool { return e.Kind == kind })
	if i < 0 {
		return inputindex.Entry{}, false
	}
	return idx.Entries[i], true
}

func TestWriteEvidence_TheStallIndexListsTheRecordAndACutOfItsOwnSignals(t *testing.T) {
	t.Parallel()
	root := evidenceRoot(t)
	inc := liveIncident(t, stallIncident)
	signals := []signalcenter.Event{
		signalAt(93944, 372, "LIVENESS_PHASE_STALLED", nil),
		signalAt(93944, 376, "LIVENESS_PHASE_STALLED", nil),
		signalAt(93944, 999, "LIVENESS_PHASE_STALLED", nil),
	}

	idx, err := WriteEvidence(root, inc, signals)
	if err != nil {
		t.Fatal(err)
	}
	ws := EvidenceWorkspace(filepath.Join(root, ".evolve"), inc.ID)
	if read, err := inputindex.Read(ws); err != nil || len(read.Entries) != len(idx.Entries) {
		t.Fatalf("the index on disk: %v %+v", err, read)
	}
	record, ok := entryOf(idx, EvidenceIncident)
	if !ok || record.ID != "i1" || !record.Required || record.Path != ".evolve/incidents/"+inc.ID+".json" || !slices.Equal(idx.Required, []string{"i1"}) {
		t.Fatalf("the incident record must be the required entry i1: %+v", idx)
	}
	cut, ok := entryOf(idx, kindIncidentSignals)
	if !ok || cut.ID != "s1" || cut.Class != EvidenceSignal {
		t.Fatalf("no signal cut: %+v", idx.Entries)
	}
	got := jsonLines(t, filepath.Join(root, filepath.FromSlash(cut.Path)))
	if len(got) != 2 || got[0].Seq != 372 || got[1].Seq != 376 {
		t.Fatalf("the cut holds only the sources of the incident, got %+v", got)
	}
	if _, body, err := idx.Open(root, "s1"); err != nil || !strings.Contains(string(body), "LIVENESS_PHASE_STALLED") {
		t.Fatalf("evolve context open s1 must read the cut: %v", err)
	}
	if idx.Schema != inputindex.Schema || idx.Phase != "selfheal-orchestrator" || len(idx.Notes) != 1 || !strings.Contains(idx.Notes[0], "diff") {
		t.Errorf("index header %+v", idx)
	}
}

func TestWriteEvidence_AnItemIncidentGetsItsCuratorProposalsAndTheRecordsOfItsCycles(t *testing.T) {
	t.Parallel()
	root := evidenceRoot(t)
	inc := liveIncident(t, churnIncident)
	writeRunFile(t, root, "1983", "failure-digest.json", `{"summary":"the retro lane minted files through inbox add"}`)
	writeRunFile(t, root, "1983", "retrospective-report.md", "# The lane re-minted its own item\n")
	signals := []signalcenter.Event{
		signalAt(5, 1, inboxcurator.CodeProposal, map[string]string{"kind": "merge", "id": churnItem, "target": "other-item"}),
		signalAt(5, 2, inboxcurator.CodeProposal, map[string]string{"kind": "reroute", "id": "x", "target": churnItem}),
		signalAt(5, 3, inboxcurator.CodeProposal, map[string]string{"kind": "consume", "id": "unrelated-item"}),
	}

	idx, err := WriteEvidence(root, inc, signals)
	if err != nil {
		t.Fatal(err)
	}
	proposals, ok := entryOf(idx, kindCuratorProposals)
	if !ok || proposals.Class != EvidenceSignal {
		t.Fatalf("no curator proposals entry: %+v", idx.Entries)
	}
	if got := jsonLines(t, filepath.Join(root, filepath.FromSlash(proposals.Path))); len(got) != 2 || got[0].Seq != 1 || got[1].Seq != 2 {
		t.Fatalf("the proposals that name the item, got %+v", got)
	}
	digest, okD := entryOf(idx, "failure-digest")
	retro, okR := entryOf(idx, "retrospective-report")
	if !okD || digest.Class != EvidenceDigest || digest.ID != "d1" || digest.Cycle != 1983 || !okR || retro.Class != EvidenceRetrospective || retro.ID != "r1" {
		t.Fatalf("the records of the cycle sources: %+v", idx.Entries)
	}
	if _, ok := entryOf(idx, kindIncidentSignals); ok {
		t.Error("an incident with no signal source has no signal cut")
	}
}

func TestWriteEvidence_ACIIncidentNamesTheRunsItCannotCutAndAFreshPassDropsTheOldCut(t *testing.T) {
	t.Parallel()
	root := evidenceRoot(t)
	inc := liveIncident(t, ciHangIncident)
	ws := EvidenceWorkspace(filepath.Join(root, ".evolve"), inc.ID)
	stale := filepath.Join(ws, curatorProposalsFile)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := WriteEvidence(root, inc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Entries) != 1 || len(idx.Notes) != 2 || !strings.Contains(idx.Notes[0], "40265789886") || !strings.Contains(idx.Notes[0], "40269772364") {
		t.Fatalf("a CI incident lists its record and names its runs: %+v", idx)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the cut of an earlier pass must go: %v", err)
	}
	if got := EntryIDs(idx); !slices.Equal(got, []string{"i1"}) {
		t.Errorf("EntryIDs = %v", got)
	}
}

func TestWriteEvidence_Faults(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	t.Run("the runs directory is a file", func(t *testing.T) {
		t.Parallel()
		root := evidenceRoot(t)
		if err := os.WriteFile(filepath.Join(root, ".evolve", "runs"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := WriteEvidence(root, inc, nil); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("the incident record is a directory", func(t *testing.T) {
		t.Parallel()
		root := evidenceRoot(t)
		path := filepath.Join(Dir(filepath.Join(root, ".evolve")), inc.ID+".json")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := WriteEvidence(root, inc, nil); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("the cut file is a directory", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		if err := os.Mkdir(filepath.Join(ws, signalsFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := writeCut(ws, signalsFile, []signalcenter.Event{{}}); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a stale workspace that cannot be removed", func(t *testing.T) {
		t.Parallel()
		root := evidenceRoot(t)
		if _, err := writeEvidenceAt(root, filepath.Join(root, ".evolve", "incidents", inc.ID+".json", "ws"), inc, nil); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a signal cut that cannot be written", func(t *testing.T) {
		t.Parallel()
		root := evidenceRoot(t)
		dangling := filepath.Join(root, ".evolve", "dangling")
		if err := os.Symlink(filepath.Join(root, "nowhere"), dangling); err != nil {
			t.Fatal(err)
		}
		if _, err := writeEvidenceAt(root, filepath.Join(dangling, "ws"), inc, []signalcenter.Event{signalAt(93944, 372, "LIVENESS_PHASE_STALLED", nil)}); err == nil {
			t.Fatal("a cut under a dangling link must fail")
		}
	})
	t.Run("a cycle source that is not a number", func(t *testing.T) {
		t.Parallel()
		root := evidenceRoot(t)
		odd := inc
		odd.Sources = []string{"cycle:x"}
		if got := cycleSources(root, odd); len(got) != 0 {
			t.Fatalf("a malformed cycle source has no records: %+v", got)
		}
	})
	t.Run("an index that cannot be written", func(t *testing.T) {
		t.Parallel()
		root := evidenceRoot(t)
		if err := os.WriteFile(filepath.Join(root, ".evolve", "runs-blocker"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := writeEvidenceAt(root, filepath.Join(root, ".evolve", "runs-blocker", "ws"), inc, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}
