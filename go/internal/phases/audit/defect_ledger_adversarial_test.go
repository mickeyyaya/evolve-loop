package audit

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// defect_ledger_adversarial_test.go — the cycle-1282 audit's PoCs promoted from
// a throwaway `-overlay` into the tree. That audit's finding was not that the
// code was sloppy; it was that every committed suite was green because none of
// them constructed adversarial input. These do. They are regression locks: each
// FAILED on the pre-fix tree and names the defect it pins.

// passingReport is the narrative-PASS artifact shape used by every case here.
func passingReport() string {
	return "# Audit Report\n\n## Verdict\n**PASS**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->` + "\n"
}

// TestAdversarial_PrePlantedWorkspaceLedgerCannotDisposition — cycle-1282 DEF-1
// (CRITICAL). The graded phase agent may write inside `.evolve/runs/cycle-N/`,
// so reading disposition state out of THIS cycle's own ledger let one file
// write close three inherited CRITICALs with no disposition artifact at all —
// and, because the merge keyed on ID alone, substitute the planted row's TEXT
// for the inherited defect's under the real id.
func TestAdversarial_PrePlantedWorkspaceLedgerCannotDisposition(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, laundered)
	planted := []any{}
	for i := range laundered {
		planted = append(planted, map[string]any{
			"id": "d" + strconv.Itoa(i+1), "text": "nothing to see", "status": "FIXED",
			"evidence": evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go"),
		})
	}
	writeJSON(t, filepath.Join(ws, ledgerFile), map[string]any{"origin_cycle": 1255, "entries": planted})

	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Errorf("a pre-planted workspace ledger closed %d inherited defects with no %s — the graded agent authored the gate's own trusted input.\ndiagnostics:\n%s",
			len(laundered), dispositionFile, diagsText(diags))
	}
	// The tamper-evident record must still carry the ANCESTOR's text.
	doc := readLedger(t, ws)
	for _, want := range laundered {
		found := false
		for _, e := range doc.Entries {
			if e.Text == want {
				found = true
				if e.Status != "OPEN" {
					t.Errorf("inherited defect %s is recorded %s on a planted row; want OPEN", e.ID, e.Status)
				}
			}
		}
		if !found {
			t.Errorf("inherited defect text %q was erased from the written-back ledger by the planted row", want)
		}
	}
}

// TestAdversarial_UnrelatedExistingFileDoesNotCloseADefect — cycle-1282 DEF-2
// (HIGH). evidenceResolves only os.Stat'd the citation under either root, with
// a raw-absolute branch, so `/etc/hosts` and the attacker's own disposition
// file each closed a CRITICAL. Existence is necessary, not sufficient.
func TestAdversarial_UnrelatedExistingFileDoesNotCloseADefect(t *testing.T) {
	cases := []struct {
		name     string
		evidence string
	}{
		{"absolute path outside the repo", "/etc/hosts"},
		{"the claim's own disposition file", dispositionFile + ":1"},
		{"the continuation manifest", "continuation-manifest.json"},
		{"the ledger the gate itself writes", ledgerFile},
		{"traversal out of the roots", "../../../../../../etc/hosts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, req := continuationFixture(t, 1255, 1270, laundered)
			var claims []any
			for i := range laundered {
				claims = append(claims, map[string]any{
					"id": "d" + strconv.Itoa(i+1), "status": "FIXED", "evidence": tc.evidence,
				})
			}
			writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{"dispositions": claims})

			verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
			if verdict == core.VerdictPASS {
				t.Errorf("evidence %q closed every inherited CRITICAL; a citation must name an in-repo file related to the fix.\ndiagnostics:\n%s", tc.evidence, diagsText(diags))
			}
		})
	}
}

// TestAdversarial_ShadowedIDIsLoudAndBlocking — cycle-1282 DEF-3 (MEDIUM). A
// 4-byte defectID is ~2^32 from a chosen second preimage, and the merge index
// resolved a duplicated id to the LAST row, shadowing the inherited entry. The
// id is now 16 bytes, the FIRST row wins the index, and a text mismatch on an
// inherited id blocks instead of silently rewriting the record.
func TestAdversarial_ShadowedIDIsLoudAndBlocking(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, laundered)
	writeJSON(t, filepath.Join(ws, ledgerFile), map[string]any{
		"origin_cycle": 1255,
		"entries":      []any{map[string]any{"id": "d1", "text": "typo in a comment", "status": "OPEN"}},
	})
	var claims []any
	for i := range laundered {
		claims = append(claims, map[string]any{"id": "d" + strconv.Itoa(i+1), "status": "DEFERRED", "reason": "cosmetic"})
	}
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{"dispositions": claims})

	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Errorf("a row shadowing an inherited id passed; got PASS.\ndiagnostics:\n%s", diagsText(diags))
	}
	if !strings.Contains(diagsText(diags), "shadow") {
		t.Errorf("the shadowing must be NAMED so the operator can tell it from an ordinary unaccounted defect; got:\n%s", diagsText(diags))
	}
	for _, e := range readLedger(t, ws).Entries {
		if e.ID == "d1" && e.Text != laundered[0] {
			t.Errorf("inherited row d1 now reads %q — the ancestor's text is the record", e.Text)
		}
	}
}

// TestDefectID_IsWideEnoughToResistASecondPreimage — DEF-3's width half. 4 bytes
// is brute-forceable over agent-chosen defect text, while the comment claimed
// "two different defects never share one".
func TestDefectID_IsWideEnoughToResistASecondPreimage(t *testing.T) {
	id := defectID("some defect text")
	if len(id) != 33 { // "d" + 16 bytes hex
		t.Errorf("defectID() = %q (%d chars); want a 16-byte hex id — a 32-bit id is aimable", id, len(id))
	}
	if defectID("a") == defectID("b") {
		t.Error("distinct texts share an id")
	}
}

// TestEmitDefectLedger_CapsUnboundedDefects — cycle-1282 DEF-6 (LOW). The
// verdict sentinel is agent-authored; neither the defect count nor the per-line
// length was bounded. The cap must RECORD the overflow, never erase it.
func TestEmitDefectLedger_CapsUnboundedDefects(t *testing.T) {
	ws := t.TempDir()
	defects := make([]string, defectLedgerMaxEntries+20)
	for i := range defects {
		defects[i] = "defect " + strconv.Itoa(i) + " " + strings.Repeat("x", defectTextMaxRunes+500)
	}
	if err := emitDefectLedger(failingReportWithDefects(defects...), core.PhaseRequest{Cycle: 1285, Workspace: ws}); err != nil {
		t.Fatalf("emitDefectLedger: %v", err)
	}
	doc := readLedger(t, ws)
	if len(doc.Entries) > defectLedgerMaxEntries+1 {
		t.Errorf("ledger holds %d entries; want at most the cap (%d) plus one overflow marker", len(doc.Entries), defectLedgerMaxEntries)
	}
	overflow := false
	for _, e := range doc.Entries {
		if len([]rune(e.Text)) > defectTextMaxRunes+len("…[truncated]") {
			t.Errorf("entry %s is %d runes; want bounded at %d", e.ID, len([]rune(e.Text)), defectTextMaxRunes)
		}
		if strings.Contains(e.Text, "were not recorded") {
			overflow = true
			if e.Status != "OPEN" {
				t.Errorf("the overflow marker is %s; it must be OPEN so a continuation inherits it", e.Status)
			}
		}
	}
	if !overflow {
		t.Error("defects beyond the cap were dropped with no marker — a silent cap is a laundering channel")
	}
}
