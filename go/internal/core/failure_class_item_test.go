package core

import (
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

var failureClassReplayRoot = flag.String("failure-class-replay-root", "", "state root that receives the replayed failure-class inbox")

type filedClassItem struct {
	Path       string
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Acceptance []string `json:"acceptance"`
	Recurrence int      `json:"recurrence"`
}

func applyClassBoundary(t *testing.T, root string, cycle int) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	_, err := recurrence.ApplyBoundary(recurrence.ApplyOptions{
		InboxDir:        filepath.Join(evolveDir, "inbox"),
		EscalationsPath: dispositionrouter.PendingActionsPath(filepath.Join(evolveDir, "escalations")),
		Cycle:           cycle,
		Now:             classClock().UTC(),
	})
	if err != nil {
		t.Fatalf("ApplyBoundary at cycle %d: %v", cycle, err)
	}
}

func filedClassItems(t *testing.T, root string) []filedClassItem {
	t.Helper()
	var items []filedClassItem
	err := filepath.WalkDir(filepath.Join(root, ".evolve", "inbox"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		item := filedClassItem{Path: path}
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		t.Fatalf("read the filed items: %v", err)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items
}

func itemsWithID(items []filedClassItem, id string) []filedClassItem {
	var out []filedClassItem
	for _, it := range items {
		if it.ID == id {
			out = append(out, it)
		}
	}
	return out
}

func requireActionableClassItem(t *testing.T, it filedClassItem, occurrences []FailureDigest) {
	t.Helper()
	if strings.TrimSpace(it.Title) == "" {
		t.Errorf("item %s has no title", it.ID)
	}
	lines := strings.Split(it.Summary, "\n")
	for _, d := range occurrences {
		phase, _, _ := strings.Cut(d.Fingerprint, "|")
		if !summaryLineNames(lines, strconv.Itoa(d.Cycle), phase, d.Checks) {
			t.Errorf("item %s summary has no line naming cycle %d, phase %q and checks %v:\n%s", it.ID, d.Cycle, phase, d.Checks, it.Summary)
		}
	}
	class := occurrences[len(occurrences)-1].ClassFingerprint
	named := false
	for _, a := range it.Acceptance {
		named = named || strings.Contains(a, class)
	}
	if !named {
		t.Errorf("item %s acceptance %q does not name the class %q that must stop recurring", it.ID, it.Acceptance, class)
	}
}

func summaryLineNames(lines []string, cycle, phase string, checks []string) bool {
	for _, line := range lines {
		ok := strings.Contains(line, cycle) && strings.Contains(line, phase)
		for _, c := range checks {
			ok = ok && strings.Contains(line, c)
		}
		if ok {
			return true
		}
	}
	return false
}

func TestFailureClassItem_TheConsoleItemNamesEachOccurrenceAndTheClassToStop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var occurrences []FailureDigest
	for _, c := range []int{2043, 2046, 2055} {
		occurrences = append(occurrences, sealClassCycle(t, root, classCycle{c, "pipeline-code"}))
	}
	third := occurrences[2]
	if third.ClassRoute != dispositionrouter.RouteConsole || len(third.Checks) == 0 {
		t.Fatalf("fixture drifted: third occurrence route %q checks %v; want console with named checks", third.ClassRoute, third.Checks)
	}

	applyClassBoundary(t, root, 2055)

	filed := itemsWithID(filedClassItems(t, root), classItemID(third.ClassFingerprint))
	if len(filed) != 1 {
		t.Fatalf("filed %d items for the class, want 1", len(filed))
	}
	requireActionableClassItem(t, filed[0], occurrences)
}

func TestFailureClassItem_AFurtherOccurrenceUpdatesTheSameItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var occurrences []FailureDigest
	for _, c := range []int{2043, 2046, 2055} {
		occurrences = append(occurrences, sealClassCycle(t, root, classCycle{c, "pipeline-code"}))
	}
	id := classItemID(occurrences[2].ClassFingerprint)
	applyClassBoundary(t, root, 2055)
	before := itemsWithID(filedClassItems(t, root), id)
	if len(before) != 1 || before[0].Recurrence != 3 {
		t.Fatalf("after the third occurrence: items %+v, want one with recurrence 3", before)
	}

	fourth := sealClassCycle(t, root, classCycle{2060, "pipeline-code"})
	if fourth.ClassFingerprint != occurrences[2].ClassFingerprint || fourth.ClassRoute != dispositionrouter.RouteConsole {
		t.Fatalf("fixture drifted: cycle 2060 class %q route %q", fourth.ClassFingerprint, fourth.ClassRoute)
	}
	applyClassBoundary(t, root, 2060)

	after := itemsWithID(filedClassItems(t, root), id)
	if len(after) != 1 {
		t.Fatalf("a further occurrence left %d items for the class, want the same 1", len(after))
	}
	if after[0].Path != before[0].Path {
		t.Errorf("the item moved from %s to %s; want the same file updated", before[0].Path, after[0].Path)
	}
	if after[0].Recurrence != 4 {
		t.Errorf("recurrence = %d after the fourth occurrence, want 4", after[0].Recurrence)
	}
	requireActionableClassItem(t, after[0], append(occurrences, fourth))
}

type replayClass struct {
	phase, preClass string
	occurrences     int
}

var replayClasses = []replayClass{
	{"tdd", "infra-error", 1}, {"ship", "infra-error", 1}, {"scout", "infra-error", 1}, {"build", "infra-error", 1},
	{"tdd", "gate-block", 2}, {"audit", "infra-error", 1}, {"build", "gate-block", 1}, {"triage", "gate-block", 1}, {"ship", "gate-block", 1},
}

var replayReasons = map[string]string{
	"infra-error": "bridge quota exhausted (85); infra teardown mid-phase",
	"gate-block":  "EGPS floor blocked ship: red_count=1 (egps-v11)",
}

func sealReplayCycle(t *testing.T, root string, cycle int, c replayClass) FailureDigest {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, ws, "audit-fail-reason.json", map[string]any{"schema_version": 1, "phase": c.phase, "reasons": []string{replayReasons[c.preClass]}})
	digest, err := AssembleFailureDigest(cycle, ws, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest(%d): %v", cycle, err)
	}
	d := validDisposition()
	d["cycle"], d["fingerprint"], d["recurrence"], d["routing"] = cycle, digest.Fingerprint, digest.Recurrence, "console"
	d["root_cause"] = map[string]any{"layer": "infra", "summary": "the " + c.phase + " phase failed on " + c.preClass}
	writeJSON(t, ws, "disposition.json", d)
	if err := (&Orchestrator{now: classClock}).finalizeRetroCompletion(ws); err != nil {
		t.Fatalf("finalizeRetroCompletion(%d): %v", cycle, err)
	}
	return readClassRecord(t, root, cycle)
}

func TestFailureClassItem_TheNineClassReplay(t *testing.T) {
	t.Parallel()
	root := *failureClassReplayRoot
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	byItem := map[string][]FailureDigest{}
	cycle := 3000
	for _, c := range replayClasses {
		for i := 0; i < c.occurrences; i++ {
			cycle++
			d := sealReplayCycle(t, root, cycle, c)
			id := classItemID(d.ClassFingerprint)
			byItem[id] = append(byItem[id], d)
			applyClassBoundary(t, root, cycle)
		}
	}
	if len(byItem) != len(replayClasses) {
		t.Fatalf("the replay made %d classes, want %d", len(byItem), len(replayClasses))
	}

	all := filedClassItems(t, root)
	if len(all) != len(replayClasses) {
		t.Fatalf("the replay filed %d items, want %d (one per class)", len(all), len(replayClasses))
	}
	for id, occurrences := range byItem {
		filed := itemsWithID(all, id)
		if len(filed) != 1 {
			t.Errorf("class %s filed %d items, want 1", id, len(filed))
			continue
		}
		requireActionableClassItem(t, filed[0], occurrences)
	}
}
