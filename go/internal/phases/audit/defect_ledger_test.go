package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	ledgerFile      = "defect-ledger.json"
	dispositionFile = "defect-dispositions.json"
)

type ledgerDoc struct {
	OriginCycle int `json:"origin_cycle"`
	Entries     []struct {
		ID       string `json:"id"`
		Text     string `json:"text"`
		Status   string `json:"status"`
		Evidence string `json:"evidence"`
		Reason   string `json:"reason"`
	} `json:"entries"`
}

func failingReportWithDefects(defects ...string) string {
	q, _ := json.Marshal(defects)
	return "# Audit Report\n\n## Verdict\n**FAIL**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":1,` +
		`"failure":{"class":"deliverable-rejected","defects":` + string(q) + `}} -->` + "\n"
}

func readLedger(t *testing.T, dir string) ledgerDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ledgerFile))
	if err != nil {
		ents, _ := os.ReadDir(dir)
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("read %s: %v (dir holds: %v)", ledgerFile, err, names)
	}
	var doc ledgerDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", ledgerFile, err)
	}
	return doc
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func diagsText(diags []core.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.Severity)
		b.WriteString(": ")
		b.WriteString(d.Message)
		b.WriteString("\n")
	}
	return b.String()
}

func TestClassify_RejectingAuditEmitsDefectLedger(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	defects := []string{
		"stale cs.ActiveWorktree survives fleet teardown",
		"symlinked test-suffix bypasses probe quarantine",
		"DirectImporters parses unbounded input",
	}
	verdict, _, _ := hooks{}.Classify(
		failingReportWithDefects(defects...),
		core.PhaseRequest{Cycle: 1255, Workspace: ws, ProjectRoot: t.TempDir()},
		core.BridgeResponse{},
	)
	if verdict != core.VerdictFAIL {
		t.Fatalf("fixture must reject: verdict = %q, want FAIL", verdict)
	}

	doc := readLedger(t, ws)
	if doc.OriginCycle != 1255 {
		t.Errorf("origin_cycle = %d, want 1255 — the ledger must name the cycle that RAISED the defects, or a continuation cannot trace lineage", doc.OriginCycle)
	}
	if len(doc.Entries) != len(defects) {
		t.Fatalf("entries = %d, want %d (one per structured defect)", len(doc.Entries), len(defects))
	}
	seenIDs := map[string]bool{}
	for i, e := range doc.Entries {
		if e.Status != "OPEN" {
			t.Errorf("entry %d status = %q, want OPEN on first emission", i, e.Status)
		}
		if strings.TrimSpace(e.ID) == "" {
			t.Errorf("entry %d has empty id — an unaddressable defect is exactly what got renamed away in the 1255 chain", i)
		}
		if seenIDs[e.ID] {
			t.Errorf("entry %d reuses id %q — ids must be unique to be addressable", i, e.ID)
		}
		seenIDs[e.ID] = true
		if e.Text != defects[i] {
			t.Errorf("entry %d text = %q, want %q (verbatim — narrowing the text IS the laundering)", i, e.Text, defects[i])
		}
	}
}

func TestClassify_PassingAuditWritesNoLedger(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	verdict, _, _ := hooks{}.Classify(
		"# Audit Report\n\n## Verdict\n**PASS**\n\n"+
			`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`+"\n",
		core.PhaseRequest{Cycle: 1279, Workspace: ws, ProjectRoot: t.TempDir()},
		core.BridgeResponse{},
	)
	if verdict != core.VerdictPASS {
		t.Fatalf("clean fixture verdict = %q, want PASS", verdict)
	}
	if _, err := os.Stat(filepath.Join(ws, ledgerFile)); err == nil {
		t.Error("a PASS with no structured defects must not mint a defect ledger")
	}
}

func continuationFixture(t *testing.T, ancestorCycle, thisCycle int, openDefects []string) (string, core.PhaseRequest) {
	t.Helper()
	root := t.TempDir()
	ancestorWS := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(ancestorCycle))

	type entry struct {
		ID     string `json:"id"`
		Text   string `json:"text"`
		Status string `json:"status"`
	}
	entries := make([]entry, 0, len(openDefects))
	for i, d := range openDefects {
		entries = append(entries, entry{ID: "d" + strconv.Itoa(i+1), Text: d, Status: "OPEN"})
	}
	writeJSON(t, filepath.Join(ancestorWS, ledgerFile), map[string]any{
		"origin_cycle": ancestorCycle,
		"entries":      entries,
	})

	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	writeJSON(t, filepath.Join(ws, "continuation-manifest.json"), map[string]any{
		"cycle":         ancestorCycle,
		"branch":        "cycle-" + strconv.Itoa(ancestorCycle),
		"snapshot_sha":  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"base_sha":      "cafebabecafebabecafebabecafebabecafebabe",
		"findings_path": filepath.Join(ancestorWS, "audit-fail-reason.json"),
	})
	return ws, core.PhaseRequest{Cycle: thisCycle, Workspace: ws, ProjectRoot: root}
}

var laundered = []string{
	"stale cs.ActiveWorktree survives fleet teardown",
	"symlinked test-suffix bypasses probe quarantine",
	"ScratchCwd follows a symlink outside the worktree",
}

func TestClassify_ContinuationCannotPassWithUnaccountedDefect(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, laundered)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": "go/internal/core/fleet.go:120"},
			map[string]any{"id": "d2", "status": "DEFERRED", "reason": "out of lane scope; queued as retro-symlink-suffix"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Error("a continuation with an unaccounted ancestor defect must not PASS — this is the exact false 'verified closed' the 1272 bookkeeping ship sealed")
	}
	text := diagsText(diags)
	if !strings.Contains(text, "d3") {
		t.Errorf("the unaccounted defect must be named by id in a diagnostic; diagnostics were:\n%s", text)
	}
}

func TestClassify_ContinuationLedgerRetainsEveryEntry(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, laundered)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")},
			map[string]any{"id": "d2", "status": "DEFERRED", "reason": "queued as retro-symlink-suffix"},
			map[string]any{"id": "d3", "status": "FIXED", "evidence": evidenceFile(t, req.ProjectRoot, "go/internal/scratch/cwd.go")},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Errorf("a fully-dispositioned continuation must retain PASS, got %q; diagnostics:\n%s", verdict, diagsText(diags))
	}

	doc := readLedger(t, ws)
	if len(doc.Entries) != len(laundered) {
		t.Fatalf("written-back ledger has %d entries, want %d — entries are never deleted, only transitioned", len(doc.Entries), len(laundered))
	}
	want := map[string]string{"d1": "FIXED", "d2": "DEFERRED", "d3": "FIXED"}
	for _, e := range doc.Entries {
		if w, ok := want[e.ID]; !ok {
			t.Errorf("unexpected ledger entry id %q", e.ID)
		} else if e.Status != w {
			t.Errorf("entry %s status = %q, want %q", e.ID, e.Status, w)
		}
		if e.Status == "FIXED" && strings.TrimSpace(e.Evidence) == "" {
			t.Errorf("entry %s claims FIXED with no evidence — an unevidenced closure claim is the laundering primitive", e.ID)
		}
		if e.Status == "DEFERRED" && strings.TrimSpace(e.Reason) == "" {
			t.Errorf("entry %s claims DEFERRED with no reason", e.ID)
		}
	}
}

func TestClassify_ContinuationWithNoDispositionArtifactCannotPass(t *testing.T) {
	_, req := continuationFixture(t, 1255, 1270, laundered)

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Errorf("a continuation that emitted NO disposition artifact must not PASS; diagnostics:\n%s", diagsText(diags))
	}
}

func TestClassify_NonContinuationPassPathUnchanged(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	verdict, diags, _ := hooks{}.Classify(
		narrativeReport("PASS"),
		core.PhaseRequest{Cycle: 1279, Workspace: ws, ProjectRoot: t.TempDir()},
		core.BridgeResponse{},
	)
	if verdict != core.VerdictPASS {
		t.Errorf("non-continuation green cycle verdict = %q, want PASS; diagnostics:\n%s", verdict, diagsText(diags))
	}
}
