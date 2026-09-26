package core

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
