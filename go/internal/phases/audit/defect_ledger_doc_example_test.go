package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// defect_ledger_doc_example_test.go — RED contract for cycle-1403 Task 2
// `disposition-schema-literal-example` (scout-report.md Task 2).
//
// agents/evolve-auditor.md tells the auditor to write
// `{"dispositions":[{"id","status","evidence","reason"}]}` — a list of FIELD
// NAMES, not a document. It is not valid JSON and shows no legal value for any
// field, so the authoring agent must invent the shape; cycles 1397/1399/1400
// each invented a different wrong one. Task 2 replaces it with a filled literal
// example and keeps it identical to the one already in
// docs/architecture/continuation-defect-ledger.md.
//
// These predicates are NOT source greps for a magic string (the cycle-85 ban).
// They EXTRACT the documented example and run it through the production reader,
// readDispositions — the same function the gate calls — so a doc example that
// the gate would reject fails here. The cross-document case then compares the
// two examples as parsed JSON, not as text, so reformatting one is fine and
// drifting one is not.

// dispositionExampleFence matches a fenced ```json block whose body mentions
// "dispositions". Documents may carry other JSON fences; only this one is the
// disposition example.
var dispositionExampleFence = regexp.MustCompile("(?s)```json\\s*\\n(.*?)```")

// docExampleRepoRoot resolves the repo root from this test file's location
// (4 levels up from go/internal/phases/audit/), mirroring skillsDriftRepoRoot.
func docExampleRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
}

// extractDispositionExample returns the first fenced JSON block in rel that
// mentions "dispositions". Absence is a FAILURE, not a skip: the example is
// this cycle's deliverable.
func extractDispositionExample(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	for _, m := range dispositionExampleFence.FindAllStringSubmatch(string(raw), -1) {
		if strings.Contains(m[1], "\"dispositions\"") {
			return m[1]
		}
	}
	t.Fatalf("%s carries no fenced ```json block containing \"dispositions\" — the schema is stated as bare field names, which is the cycle-1397/1399/1400 root cause: an authoring agent has no legal document to copy", rel)
	return ""
}

// TestAuditorPromptDispositionExampleIsAcceptedByProductionReader — AC8. The
// example in the auditor's own prompt must be a document the gate can read: it
// goes through readDispositions, the production reader, exactly as a real
// workspace file would.
func TestAuditorPromptDispositionExampleIsAcceptedByProductionReader(t *testing.T) {
	root := docExampleRepoRoot(t)
	example := extractDispositionExample(t, root, "agents/evolve-auditor.md")

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, dispositionFile), []byte(example), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	claims, diags, blocked := readDispositions(ws, 1398)
	if blocked {
		t.Fatalf("AC8 unmet — the example the auditor prompt tells agents to copy is rejected by the gate's own reader. Copying the documentation must not fail the gate. diagnostics:\n%s\nexample:\n%s", diagsText(diags), example)
	}
	if len(claims) < 2 {
		t.Fatalf("AC8 unmet — the example must show at least two entries (one FIXED, one DEFERRED); parsed %d. example:\n%s", len(claims), example)
	}

	var fixed, deferred int
	for id, c := range claims {
		if id == "" || strings.ContainsAny(id, "<>") {
			t.Errorf("AC8 unmet — entry id %q is a placeholder, not a literal value; the whole point of the example is that every field shows a concrete legal value. example:\n%s", id, example)
		}
		switch c.Status {
		case defectStatusFixed:
			fixed++
			if strings.TrimSpace(c.Evidence) == "" {
				t.Errorf("AC8 unmet — the FIXED entry %q must carry literal `evidence`; a FIXED without evidence is the unevidenced closure the ledger blocks. example:\n%s", id, example)
			}
			if strings.Contains(c.Evidence, " (") {
				t.Errorf("AC8 unmet — the FIXED entry %q cites %q with a trailing parenthetical annotation; the example must model a BARE cite (cycles 1356/1360 ground out on decorated cites copied from prose). example:\n%s", id, c.Evidence, example)
			}
		case defectStatusDeferred:
			deferred++
			if strings.TrimSpace(c.Reason) == "" {
				t.Errorf("AC8 unmet — the DEFERRED entry %q must carry a literal non-empty `reason`. example:\n%s", id, example)
			}
		default:
			t.Errorf("AC8 unmet — entry %q has status %q; the example must show only the legal statuses %s / %s. example:\n%s", id, c.Status, defectStatusFixed, defectStatusDeferred, example)
		}
	}
	if fixed < 1 || deferred < 1 {
		t.Errorf("AC8 unmet — the example must show BOTH dispositions (got %d FIXED, %d DEFERRED); an agent shown only one shape guesses the other. example:\n%s", fixed, deferred, example)
	}
}

// TestAuditorPromptAndArchDocDispositionExamplesAgree — AC9, the doc-sync half
// (`always_full_documentation` house rule; cycle-1342 landed prompt and
// architecture doc together for exactly this reason). Compared as PARSED JSON,
// so reflowing or re-indenting one document is free and drifting its content is
// not.
func TestAuditorPromptAndArchDocDispositionExamplesAgree(t *testing.T) {
	root := docExampleRepoRoot(t)
	promptRaw := extractDispositionExample(t, root, "agents/evolve-auditor.md")
	docRaw := extractDispositionExample(t, root, "docs/architecture/continuation-defect-ledger.md")

	var promptDoc, archDoc any
	if err := json.Unmarshal([]byte(promptRaw), &promptDoc); err != nil {
		t.Fatalf("AC9 unmet — the auditor prompt's example is not valid JSON: %v\nexample:\n%s", err, promptRaw)
	}
	if err := json.Unmarshal([]byte(docRaw), &archDoc); err != nil {
		t.Fatalf("AC9 unmet — the architecture doc's example is not valid JSON: %v\nexample:\n%s", err, docRaw)
	}
	if !reflect.DeepEqual(promptDoc, archDoc) {
		t.Errorf("AC9 unmet — the schema example must be identical in agents/evolve-auditor.md and docs/architecture/continuation-defect-ledger.md; two divergent examples are two contracts and the agent obeys whichever it read.\nprompt:\n%s\narch doc:\n%s", promptRaw, docRaw)
	}
}
