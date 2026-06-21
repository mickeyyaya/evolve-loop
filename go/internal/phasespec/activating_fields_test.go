package phasespec

// activating_fields_test.go — PA-BIG S4 (ADR-0058): the load-time validator for
// the transition-activating fields. ValidateActivatingFields enforces
// well-formedness (a known branching_strategy; on_pass/on_fail declared as a
// pair), and Load rejects a malformed registry — the registry is a contract, so
// a half-declared verdict branch or an unknown strategy fails loudly at load
// rather than silently degrading to the literal kernel.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateActivatingFields pins the well-formedness rules: a known (or empty)
// branching_strategy passes; an unknown one is rejected; on_pass/on_fail must be
// declared together or not at all (a half-set is dead config Next ignores).
func TestValidateActivatingFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		spec          PhaseSpec
		wantViolation bool
	}{
		{name: "empty-is-valid", spec: PhaseSpec{Name: "scout"}, wantViolation: false},
		{name: "history-is-valid", spec: PhaseSpec{Name: "retrospective", BranchingStrategy: BranchingHistory}, wantViolation: false},
		{name: "signal-is-valid", spec: PhaseSpec{Name: "debugger", BranchingStrategy: BranchingSignal}, wantViolation: false},
		{name: "verdict-pair-is-valid", spec: PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "retrospective"}, wantViolation: false},
		{name: "unknown-strategy-rejected", spec: PhaseSpec{Name: "x", BranchingStrategy: "telepathy"}, wantViolation: true},
		{name: "on_pass-without-on_fail-rejected", spec: PhaseSpec{Name: "x", OnPass: "ship"}, wantViolation: true},
		{name: "on_fail-without-on_pass-rejected", spec: PhaseSpec{Name: "x", OnFail: "retrospective"}, wantViolation: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ValidateActivatingFields(c.spec)
			if (len(got) > 0) != c.wantViolation {
				t.Errorf("ValidateActivatingFields(%+v) = %v, wantViolation=%v", c.spec, got, c.wantViolation)
			}
		})
	}
}

// TestLoad_RejectsMalformedActivatingFields proves Load wires the validator: a
// registry with a half-declared verdict branch is a hard load error, not a
// silent degrade.
func TestLoad_RejectsMalformedActivatingFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "phase-registry.json")
	const malformed = `{"phases":[{"name":"audit","role":"audit","on_pass":"ship"}]}` // on_fail missing
	if err := os.WriteFile(path, []byte(malformed), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load must reject a registry with a half-declared verdict branch")
	}
	if !strings.Contains(err.Error(), "on_pass") {
		t.Errorf("Load error should name the offending field; got %v", err)
	}
}

// TestDiscoverUserSpecs_StripsActivatingFields enforces the ADR-0058 trust
// boundary at the real user-file ingestion point: a user phase.json may NOT
// inject a transition branch into the kernel. Activating fields on a discovered
// user spec are stripped (with a warning) so the verdict/history/signal
// vocabulary stays built-in-only — a user phase that is a `current` in the flow
// can never route via injected on_pass/on_fail.
func TestDiscoverUserSpecs_StripsActivatingFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeUserPhase := func(name, body string) {
		d := filepath.Join(dir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "phase.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeUserPhase("my-check", `{"name":"my-check","optional":true,"kind":"llm","on_pass":"ship","on_fail":"retrospective","branching_strategy":"history"}`)
	writeUserPhase("clean-check", `{"name":"clean-check","optional":true,"kind":"llm"}`)

	specs, warns := DiscoverUserSpecs(dir)

	byName := map[string]PhaseSpec{}
	for _, s := range specs {
		byName[s.Name] = s
	}
	got, ok := byName["my-check"]
	if !ok {
		t.Fatal("user phase must still be admitted (only its activating fields stripped)")
	}
	if got.OnPass != "" || got.OnFail != "" || got.BranchingStrategy != "" {
		t.Errorf("user activating fields must be stripped; got on_pass=%q on_fail=%q strategy=%q",
			got.OnPass, got.OnFail, got.BranchingStrategy)
	}
	if clean := byName["clean-check"]; clean.BranchingStrategy != "" {
		t.Errorf("a clean user phase must pass through untouched")
	}

	stripWarns := 0
	for _, w := range warns {
		if strings.Contains(w, "my-check") && strings.Contains(w, "activating") {
			stripWarns++
		}
	}
	if stripWarns != 1 {
		t.Errorf("exactly one strip warning naming my-check expected; got %d in %v", stripWarns, warns)
	}
}

// TestLoad_RealRegistry_ActivatingFieldsWellFormed loads the SHIPPED registry and
// asserts every spec passes the well-formedness validator — guards against a
// typo'd strategy or a half-declared branch landing in the runtime contract.
func TestLoad_RealRegistry_ActivatingFieldsWellFormed(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json")
	cat, err := Load(path)
	if err != nil {
		t.Fatalf("Load shipped registry: %v", err)
	}
	for _, name := range cat.Names() {
		spec, _ := cat.Get(name)
		if viol := ValidateActivatingFields(spec); len(viol) > 0 {
			t.Errorf("shipped registry phase %q has malformed activating fields: %v", name, viol)
		}
	}
}
