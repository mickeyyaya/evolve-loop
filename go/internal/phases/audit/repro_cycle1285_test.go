package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const reproScopeID = "continuation-defect-ledger"

func reproContinuationFixture(t *testing.T, ancestorCycle, thisCycle int, openDefects []string) (string, core.PhaseRequest) {
	t.Helper()
	root := t.TempDir()
	ancestorWS := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(ancestorCycle))

	entries := make([]any, 0, len(openDefects))
	for i, d := range openDefects {
		entries = append(entries, map[string]any{
			"id": "d" + strconv.Itoa(i+1), "text": d, "status": "OPEN",
		})
	}
	writeJSON(t, filepath.Join(ancestorWS, ledgerFile), map[string]any{
		"origin_cycle": ancestorCycle,
		"entries":      entries,
	})

	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(thisCycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	binding := map[string]any{
		"cycle":         ancestorCycle,
		"branch":        "cycle-" + strconv.Itoa(ancestorCycle),
		"snapshot_sha":  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"base_sha":      "cafebabecafebabecafebabecafebabecafebabe",
		"findings_path": filepath.Join(ancestorWS, "audit-fail-reason.json"),
	}
	writeJSON(t, filepath.Join(ws, "continuation-manifest.json"), binding)
	writeJSON(t, continuation.RegistryPath(root), map[string]any{reproScopeID: binding})

	writeJSON(t, filepath.Join(ws, core.LaneScopeFile), map[string]any{
		"todo_ids": []string{reproScopeID}, "goal_hash": "goal",
	})

	return ws, core.PhaseRequest{Cycle: thisCycle, Workspace: ws, ProjectRoot: root}
}

func TestRepro1285_F2_ManifestDeletionSilentlyDisarmsTheReconcileGate(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if err := os.Remove(filepath.Join(ws, "continuation-manifest.json")); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}

	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Errorf("deleting the workspace continuation manifest closed %d inherited OPEN defects with no disposition artifact; verdict = PASS.\n"+
			"The lineage is still recorded out of band at %s — arming must not depend solely on a file the graded agent may delete.\ndiagnostics:\n%s",
			len(laundered), continuation.RegistryPath(req.ProjectRoot), diagsText(diags))
	}
	if len(diags) == 0 {
		t.Errorf("the disarm produced ZERO diagnostics — a deleted ancestor ledger is already recorded as a loud warning for exactly this reason (defect_ledger.go:336-344); the manifest is the cheaper `rm` and is silent")
	}
}

func TestRepro1285_F2_CorruptManifestDegradesOpen(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if err := os.WriteFile(filepath.Join(ws, "continuation-manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("corrupt manifest: %v", err)
	}

	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Errorf("a malformed continuation manifest degraded OPEN: %d inherited defects went unenforced and the cycle PASSed.\n"+
			"An unparseable defect-dispositions.json blocks on this same argument; the manifest decides whether the gate runs at all.\ndiagnostics:\n%s",
			len(laundered), diagsText(diags))
	}
}

func caseInsensitiveVolume(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "CaseProbe.tmp")
	if err := os.WriteFile(probe, []byte("probe\n"), 0o644); err != nil {
		t.Fatalf("write case probe: %v", err)
	}
	defer os.Remove(probe)
	_, err := os.Lstat(filepath.Join(dir, "caseprobe.tmp"))
	return err == nil
}

func TestRepro1285_F3_CaseVariantSelfCitationClosesInheritedDefects(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if !caseInsensitiveVolume(t, ws) {
		t.Skip("case-sensitive volume: the citation would fail to resolve for the wrong reason, which is a green-by-accident rather than a pass")
	}

	cited := filepath.Join(".evolve", "runs", "cycle-1255", "Defect-Ledger.json")
	claims := make([]any, 0, len(laundered))
	for i := range laundered {
		claims = append(claims, map[string]any{
			"id": "d" + strconv.Itoa(i+1), "status": "FIXED", "evidence": cited,
		})
	}
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{"dispositions": claims})

	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Errorf("evidence %q closed all %d inherited defects — a case variant of the gate's own record is still the gate's own record.\ndiagnostics:\n%s",
			cited, len(laundered), diagsText(diags))
	}
}

func TestRepro1285_F5_QuotedDefectTextIsNotAClosureClaim(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	report := "# Audit Report\n\n## Findings\n\n" +
		"Inherited defect text: \"the 1255-D1 stale-worktree CRITICAL narrowed to 'verified closed'\" — still OPEN, not fixed.\n\n" +
		"## Verdict\n**PASS**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->` + "\n"

	verdict, diags, _ := hooks{}.Classify(
		report,
		core.PhaseRequest{Cycle: 1285, Workspace: ws, ProjectRoot: t.TempDir()},
		core.BridgeResponse{},
	)
	if verdict != core.VerdictPASS {
		t.Errorf("a report QUOTING an inherited defect and declaring it still OPEN was graded %q — the gate must match an assertion of closure, not the presence of the phrase.\ndiagnostics:\n%s",
			verdict, diagsText(diags))
	}
}
