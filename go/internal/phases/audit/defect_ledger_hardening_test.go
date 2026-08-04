package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// defect_ledger_hardening_test.go — RED contract for cycle-1282, the
// CONTINUATION of cycle-1279 (`continuation-defect-ledger`). The mechanism
// landed in 1279; its own audit then rejected it with seven defects (D1–D7,
// .evolve/runs/cycle-1279/audit-report.md). This file encodes D1, D2, D3, D4
// and D6 — the five that live in this package — as executable criteria.
//
// Every assertion reaches its subject through the REAL production seam
// (`hooks.Classify`, audit.go:311 reconcile / :394 emit), never by calling an
// unexported helper directly: a predicate that passes on a helper passes on
// dead code, which is the failure mode this whole cycle exists to close.
//
// Contract summary the builder inherits:
//
//	D1  reconcile MERGES the current workspace ledger with the ancestor's; an
//	    entry present after one Classify is present after the next. Ids are
//	    derived from defect CONTENT, never from a position counter, so a
//	    re-mint cannot bind an old id to new text.
//	D2  a manifest-named continuation whose ancestor left NO ledger is
//	    DIAGNOSED (the `rm` that silently disarmed the gate becomes visible).
//	D3  a FIXED claim's evidence must RESOLVE to a real file (under ProjectRoot
//	    or the workspace); `evidence:"x"` is an unaccounted defect, not a
//	    closure.
//	D4  every disposition switch arm plus the non-OPEN carry-forward is
//	    exercised by a table case (the headline rule had zero coverage).
//	D6  emit fires on FAIL *and* WARN — a WARN-shipped cycle carrying
//	    structured defects must not leave the next continuation nothing to
//	    inherit.

// evidenceFile materializes rel under root and returns the "path:line" form an
// auditor writes into a closure claim. D3 makes evidence a citation of a REAL
// artifact, so fixtures that expect a closure to be honored must produce one.
func evidenceFile(t *testing.T, root, rel string) string {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte("// evidence fixture\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
	return rel + ":12"
}

// ledgerByText indexes a written-back ledger by defect text — the identity that
// must survive a retry, independent of whatever id scheme the builder picks.
func ledgerByText(doc ledgerDoc) map[string]string {
	out := make(map[string]string, len(doc.Entries))
	for _, e := range doc.Entries {
		out[e.Text] = e.ID
	}
	return out
}

// -- D1: the ledger must never shrink -------------------------------------

// TestClassify_ContinuationRetryDoesNotEraseOwnEntries — D1, CRITICAL. On a
// second Classify call (an ordinary audit retry — no adversary required)
// reconcile rebuilds `merged` from ancestor.Entries alone and truncate-writes
// it, erasing the entries emit appended on the previous call. That is the state
// defect_ledger.go:34 itself declares forbidden: "Entries transition; they are
// never deleted. A ledger that shrinks is a ledger that launders."
func TestClassify_ContinuationRetryDoesNotEraseOwnEntries(t *testing.T) {
	inherited := []string{"stale cs.ActiveWorktree survives fleet teardown"}
	ws, req := continuationFixture(t, 1255, 1270, inherited)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")},
		},
	})

	own := "reconcile truncate-writes the ledger from ancestor entries only"

	// Attempt 1 rejects and records this cycle's OWN defect beside the
	// inherited one.
	hooks{}.Classify(failingReportWithDefects(own), req, core.BridgeResponse{})
	first := ledgerByText(readLedger(t, ws))
	if _, ok := first[own]; !ok {
		t.Fatalf("fixture precondition: this cycle's own defect was not recorded on the first Classify; ledger held %v", first)
	}
	if _, ok := first[inherited[0]]; !ok {
		t.Fatalf("fixture precondition: the inherited defect was not carried into the workspace ledger; ledger held %v", first)
	}

	// Attempt 2 is the ordinary retry that now grades clean. reconcile runs
	// (audit.go:311) and emit does not (nothing to emit on a PASS), so the
	// truncate-write is unmasked: this cycle's own recorded defect is erased
	// with no adversary at all, and the operator's record of what cycle-1270
	// itself got wrong is gone.
	hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	second := ledgerByText(readLedger(t, ws))

	for text, id := range first {
		gotID, ok := second[text]
		if !ok {
			t.Errorf("defect %q vanished from the ledger on the second Classify — a ledger that shrinks is a ledger that launders (defect_ledger.go:34)", text)
			continue
		}
		if gotID != id {
			t.Errorf("defect %q changed id %q → %q across a retry — a disposition keyed on the old id now closes something else", text, id, gotID)
		}
	}
}

// TestClassify_LedgerIDsAreContentDerived — D1, second half. Ids minted as
// "d"+len(entries)+1 (defect_ledger.go:148) are POSITIONAL: once any shrink or
// reordering happens, the same id string is re-minted for different defect
// text, and a disposition keyed on that id closes something other than what it
// claims. The id must be a function of the defect text alone.
func TestClassify_LedgerIDsAreContentDerived(t *testing.T) {
	root := t.TempDir()
	textA := "symlinked test-suffix bypasses probe quarantine"
	textB := "DirectImporters parses unbounded input"

	wsA := t.TempDir()
	yes := true
	writeACSVerdictShip(t, wsA, 0, &yes)
	hooks{}.Classify(failingReportWithDefects(textA, textB),
		core.PhaseRequest{Cycle: 1279, Workspace: wsA, ProjectRoot: root}, core.BridgeResponse{})
	idsA := ledgerByText(readLedger(t, wsA))

	// Same defects, opposite order, different cycle: a content-derived id is
	// unchanged; a positional one swaps.
	wsB := t.TempDir()
	writeACSVerdictShip(t, wsB, 0, &yes)
	hooks{}.Classify(failingReportWithDefects(textB, textA),
		core.PhaseRequest{Cycle: 1280, Workspace: wsB, ProjectRoot: root}, core.BridgeResponse{})
	idsB := ledgerByText(readLedger(t, wsB))

	for _, text := range []string{textA, textB} {
		if idsA[text] == "" || idsB[text] == "" {
			t.Fatalf("defect %q missing an id (A=%q B=%q)", text, idsA[text], idsB[text])
		}
		if idsA[text] != idsB[text] {
			t.Errorf("defect %q got id %q in one emission and %q in another — position-derived ids re-bind an id to different text across a chain; derive the id from the defect CONTENT", text, idsA[text], idsB[text])
		}
	}
}

// -- D2: a missing ancestor ledger must be visible ------------------------

// TestClassify_ContinuationWithNoAncestorLedgerIsDiagnosed — D2, HIGH. The
// ancestor ledger lives at <ProjectRoot>/.evolve/runs/cycle-N/, outside the
// workspace, and the role guard matches Edit|Write only — one Bash `rm`
// disarms the entire gate. Today that path returns (nil, false) with no
// diagnostic at all (defect_ledger.go:190-192), so the cycle PASSes with no
// record that reconciliation was ever skipped.
func TestClassify_ContinuationWithNoAncestorLedgerIsDiagnosed(t *testing.T) {
	_, req := continuationFixture(t, 1255, 1270, laundered)
	// The `rm`: the manifest still names cycle-1255 as the ancestor, but its
	// ledger is gone.
	if err := os.Remove(filepath.Join(req.ProjectRoot, ".evolve", "runs", "cycle-1255", ledgerFile)); err != nil {
		t.Fatalf("remove ancestor ledger: %v", err)
	}

	_, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	text := diagsText(diags)
	if !strings.Contains(text, "1255") {
		t.Errorf("a manifest-named continuation whose ancestor ledger is absent must be DIAGNOSED by ancestor cycle number — one `rm` currently disarms the gate in silence; diagnostics were:\n%s", text)
	}
}

// TestClassify_NonContinuationEmitsNoLedgerDiagnostic — the NEGATIVE half of
// D2. The diagnostic above must be reachable only from the continuation path;
// warning on every ordinary cycle would train operators to ignore it.
func TestClassify_NonContinuationEmitsNoLedgerDiagnostic(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"),
		core.PhaseRequest{Cycle: 1281, Workspace: ws, ProjectRoot: t.TempDir()}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("ordinary green cycle verdict = %q, want PASS", verdict)
	}
	if strings.Contains(strings.ToLower(diagsText(diags)), "defect ledger") {
		t.Errorf("a cycle with no continuation manifest must emit no defect-ledger diagnostic; diagnostics were:\n%s", diagsText(diags))
	}
}

// -- D3: evidence must be a resolvable citation ---------------------------

// TestClassify_UnresolvableEvidenceDoesNotCloseADefect — D3, HIGH. Closure
// claims are validated for non-emptiness after trim only
// (defect_ledger.go:210-213), so `evidence:"x"` transitions an inherited
// CRITICAL to FIXED with no diagnostic — the unverifiable closure claim F1
// indicts. Evidence must name a file that actually exists.
func TestClassify_UnresolvableEvidenceDoesNotCloseADefect(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, laundered[:1])
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": "x"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("evidence \"x\" must not close an inherited defect; diagnostics:\n%s", diagsText(diags))
	}
	if !strings.Contains(diagsText(diags), "d1") {
		t.Errorf("the unevidenced closure must be named by id; diagnostics:\n%s", diagsText(diags))
	}
	doc := readLedger(t, ws)
	for _, e := range doc.Entries {
		if e.ID == "d1" && e.Status == "FIXED" {
			t.Error("the ledger recorded d1 as FIXED on unresolvable evidence — the written-back artifact must not assert a closure the gate rejected")
		}
	}
}

// TestClassify_ResolvableEvidenceClosesADefect — the POSITIVE half of D3.
// Evidence that names a real file (in "path:line" form, and as a phase
// artifact) must still close the defect: a rule that rejected every claim would
// block every continuation forever.
//
// cycle-1282 DEF-2 narrowed the rule to PROJECT-ROOT resolution only. A phase
// artifact is still perfectly citable — by its path FROM the root, which is
// also what makes the citation followable by a reader who has only the repo.
// What is gone is the bare workspace-relative form, because the workspace is
// this cycle's own agent-authored ephemera and citing it is self-vouching.
func TestClassify_ResolvableEvidenceClosesADefect(t *testing.T) {
	_, req := continuationFixture(t, 1255, 1270, laundered[:2])
	ws := req.Workspace
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")},
			map[string]any{"id": "d2", "status": "FIXED", "evidence": evidenceFile(t, req.ProjectRoot, ".evolve/runs/cycle-1270/build-report.md")},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Errorf("closures citing REAL artifacts must be honored, got verdict %q; diagnostics:\n%s", verdict, diagsText(diags))
	}
}

// -- D4: every disposition arm is exercised -------------------------------

// TestClassify_DispositionArms — D4, HIGH. The coverage gate FAILed at 76.2%
// with three of the five disposition switch arms unexecuted, and those three
// ARE the acceptance criterion's headline rule. One table case per arm, each
// asserting the verdict AND that the gap is named by id.
func TestClassify_DispositionArms(t *testing.T) {
	// cycle-1282 DEF-2: closure evidence resolves under the PROJECT ROOT only.
	realEvidence := "go/internal/core/fleet.go" // materialized per-case under the root

	cases := []struct {
		name       string
		claim      map[string]any
		wantPass   bool
		wantNamed  string
		wantStatus string
	}{
		{
			name:       "no disposition at all",
			claim:      nil,
			wantPass:   false,
			wantNamed:  "d1",
			wantStatus: "OPEN",
		},
		{
			name:       "FIXED without evidence",
			claim:      map[string]any{"id": "d1", "status": "FIXED"},
			wantPass:   false,
			wantNamed:  "d1",
			wantStatus: "OPEN",
		},
		{
			name:       "DEFERRED without reason",
			claim:      map[string]any{"id": "d1", "status": "DEFERRED"},
			wantPass:   false,
			wantNamed:  "d1",
			wantStatus: "OPEN",
		},
		{
			name:       "status outside FIXED/DEFERRED",
			claim:      map[string]any{"id": "d1", "status": "WONTFIX", "reason": "disagree"},
			wantPass:   false,
			wantNamed:  "d1",
			wantStatus: "OPEN",
		},
		{
			name:       "FIXED with resolvable evidence",
			claim:      map[string]any{"id": "d1", "status": "FIXED", "evidence": realEvidence},
			wantPass:   true,
			wantStatus: "FIXED",
		},
		{
			name:       "DEFERRED with a reason",
			claim:      map[string]any{"id": "d1", "status": "DEFERRED", "reason": "out of lane scope; queued as retro-symlink-suffix"},
			wantPass:   true,
			wantStatus: "DEFERRED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, req := continuationFixture(t, 1255, 1270, laundered[:1])
			evidenceFile(t, req.ProjectRoot, realEvidence)
			if tc.claim != nil {
				writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
					"dispositions": []any{tc.claim},
				})
			}

			verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
			gotPass := verdict == core.VerdictPASS
			if gotPass != tc.wantPass {
				t.Errorf("verdict = %q (pass=%v), want pass=%v; diagnostics:\n%s", verdict, gotPass, tc.wantPass, diagsText(diags))
			}
			if tc.wantNamed != "" && !strings.Contains(diagsText(diags), tc.wantNamed) {
				t.Errorf("unaccounted defect %s must be named in a diagnostic; diagnostics:\n%s", tc.wantNamed, diagsText(diags))
			}
			for _, e := range readLedger(t, ws).Entries {
				if e.ID == "d1" && e.Status != tc.wantStatus {
					t.Errorf("written-back status for d1 = %q, want %q", e.Status, tc.wantStatus)
				}
			}
		})
	}
}

// TestClassify_CarriesForwardAlreadyDispositionedAncestorEntry — D4, second
// half: the multi-hop invariant (1255→1268→1270→1272) that is the literal
// shape being defended. An ancestor entry already FIXED/DEFERRED upstream needs
// no fresh claim, and must survive into this cycle's ledger verbatim —
// evidence and reason included, or the chain loses the proof of closure.
func TestClassify_CarriesForwardAlreadyDispositionedAncestorEntry(t *testing.T) {
	root := t.TempDir()
	ancestorWS := filepath.Join(root, ".evolve", "runs", "cycle-1255")
	writeJSON(t, filepath.Join(ancestorWS, ledgerFile), map[string]any{
		"origin_cycle": 1255,
		"entries": []any{
			map[string]any{"id": "d1", "text": laundered[0], "status": "FIXED", "evidence": "build-report.md"},
			map[string]any{"id": "d2", "text": laundered[1], "status": "DEFERRED", "reason": "queued as retro-symlink-suffix"},
		},
	})

	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	writeJSON(t, filepath.Join(ws, "continuation-manifest.json"), map[string]any{
		"cycle":        1255,
		"branch":       "cycle-1255",
		"snapshot_sha": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"base_sha":     "cafebabecafebabecafebabecafebabecafebabe",
	})
	req := core.PhaseRequest{Cycle: 1270, Workspace: ws, ProjectRoot: root}

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Errorf("an ancestor whose entries are ALL already dispositioned needs no fresh claim; verdict = %q, diagnostics:\n%s", verdict, diagsText(diags))
	}

	doc := readLedger(t, ws)
	if len(doc.Entries) != 2 {
		t.Fatalf("carried-forward ledger has %d entries, want 2 — a dispositioned entry is carried, never dropped", len(doc.Entries))
	}
	for _, e := range doc.Entries {
		switch e.ID {
		case "d1":
			if e.Status != "FIXED" || strings.TrimSpace(e.Evidence) == "" {
				t.Errorf("d1 carried forward as status=%q evidence=%q — the upstream closure proof must survive the hop", e.Status, e.Evidence)
			}
		case "d2":
			if e.Status != "DEFERRED" || strings.TrimSpace(e.Reason) == "" {
				t.Errorf("d2 carried forward as status=%q reason=%q — the upstream deferral reason must survive the hop", e.Status, e.Reason)
			}
		default:
			t.Errorf("unexpected entry id %q in the carried-forward ledger", e.ID)
		}
	}
}

// -- D6: emit covers WARN, not FAIL alone ---------------------------------

// TestClassify_WarnWithStructuredDefectsEmitsLedger — D6, MEDIUM. emit is
// gated on verdict == VerdictFAIL (audit.go:394) while scout Task 1 and F1 both
// specify FAIL/WARN. A WARN-shipped cycle carrying structured defects mints no
// ledger, so no later continuation can inherit them — a laundering channel left
// open by the fix that exists to close laundering.
func TestClassify_WarnWithStructuredDefectsEmitsLedger(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	defect := "probe quarantine skipped for a symlinked worktree"
	q, _ := json.Marshal([]string{defect})
	artifact := "# Audit Report\n\n## Verdict\n**WARN**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,` +
		`"failure":{"class":"deliverable-warn","defects":` + string(q) + `}} -->` + "\n"

	verdict, diags, _ := hooks{}.Classify(artifact,
		core.PhaseRequest{Cycle: 1281, Workspace: ws, ProjectRoot: t.TempDir()}, core.BridgeResponse{})

	raw, err := os.ReadFile(filepath.Join(ws, ledgerFile))
	if err != nil {
		t.Fatalf("a WARN carrying structured defects must mint a ledger (read %s: %v); verdict was %q, diagnostics:\n%s",
			ledgerFile, err, verdict, diagsText(diags))
	}
	if !strings.Contains(string(raw), defect) {
		t.Errorf("the WARN ledger does not carry the defect verbatim; ledger:\n%s", raw)
	}
	if verdict != core.VerdictWARN {
		t.Errorf("fixture verdict = %q, want WARN — the emit widening must not change how a WARN is graded", verdict)
	}
}

// TestClassify_WarnWithoutStructuredDefectsMintsNothing — the NEGATIVE guard on
// D6. Widening to WARN must not make every warned cycle mint a ledger: an empty
// or defect-less ledger makes every later cycle look like a continuation and is
// the cheapest way to render the reconcile gate vacuous.
func TestClassify_WarnWithoutStructuredDefectsMintsNothing(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	artifact := "# Audit Report\n\n## Verdict\n**WARN**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":1} -->` + "\n"

	hooks{}.Classify(artifact,
		core.PhaseRequest{Cycle: 1281, Workspace: ws, ProjectRoot: t.TempDir()}, core.BridgeResponse{})

	if _, err := os.Stat(filepath.Join(ws, ledgerFile)); err == nil {
		t.Error("a WARN with no structured defects must not mint a ledger — an empty ledger makes every later cycle look like a continuation")
	}
}
