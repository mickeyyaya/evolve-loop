package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// defect_ledger_schema_inline_test.go — RED contract for cycle-1403 Task 3
// `disposition-parse-error-surfaced-inline` (scout-report.md Task 3).
//
// Today a rejected defect-dispositions.json yields the raw encoding/json error
// ("cannot unmarshal number into Go struct field …") and nothing else. The
// agent that must re-author the file on the next dispatch does not read Go, so
// the diagnostic names the failure without naming the remedy. Task 3 makes the
// rejection self-sufficient: the message carries the literal schema the file
// was supposed to match.
//
// Both cases drive hooks{}.Classify, the production verdict seam.
//
// Adversarial diversity: positive (the unparseable branch gains the schema) and
// negative (the MISSING branch is a DIFFERENT operator action — author the file
// — and must not be relabelled as a parse failure by this change).

// dispositionSchemaTokens are the field names the inline schema must carry. The
// contract is on the field vocabulary, not on one exact punctuation of it: any
// rendering that names all five is self-sufficient for a re-authoring agent.
var dispositionSchemaTokens = []string{"dispositions", "id", "status", "evidence", "reason"}

// TestClassify_DispositionUnparseableErrorNamesSchema — AC6. A wrong-typed
// `evidence` (a number — neither the string nor the array-of-strings Task 1
// admits) is rejected. The blocking diagnostic must name the expected schema
// inline, alongside the existing unparseable marker and the underlying error.
func TestClassify_DispositionUnparseableErrorNamesSchema(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": 42},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if verdict == core.VerdictPASS {
		t.Fatalf("fixture is wrong: an unparseable disposition file must block. diagnostics:\n%s", text)
	}
	if !strings.Contains(text, evidenceUnparseableMarker) {
		t.Fatalf("fixture is wrong: expected the unparseable branch, got:\n%s", text)
	}
	for _, tok := range dispositionSchemaTokens {
		if !strings.Contains(text, tok) {
			t.Errorf("AC6 unmet — the unparseable diagnostic must carry the expected schema inline so the next dispatch can re-author the file without reading Go; field %q is absent. diagnostics:\n%s", tok, text)
		}
	}
	if !strings.Contains(text, "FIXED") || !strings.Contains(text, "DEFERRED") {
		t.Errorf("AC6 unmet — the inline schema must name the two legal statuses (FIXED / DEFERRED); a field list alone still leaves the agent guessing the values. diagnostics:\n%s", text)
	}
}

// TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable —
// NEGATIVE, the no-false-positive half. An ABSENT file is a different finding
// with a different remedy and already has its own named marker. Task 3 must not
// smear the parse-failure text across it.
func TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable(t *testing.T) {
	_, req := continuationFixture(t, 1398, 1403, oneDefect)
	// Deliberately write no defect-dispositions.json.

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if verdict == core.VerdictPASS {
		t.Fatalf("fixture is wrong: a continuation with no disposition file must block. diagnostics:\n%s", text)
	}
	if !strings.Contains(text, "disposition-preflight: MISSING") {
		t.Errorf("AC7 unmet — the absent-file branch must keep its own named marker after the inline-schema change. diagnostics:\n%s", text)
	}
	if strings.Contains(text, evidenceUnparseableMarker) {
		t.Errorf("AC7 unmet — an absent file was never parsed; reporting it as unparseable would send the operator after the wrong remedy. diagnostics:\n%s", text)
	}
}
