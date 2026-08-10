package core

// disposition_gate_singlesource_test.go — three-legged single-source pin for
// the disposition contract (ADR-0084 invariant 2, #422 pattern): (1) the
// retro persona's literal example is byte-for-byte the same JSON document as
// the Go-side dispositionSchemaExample; (2) that document passes the real
// VerifyDisposition against a matching digest; (3) drift in either projection
// fails CI here instead of failing a live retro against a fail-HARD gate.
// The pre-2026-08-10 prose "example" was placeholder pseudo-JSON ("<int>",
// "P0 | P1 | P2 | P3") — an agent copying it failed its own gate.

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

func repoRootForDisposition(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skipf("repo layout not found: %v", err)
	}
	return root
}

// personaDispositionExample extracts the ```json block that follows the
// "Required deliverable: disposition.json" heading in the retro persona.
func personaDispositionExample(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read retro persona: %v", err)
	}
	sec := string(raw)
	i := strings.Index(sec, "Required deliverable: disposition.json")
	if i < 0 {
		t.Fatal("retro persona no longer carries the disposition deliverable section")
	}
	m := regexp.MustCompile("(?s)```json\\s*(\\{.*?\\})\\s*```").FindStringSubmatch(sec[i:])
	if m == nil {
		t.Fatal("no ```json example block under the disposition deliverable section — the persona must show a literal example")
	}
	return m[1]
}

func TestDispositionExample_PersonaAndGoAgreeAsJSON(t *testing.T) {
	root := repoRootForDisposition(t)
	var fromPersona, fromGo map[string]any
	if err := json.Unmarshal([]byte(personaDispositionExample(t, root)), &fromPersona); err != nil {
		t.Fatalf("persona example is not legal JSON (the placeholder-pseudo-JSON regression): %v", err)
	}
	if err := json.Unmarshal([]byte(dispositionSchemaExample), &fromGo); err != nil {
		t.Fatalf("Go example const is not legal JSON: %v", err)
	}
	if !reflect.DeepEqual(fromPersona, fromGo) {
		t.Errorf("persona example and Go dispositionSchemaExample have drifted apart — single-source violation.\npersona: %v\ngo:      %v", fromPersona, fromGo)
	}
}

func TestDispositionExample_PassesProductionGate(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "disposition.json"), []byte(dispositionSchemaExample), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := json.Marshal(FailureDigest{Cycle: 1398, Fingerprint: "ship|gate-block|cd49274beab2", Recurrence: 2, PreClass: "gate-block"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "failure-digest.json"), digest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDisposition(ws); err != nil {
		t.Errorf("the documented literal example must pass the real gate against a matching digest; got: %v", err)
	}
}
