package deliverable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// buildSpecWithHandoff is the registry's build declaration in miniature: two files, the second owed by the agent.
func buildSpecWithHandoff() phasespec.PhaseSpec {
	return phasespec.PhaseSpec{
		Name: "build", Role: "build",
		Outputs: phasespec.IO{
			Files:     []string{".evolve/runs/cycle-{cycle}/build-report.md", ".evolve/runs/cycle-{cycle}/handoff-build.json"},
			AgentOwed: []string{"handoff-build.json"},
		},
	}
}

func resolverFor(specs ...phasespec.PhaseSpec) phasecontract.Resolver {
	cat, warnings := (phasespec.Catalog{}).Merge(specs)
	if len(warnings) != 0 {
		panic(warnings)
	}
	return phasecontract.NewCatalogResolver(cat.Get)
}

const validBuildReport = "# Build Report\n\n## Changes\n- foo.go\n\n## Handoff Summary\n- done\n"

func TestVerify_AgentOwedSecondary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		handoff  *string // nil = absent
		wantCode string  // "" = OK
	}{
		{"absent", nil, CodeMissingSecondary},
		{"empty", strPtr(""), CodeEmptySecondary},
		{"malformed json", strPtr("{not json"), CodeMalformedSecondary},
		{"present and valid", strPtr(`{"task":"x"}`), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, "build-report.md", validBuildReport)
			if tc.handoff != nil {
				writeFile(t, ws, "handoff-build.json", *tc.handoff)
			}
			res, err := VerifyWithStage("build", phasecontract.Roots{Workspace: ws}, resolverFor(buildSpecWithHandoff()), config.StageOff)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantCode == "" {
				if !res.OK {
					t.Fatalf("want OK, got violations: %+v", res.Violations)
				}
				return
			}
			if res.OK || !hasCode(res, tc.wantCode) {
				t.Fatalf("handoff-build.json is declared agent-owed and is %s, but the gate returned OK=%v violations=%+v (want %s) — the phase would proceed with its declared deliverable missing",
					tc.name, res.OK, res.Violations, tc.wantCode)
			}
			if !violationMentions(res, "handoff-build.json") {
				t.Fatalf("violation does not name the missing file: %+v", res.Violations)
			}
		})
	}
}

func TestVerify_HarnessProducedSecondaryIsNotGated(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", "# Audit Report\n\n## Verdict\nPASS\n\n## Findings\n- none\n\n## Handoff Summary\n- ok\n")
	spec := phasespec.PhaseSpec{Name: "audit", Role: "evaluate",
		Outputs: phasespec.IO{
			Files:           []string{".evolve/runs/cycle-{cycle}/audit-report.md", ".evolve/runs/cycle-{cycle}/acs-verdict.json"},
			HarnessProduced: []string{"acs-verdict.json"},
		}}
	res, err := VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, resolverFor(spec), config.StageOff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(res, CodeMissingSecondary) {
		t.Fatalf("a harness-produced file must never be demanded of the agent: %+v", res.Violations)
	}
}

func TestVerify_NDJSONSecondary(t *testing.T) {
	spec := phasespec.PhaseSpec{Name: "scout", Role: "discover",
		Outputs: phasespec.IO{
			Files:     []string{".evolve/runs/cycle-{cycle}/scout-report.md", ".evolve/runs/cycle-{cycle}/findings.ndjson"},
			AgentOwed: []string{"findings.ndjson"},
		}}
	for _, tc := range []struct{ name, body, want string }{
		{"valid lines", "{\"a\":1}\n\n{\"b\":2}\n", ""},
		{"one bad line", "{\"a\":1}\nnot json\n", CodeMalformedSecondary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, "scout-report.md", "# Scout\n\n## Selected Tasks\n- t1\n\n## Handoff Summary\n- ok\n")
			writeFile(t, ws, "findings.ndjson", tc.body)
			res, err := VerifyWithStage("scout", phasecontract.Roots{Workspace: ws}, resolverFor(spec), config.StageOff)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := hasCode(res, CodeMalformedSecondary); got != (tc.want != "") {
				t.Fatalf("malformed=%v, want %v; violations=%+v", got, tc.want != "", res.Violations)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func violationMentions(res Result, s string) bool {
	for _, v := range res.Violations {
		if strings.Contains(v.Message, s) {
			return true
		}
	}
	return false
}

func TestReviewer_VerifiesDeclaredDeliverables(t *testing.T) {
	for _, tc := range []struct {
		stage config.Stage
		want  bool
	}{{config.StageOff, false}, {config.StageShadow, true}, {config.StageEnforce, true}} {
		r := newReviewer(tc.stage, phasecontract.BuiltinResolver{}, config.StageOff)
		if got := r.VerifiesDeclaredDeliverables(); got != tc.want {
			t.Fatalf("stage %v: VerifiesDeclaredDeliverables() = %v, want %v", tc.stage, got, tc.want)
		}
	}
}

// A directory where the file should be is a non-absence read fault.
func TestVerify_UnreadableSecondaryIsAmbiguity(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", validBuildReport)
	if err := os.MkdirAll(filepath.Join(ws, "handoff-build.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := VerifyWithStage("build", phasecontract.Roots{Workspace: ws}, resolverFor(buildSpecWithHandoff()), config.StageOff)
	if err == nil {
		t.Fatal("an unreadable secondary must surface as an error (fail-open ambiguity), not as a violation the ladder would re-dispatch for")
	}
}
