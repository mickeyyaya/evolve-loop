//go:build acs

package cycle1147

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1147_001_artifact_filename_resolves_registry_name(t *testing.T) {
	cases := []struct {
		phase string
		want  string
	}{
		{"retro", "retrospective-report.md"},
		{"build-planner", "build-plan.md"},
		{"scout", "scout-report.md"},
		{"build", "build-report.md"},
		{"audit", "audit-report.md"},
	}
	for _, tc := range cases {
		if got := phasecontract.ArtifactFilename(tc.phase); got != tc.want {
			t.Errorf("ArtifactFilename(%q) = %q, want %q", tc.phase, got, tc.want)
		}
	}
}

func TestC1147_002_artifact_filename_falls_back_on_convention(t *testing.T) {
	if got := phasecontract.ArtifactFilename("ship"); got != "ship-report.md" {
		t.Errorf("ArtifactFilename(\"ship\") = %q, want %q (NoArtifact ⇒ convention fallback)", got, "ship-report.md")
	}
	const unregistered = "cycle1147-not-a-registered-phase"
	if got := phasecontract.ArtifactFilename(unregistered); got != unregistered+"-report.md" {
		t.Errorf("ArtifactFilename(%q) = %q, want %q", unregistered, got, unregistered+"-report.md")
	}
	if got := phasecontract.ArtifactFilename(""); got == "" {
		t.Error("ArtifactFilename(\"\") returned \"\" — a caller joining this onto the workspace path would address the directory itself")
	}
}

func TestC1147_003_classify_prompt_echo_veto_finds_retro_deliverable(t *testing.T) {
	ws := writeRetroEchoWorkspace(t, phasecontract.ArtifactFilename("retro"))

	got := cycleclassify.Classify(ws)
	if got.Class == cycleclassify.ClassInfrastructure && got.Marker == echoMarker {
		t.Errorf("Classify(retro prompt-echo workspace) = %+v; want the echo VETOED "+
			"(deliverable present at the registry name %q) — the veto is reading %q instead",
			got, phasecontract.ArtifactFilename("retro"), "retro-report.md")
	}
}

func TestC1147_004_classify_veto_declines_on_legacy_only_name(t *testing.T) {
	ws := writeRetroEchoWorkspace(t, "retro-report.md")

	got := cycleclassify.Classify(ws)
	if got.Class != cycleclassify.ClassInfrastructure || got.Marker != echoMarker {
		t.Errorf("Classify(legacy-only-name workspace) = %+v; want ClassInfrastructure/%q — "+
			"the retro deliverable is absent at its registry name, so the veto must NOT fire",
			got, echoMarker)
	}
}

func TestC1147_005_callsites_carry_no_handrolled_report_literal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	forbidden := `+ "` + "-report.md" + `"`
	forbiddenNoSpace := `+"` + "-report.md" + `"`

	for _, rel := range []string{
		"go/internal/core/cyclerun_remediate.go",
		"go/internal/core/phase_bindings.go",
		"go/internal/cycleclassify/classify.go",
	} {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", rel, err)
			continue
		}
		src := string(data)
		if strings.Contains(src, forbidden) || strings.Contains(src, forbiddenNoSpace) {
			t.Errorf("%s still concatenates the %q convention by hand — "+
				"route it through phasecontract.ArtifactFilename so retro resolves to %q",
				rel, "-report.md", "retrospective-report.md")
		}
		if !strings.Contains(src, "phasecontract.ArtifactFilename(") {
			t.Errorf("%s does not call phasecontract.ArtifactFilename — "+
				"the SSOT must be the only declaration of the artifact-name vocabulary", rel)
		}
	}
}

func TestC1147_006_docsfloor_gate_regression(t *testing.T) {
	if stage := (policy.Policy{}).DocsFloorConfig().Stage; stage != "enforce" {
		t.Errorf("DocsFloorConfig().Stage = %q, want %q (compiled default must arm the floor)", stage, "enforce")
	}

	archOnly := []string{"go/internal/core/orchestrator.go"}
	archPlusDoc := []string{"go/internal/core/orchestrator.go", "docs/architecture/adr/0077-docs-floor-for-architecture-changes.md"}

	v := docsfloor.Evaluate(docsfloor.Config{Stage: "enforce"}, docsfloor.Input{
		ArchitectureLabeled: docsfloor.LabelArchitecture(archOnly),
		ChangedFiles:        archOnly,
	})
	if v.Status != docsfloor.StatusWarn {
		t.Errorf("Evaluate(undocumented arch change).Status = %q, want %q", v.Status, docsfloor.StatusWarn)
	}
	if strings.TrimSpace(v.Reason) == "" {
		t.Error("Evaluate(undocumented arch change).Reason is empty — an unexplained warning is unactionable")
	}

	v = docsfloor.Evaluate(docsfloor.Config{Stage: "enforce"}, docsfloor.Input{
		ArchitectureLabeled: docsfloor.LabelArchitecture(archPlusDoc),
		ChangedFiles:        archPlusDoc,
	})
	if v.Status != docsfloor.StatusPass {
		t.Errorf("Evaluate(documented arch change).Status = %q, want %q", v.Status, docsfloor.StatusPass)
	}

	v = docsfloor.Evaluate(docsfloor.Config{Stage: "off"}, docsfloor.Input{
		ArchitectureLabeled: true, ChangedFiles: archOnly,
	})
	if v.Status != docsfloor.StatusSkip {
		t.Errorf("Evaluate(stage=off).Status = %q, want %q — a disabled gate must not report a pass", v.Status, docsfloor.StatusSkip)
	}

	acsassert.FileExists(t, filepath.Join(acsassert.RepoRoot(t),
		"docs/architecture/adr/0077-docs-floor-for-architecture-changes.md"))
}

const echoMarker = "cycle1147 synthetic infra marker"

const echoExcerpt = "cycle1147 verbatim prompt sentence the agent echoed back"

func writeRetroEchoWorkspace(t *testing.T, deliverableName string) string {
	t.Helper()
	ws := t.TempDir()

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("retro-events.ndjson",
		`{"kind":"infra_failure","source":{"phase":"retro"},"data":{"marker":`+
			quote(echoMarker)+`,"excerpt":`+quote(echoExcerpt)+`}}`+"\n")
	write("retro-prompt.txt", "preamble\n"+echoExcerpt+"\ntrailer\n")
	write(deliverableName, "# Retrospective\n\n"+
		phasecontract.RenderVerdictSentinel("retro", "PASS")+"\n")
	write("llm-calls.ndjson", `{"phase":"retro","exit_code":0}`+"\n")

	return ws
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
