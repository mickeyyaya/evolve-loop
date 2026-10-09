package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var dispositionSchemaTokens = []string{"dispositions", "id", "status", "evidence", "reason"}

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

func TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable(t *testing.T) {
	_, req := continuationFixture(t, 1398, 1403, oneDefect)

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
