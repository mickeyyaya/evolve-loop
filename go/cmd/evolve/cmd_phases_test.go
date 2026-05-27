package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newPhasesProject builds a temp project root with a minimal built-in registry
// and points EVOLVE_PROJECT_ROOT at it. Returns the root.
func newPhasesProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	reg := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":4,"phases":[
		{"name":"scout","optional":false},
		{"name":"build","optional":false}
	]}`
	if err := os.WriteFile(filepath.Join(reg, "phase-registry.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

func writeUserPhaseFile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "phases", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunPhases_NoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runPhases(nil, nil, &out, &errb); rc != 10 {
		t.Errorf("rc = %d, want 10; stderr=%s", rc, errb.String())
	}
}

func TestRunPhases_List(t *testing.T) {
	root := newPhasesProject(t)
	writeUserPhaseFile(t, root, "security-scan", `{"name":"security-scan","optional":true,"kind":"llm"}`)

	var out, errb bytes.Buffer
	if rc := runPhases([]string{"list"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr=%s", rc, errb.String())
	}
	s := out.String()
	for _, want := range []string{"scout", "build", "security-scan", "builtin", "user"} {
		if !strings.Contains(s, want) {
			t.Errorf("list output missing %q:\n%s", want, s)
		}
	}
}

func TestRunPhases_ValidateClean(t *testing.T) {
	root := newPhasesProject(t)
	writeUserPhaseFile(t, root, "security-scan", `{"name":"security-scan","optional":true,"kind":"llm"}`)
	var out, errb bytes.Buffer
	if rc := runPhases([]string{"validate"}, nil, &out, &errb); rc != 0 {
		t.Errorf("rc = %d, want 0; out=%s", rc, out.String())
	}
}

func TestRunPhases_ValidateFail(t *testing.T) {
	root := newPhasesProject(t)
	// not optional → floor violation
	writeUserPhaseFile(t, root, "bad-phase", `{"name":"bad-phase","optional":false}`)
	var out, errb bytes.Buffer
	if rc := runPhases([]string{"validate"}, nil, &out, &errb); rc != 2 {
		t.Errorf("rc = %d, want 2; out=%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "must be optional") {
		t.Errorf("expected floor violation in output:\n%s", out.String())
	}
}

func TestRunPhases_AddThenValidate(t *testing.T) {
	root := newPhasesProject(t)
	var out, errb bytes.Buffer
	if rc := runPhases([]string{"add", "my-check"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("add rc = %d, want 0; stderr=%s", rc, errb.String())
	}
	// scaffolded files exist
	for _, f := range []string{"phase.json", "agent.md", "profile.json"} {
		p := filepath.Join(root, ".evolve", "phases", "my-check", f)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}
	// the scaffolded spec validates clean
	out.Reset()
	if rc := runPhases([]string{"validate", "my-check"}, nil, &out, &errb); rc != 0 {
		t.Errorf("validate scaffolded rc = %d, want 0; out=%s", rc, out.String())
	}
	// re-add refuses
	if rc := runPhases([]string{"add", "my-check"}, nil, &out, &errb); rc != 1 {
		t.Errorf("re-add rc = %d, want 1 (exists)", rc)
	}
}

func TestRunPhases_BadName(t *testing.T) {
	newPhasesProject(t)
	// Uppercase/space and path-traversal names are both rejected before any
	// mkdir (ValidateUserSpec's kebab-case floor), closing the escape vector.
	for _, bad := range []string{"Bad Name", "../escape", "a/b"} {
		var out, errb bytes.Buffer
		if rc := runPhases([]string{"add", bad}, nil, &out, &errb); rc != 10 {
			t.Errorf("add %q: rc = %d, want 10 (rejected name)", bad, rc)
		}
	}
}
