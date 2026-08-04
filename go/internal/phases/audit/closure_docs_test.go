package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// closure_docs_test.go — cycle-1287 RED contract for the closure-citation gate's
// own paperwork (Task 2, batch-integrity-review-doc-closure-crossref).
//
// The gate in closure_claim.go exists because the 1255 → 1272 chain closed a
// CRITICAL with the words "verified closed" and no record. The two documents
// that narrate that gate must therefore SATISFY it — a doc that announces the
// rule while breaking it is the exact pattern the inbox item names. This is a
// self-check, not prose review: it runs the production `closureClaimOffenders`
// over the real committed files.

// closureGovernedDocs are the documents this cycle's landing must leave in a
// state the shipped gate accepts. Scoped deliberately to the two files named in
// triage's top_n — a repo-wide sweep would make an unrelated future doc edit
// fail this cycle's contract.
var closureGovernedDocs = []string{
	"docs/operations/batch-integrity-review-2026-08-04.md",
	"docs/architecture/continuation-defect-ledger.md",
}

// closureDocsRepoRoot walks up from the test's working directory (the package
// dir under `go test`) until it finds the checkout root, identified by the
// docs/architecture directory. Walking beats a hard-coded "../../../.." because
// the audit package's depth is not this test's business.
func closureDocsRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "docs", "architecture")); statErr == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate checkout root (no docs/architecture above %q)", dir)
		}
		dir = parent
	}
}

// TestC1287_DocsPassClosureCitationGate is the cycle-1287 crux for Task 2: every
// closure claim in the two governed documents must name the per-defect
// disposition record on its own line. RED until the "Not closed here" section
// and the F1 accounting lines are rewritten as cited closure records.
func TestC1287_DocsPassClosureCitationGate(t *testing.T) {
	root := closureDocsRepoRoot(t)
	for _, rel := range closureGovernedDocs {
		path := filepath.Join(root, filepath.FromSlash(rel))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		offenders := closureClaimOffenders(string(raw))
		if len(offenders) > 0 {
			t.Errorf("%s: %d closure claim(s) cite no disposition record on their own line; each must name %s or %s inline:",
				rel, len(offenders), defectDispositionFile, defectLedgerFile)
			for _, o := range offenders {
				t.Errorf("  offender: %s", o)
			}
		}
	}
}

// TestC1287_ClosureGateRejectsUncitedClaim is the anti-neutering guard for the
// test above. The cheapest way to green a docs gate is to weaken the gate, so
// the rejection behaviour is pinned independently: an uncited claim must still
// be flagged, a cited one must not, and ordinary prose about a closed file
// handle must stay invisible.
func TestC1287_ClosureGateRejectsUncitedClaim(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool // want flagged
	}{
		{"bare verified-closed", "The 1255 CRITICAL is verified closed.", true},
		{"closed with cycle reference", "D1 from cycle-1272 is closed.", true},
		{"cited by disposition file", "D1 from cycle-1272 is closed — see " + defectDispositionFile + ".", false},
		{"cited by ledger file", "The 1255 CRITICAL is verified closed per " + defectLedgerFile + ".", false},
		{"ordinary prose", "the file handle is closed in the deferred cleanup", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(closureClaimOffenders(tc.line)) > 0
			if got != tc.want {
				t.Errorf("closureClaimOffenders(%q) flagged=%v, want %v", tc.line, got, tc.want)
			}
			if !tc.want {
				return
			}
			diags := closureClaimDiagnostics(tc.line)
			if len(diags) != 1 || diags[0].Severity != "error" {
				t.Errorf("closureClaimDiagnostics(%q) = %#v, want exactly one error diagnostic", tc.line, diags)
			}
			if !strings.Contains(diags[0].Message, defectDispositionFile) {
				t.Errorf("diagnostic does not name the remedy artifact %s: %s", defectDispositionFile, diags[0].Message)
			}
		})
	}
}
