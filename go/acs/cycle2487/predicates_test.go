//go:build acs

package cycle2487

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
)

const registryBody = `{"schema_version":4,"phases":[{"name":"scout","optional":false},{"name":"build","optional":false}]}`

func ghostFieldSpec(trigger string) string {
	return `{"name":"late-check","kind":"llm","optional":true,"archetype":"evaluate","after":"build",
	"routing":{"` + trigger + `":[{"field":"ghost.signal","op":"==","value":"x"}],"rubric_hint":["r"]}}`
}

func producedFieldSpec() string {
	return `{"name":"late-check","kind":"llm","optional":true,"archetype":"evaluate","after":"build",
	"routing":{"insert_when":[{"field":"build.verdict","op":"==","value":"FAIL"}],"rubric_hint":["r"]}}`
}

func projectWith(t *testing.T, userPhases map[string]string) string {
	t.Helper()
	root := t.TempDir()
	reg := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reg, "phase-registry.json"), []byte(registryBody), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range userPhases {
		specPath := filepath.Join(root, ".evolve", "phases", name, "phase.json")
		if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(specPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

func runPhases(args []string, stdin string) (int, string, string) {
	var out, errb bytes.Buffer
	code := phasecmd.RunPhases(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestC2487_001_ValidateRefusesAnInsertWhenFieldNothingProduces(t *testing.T) {
	projectWith(t, map[string]string{"late-check": ghostFieldSpec("insert_when")})
	code, out, errb := runPhases([]string{"validate"}, "")
	if code == 0 {
		t.Fatalf("RED: validate exit 0 for an unproduced insert_when field; out=%s", out)
	}
	if !strings.Contains(out+errb, "late-check") || !strings.Contains(out+errb, "ghost.signal") {
		t.Errorf("RED: output must name the phase and the field; out=%s stderr=%s", out, errb)
	}
}

func TestC2487_002_ValidateRefusesASkipWhenFieldNothingProduces(t *testing.T) {
	projectWith(t, map[string]string{"late-check": ghostFieldSpec("skip_when")})
	code, out, errb := runPhases([]string{"validate", "late-check"}, "")
	if code == 0 {
		t.Fatalf("RED: validate exit 0 for an unproduced skip_when field; out=%s", out)
	}
	if !strings.Contains(out+errb, "late-check") || !strings.Contains(out+errb, "ghost.signal") {
		t.Errorf("RED: output must name the phase and the field; out=%s stderr=%s", out, errb)
	}
}

func TestC2487_003_ValidateAcceptsAFieldTheHarnessTypesAsASignal(t *testing.T) {
	projectWith(t, map[string]string{"late-check": producedFieldSpec()})
	code, out, errb := runPhases([]string{"validate"}, "")
	if code != 0 {
		t.Fatalf("validate exit %d for a typed-signal trigger; out=%s stderr=%s", code, out, errb)
	}
}

func TestC2487_004_ValidateAcceptsAFieldAnEarlierPhaseDeclaresInItsOutputs(t *testing.T) {
	projectWith(t, map[string]string{
		"scan-hits": `{"name":"scan-hits","kind":"llm","optional":true,"archetype":"evaluate","after":"build","outputs":{"files":["scan-hits-report.md"],"signals":["scan.hits"]}}`,
		"late-check": `{"name":"late-check","kind":"llm","optional":true,"archetype":"evaluate","after":"scan-hits",
	"routing":{"insert_when":[{"field":"scan.hits","op":">","value":0}],"rubric_hint":["r"]}}`,
	})
	code, out, errb := runPhases([]string{"validate"}, "")
	if code != 0 {
		t.Fatalf("validate exit %d for a trigger an earlier phase produces; out=%s stderr=%s", code, out, errb)
	}
}

func TestC2487_005_ValidateRefusesAFieldOnlyALaterPhaseDeclares(t *testing.T) {
	projectWith(t, map[string]string{
		"late-check": `{"name":"late-check","kind":"llm","optional":true,"archetype":"evaluate","after":"build",
	"routing":{"insert_when":[{"field":"scan.hits","op":">","value":0}],"rubric_hint":["r"]}}`,
		"scan-hits": `{"name":"scan-hits","kind":"llm","optional":true,"archetype":"evaluate","after":"late-check","outputs":{"files":["scan-hits-report.md"],"signals":["scan.hits"]}}`,
	})
	code, out, errb := runPhases([]string{"validate"}, "")
	if code == 0 {
		t.Fatalf("RED: validate exit 0 although the producer runs after the consumer; out=%s", out)
	}
	if !strings.Contains(out+errb, "late-check") || !strings.Contains(out+errb, "scan.hits") {
		t.Errorf("RED: output must name the phase and the field; out=%s stderr=%s", out, errb)
	}
}

func TestC2487_006_CreateRefusesAPhaseWhoseTriggerFieldNothingProduces(t *testing.T) {
	root := projectWith(t, nil)
	code, out, errb := runPhases([]string{"create", "--spec", "-"}, ghostFieldSpec("insert_when"))
	if code == 0 {
		t.Fatalf("RED: create exit 0 for an unproduced trigger field; out=%s", out)
	}
	var env struct {
		OK     bool     `json:"ok"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("RED: stdout is not the create envelope: %v; out=%q stderr=%q", err, out, errb)
	}
	joined := strings.Join(env.Errors, "\n")
	if env.OK || !strings.Contains(joined, "late-check") || !strings.Contains(joined, "ghost.signal") {
		t.Errorf("RED: envelope must refuse naming the phase and the field; env=%+v", env)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "phases", "late-check", "phase.json")); err == nil {
		t.Errorf("RED: a refused phase must leave no phase.json behind")
	}
}

func TestC2487_007_CreateAcceptsAPhaseWhoseTriggerFieldTheHarnessProduces(t *testing.T) {
	root := projectWith(t, nil)
	code, out, errb := runPhases([]string{"create", "--spec", "-"}, producedFieldSpec())
	if code != 0 {
		t.Fatalf("create exit %d for a typed-signal trigger; out=%s stderr=%s", code, out, errb)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "phases", "late-check", "phase.json")); err != nil {
		t.Errorf("accepted phase must be written: %v", err)
	}
}
