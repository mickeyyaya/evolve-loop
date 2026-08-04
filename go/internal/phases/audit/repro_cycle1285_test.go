package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// repro_cycle1285_test.go — executable reproduction of the cycle-1285
// adversarial review's findings that live in this package (F2 HIGH, F3 MEDIUM,
// F5 MEDIUM). Every case drives the production seam `hooks.Classify`, never a
// helper: the committed suites are green precisely because none of them
// constructs these inputs.
//
// Shared shape of the two F2 cases: the whole reconcile gate's ARMING condition
// is `continuation.ReadManifest(req.Workspace)` — a plain 0644 file in the
// directory the graded phase agent writes. The gate is well defended against a
// forged disposition and undefended against never being asked to run.
//
// These tests are RED on the tree that produced them. They are not a style
// preference: each asserts the posture defect_ledger.go states for its OTHER
// inputs. A deleted ancestor ledger is already a loud warning because "one `rm`
// outside the workspace would otherwise disarm the whole gate in silence"
// (defect_ledger.go:336-344), and an unparseable defect-dispositions.json
// already BLOCKS because "degrading open there would hand the gate its cheapest
// bypass" (:458-460). The manifest — the input that decides whether the gate
// runs at all, and the one the agent writes most directly — gets neither.

// reproContinuationFixture is continuationFixture with two changes that matter
// to the arming question:
//
//  1. the current workspace sits at its REAL production location,
//     <root>/.evolve/runs/cycle-<n>, rather than an unrelated temp dir; and
//  2. the same lineage is additionally witnessed OUT OF BAND, in the root-owned
//     continuation registry (continuation.RegistryPath —
//     .evolve/continuation-registry.json, the scope-id-keyed map ADR-0076 slice
//     C G2 already writes at the preserve decision). That file is outside every
//     per-cycle workspace, so it still names this lane's ancestor after the
//     workspace manifest is deleted.
//
// The registry is what makes these cases a DEFECT rather than an over-strict
// test: "this cycle is a continuation of cycle-N" remains knowable from a
// non-workspace source, so a silent no-op is a choice the code makes, not a
// limit the environment imposes.
// reproScopeID is the lane's pinned todo id. It is the registry KEY and the
// lane-scope.json entry alike — one identity, two records, which is precisely
// the property the F2 fix depends on.
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

	// cycle-1287, adopting this reproducer into the tree: the fixture gained
	// lane-scope.json, which it did not carry when it was written in the
	// cycle-1285 workspace. This is an ADDITION of a production artifact, not a
	// relaxation — core.materializeLaneScope pins this file into every run
	// workspace BEFORE any phase runs, so a real graded audit always sees it,
	// and the registry is keyed by exactly these todo ids. Without it the
	// fixture asked the gate to recover a lineage from a root-owned map with no
	// key to look under; arming on ANY registry entry instead would block every
	// ordinary cycle in a project where any lane had ever preserved work, which
	// is a worse defect than the one being fixed. Every assertion below is
	// unchanged from the cycle-1285 original.
	writeJSON(t, filepath.Join(ws, core.LaneScopeFile), map[string]any{
		"todo_ids": []string{reproScopeID}, "goal_hash": "goal",
	})

	return ws, core.PhaseRequest{Cycle: thisCycle, Workspace: ws, ProjectRoot: root}
}

// -- F2 (HIGH): the arming condition is agent-writable ---------------------

// TestRepro1285_F2_ManifestDeletionSilentlyDisarmsTheReconcileGate — `rm` on one
// 0644 file in the graded agent's own workspace retires every inherited OPEN
// defect. reconcileContinuationDefects reads the manifest, gets
// (zero, false, nil), and returns (nil, false): PASS, and not one diagnostic.
//
// This is the same trust class as cycle-1282's DEF-1, which that audit graded
// CRITICAL — and strictly cheaper, because DEF-1 needed a well-formed planted
// ledger while this needs a delete. The root-owned registry written by the
// fixture still names cycle-1255 as this lane's ancestor, so the disarm is
// detectable; the code simply does not look.
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

// TestRepro1285_F2_CorruptManifestDegradesOpen — one byte of garbage in the same
// file yields a single `warning` diagnostic and NO block, so the gate is
// disarmed by corruption as well as by deletion.
//
// The asymmetry is internal to this file, not imposed from outside:
// readDispositions blocks on an unparseable defect-dispositions.json because
// "degrading open there would hand the gate its cheapest bypass"
// (defect_ledger.go:458-460). Writing garbage into the manifest is cheaper still
// and buys more — it retires every inherited defect rather than one claim.
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

// -- F3 (MEDIUM): rule 4 is case-sensitive, the filesystem is not ----------

// caseInsensitiveVolume probes dir rather than switching on runtime.GOOS: the
// property that matters is the volume's, and a darwin host can mount either
// kind. A case-SENSITIVE volume makes the citation below fail to resolve for the
// wrong reason, so the caller skips instead of banking a green-by-accident.
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

// TestRepro1285_F3_CaseVariantSelfCitationClosesInheritedDefects — evidenceResolves
// rejects self-vouching citations with an exact-string `switch filepath.Base(clean)`
// (defect_ledger.go:275-278) and then resolves the path with os.Lstat. On a
// case-insensitive volume — the stated platform, darwin/APFS — those two
// disagree: the switch misses "Defect-Ledger.json" and Lstat resolves it anyway,
// so the gate's OWN record closes every inherited defect.
//
// The committed lock TestAdversarial_UnrelatedExistingFileDoesNotCloseADefect
// tests the exact-case spellings only and passes straight over this.
func TestRepro1285_F3_CaseVariantSelfCitationClosesInheritedDefects(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if !caseInsensitiveVolume(t, ws) {
		t.Skip("case-sensitive volume: the citation would fail to resolve for the wrong reason, which is a green-by-accident rather than a pass")
	}

	// The ancestor's ledger genuinely exists at this path; only its spelling is
	// varied, so nothing but the case comparison stands between the claim and
	// the mechanism's own bookkeeping.
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

// -- F5 (MEDIUM): the closure gate FAILs honest reporting ------------------

// TestRepro1285_F5_QuotedDefectTextIsNotAClosureClaim — closureClaimOffenders
// substring-matches "verified closed" per line with no notion of quoting or
// negation. The canonical inherited defect text in this repo literally contains
// the phrase (docs/operations/batch-integrity-review-2026-08-04.md: "the 1255-D1
// stale-worktree CRITICAL narrowed to 'verified closed'"), so an auditor who
// correctly reports that defect as STILL OPEN is blocked for accuracy.
//
// The second-order damage is worse than the availability hit: the cheapest way
// out is to append the literal string "defect-dispositions.json" to the line,
// which satisfies the gate and adds no evidence at all. A gate whose remedy is a
// one-token appeasement becomes noise, then gets disabled.
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
