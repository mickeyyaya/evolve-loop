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

var dispositionExampleFence = regexp.MustCompile("(?s)```json\\s*\\n(.*?)```")

func docExampleRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
}

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
