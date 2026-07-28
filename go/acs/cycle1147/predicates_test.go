//go:build acs

// Package cycle1147 materialises the cycle-1147 acceptance criteria for the two
// tasks triage committed to THIS cycle:
//
//   - artifact-name-ssot-remaining-callsites → add phasecontract.ArtifactFilename
//     and route the three remaining hand-rolled `<phase>+"-report.md"` call
//     sites (core/cyclerun_remediate.go:81, core/phase_bindings.go:254,
//     cycleclassify/classify.go:446) through it, fixing the retro-phase path
//     mismatch (registry says "retrospective-report.md", the literal says
//     "retro-report.md").
//   - docs-floor-architecture-change-gate → PRE-EXISTING GREEN, see 006.
//
// The third fleet-scoped id (required-roles-ssot) was DROPPED by triage
// (already implemented; contract_registry.go:250-271 + required_ssot_test.go)
// and therefore carries ZERO predicates here — R9.3: predicates bind only to
// triage-committed work, and a predicate gating dropped/deferred work starves
// the committed task (the cycle-280 failure mode).
//
// Predicate strategy. The defect is a vocabulary duplication whose ONLY
// observable divergence is the retro phase, so the predicates attack it on the
// two axes that can actually see it:
//
//   - 001/002 are BEHAVIORAL over the new SSOT helper itself: they call
//     phasecontract.ArtifactFilename and assert its return value. 001 pins the
//     divergent phase (retro), 002 pins the fallback + NoArtifact edges. Both
//     red-fail today at COMPILE time — the helper does not exist.
//   - 003 is BEHAVIORAL end-to-end through the exported cycleclassify.Classify
//     over a synthetic workspace: the prompt-echo veto must find the retro
//     deliverable at its REGISTRY name. It red-fails today because
//     classify.go:446 reads "retro-report.md", which never exists.
//   - 004 is 003's negative twin and the anti-gaming half: with the deliverable
//     present ONLY at the legacy "retro-report.md" name, the veto must NOT
//     fire. Without it, a builder could green 003 by reading both names (or by
//     failing open), which would preserve the very duplication the task
//     removes.
//   - 005 is the duplication-ABSENCE check over the three named call sites.
//     Duplication is inherently a source-level property — no runtime
//     observation can distinguish three copies of an equal string from one
//     shared call — so this is the sanctioned absence-check form
//     (go/acs/README.md), and it is load-bearing only in company with 001,
//     which fails if the SSOT declaration is deleted or renamed to green it.
//   - 006 is the docs-floor task's REGRESSION predicate. That task's
//     implementation is already in-tree (see the AC-Materialization table in
//     test-report.md); 006 is behavioral and pins the gate's decision table and
//     its policy-injected default so the pre-existing GREEN cannot silently rot.
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

// TestC1147_001_artifact_filename_resolves_registry_name is the crux predicate.
// The registry declares retro's deliverable as "retrospective-report.md"
// (contract_registry.go:177); every consumer that hand-rolls `phase+"-report.md"`
// silently addresses a file that will never exist. ArtifactFilename is the one
// declaration site those consumers must route through.
//
// Behavioral: calls the helper and asserts its return value. Cannot be greened
// by adding a magic string anywhere — the function must exist and resolve
// through the registry.
func TestC1147_001_artifact_filename_resolves_registry_name(t *testing.T) {
	cases := []struct {
		phase string
		want  string
	}{
		// The divergent phase — the entire reason this task exists.
		{"retro", "retrospective-report.md"},
		// build-planner also diverges from the convention.
		{"build-planner", "build-plan.md"},
		// Phases whose registry name happens to match the convention must
		// keep resolving identically, so the swap is behaviour-preserving
		// everywhere except retro.
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

// TestC1147_002_artifact_filename_falls_back_on_convention pins the edges the
// helper inherits from core.backfillArtifactPath (routing_dispatch.go:376-379),
// the one call site that already got this right. An unregistered phase and a
// NoArtifact phase (ship, whose deliverable is a pushed commit) must both fall
// back to the "<phase>-report.md" convention rather than returning "" — the
// callers being migrated join this onto a workspace path, and an empty
// filename would silently address the workspace DIRECTORY.
//
// Edge/OOD axis: unregistered, NoArtifact, and empty input.
func TestC1147_002_artifact_filename_falls_back_on_convention(t *testing.T) {
	// NoArtifact phase: ArtifactName returns "", so the helper must supply
	// the convention instead of propagating the empty string.
	if got := phasecontract.ArtifactFilename("ship"); got != "ship-report.md" {
		t.Errorf("ArtifactFilename(\"ship\") = %q, want %q (NoArtifact ⇒ convention fallback)", got, "ship-report.md")
	}
	// Unregistered phase (a user-minted phase not in the registry).
	const unregistered = "cycle1147-not-a-registered-phase"
	if got := phasecontract.ArtifactFilename(unregistered); got != unregistered+"-report.md" {
		t.Errorf("ArtifactFilename(%q) = %q, want %q", unregistered, got, unregistered+"-report.md")
	}
	// Empty phase: the helper must stay total — return SOMETHING rather than
	// panicking or yielding "", either of which would make a caller's
	// filepath.Join resolve to the workspace directory itself. The exact
	// value is deliberately unconstrained: no migrated call site can produce
	// an empty phase (all three pass a state-machine Phase, and
	// classify.go:437 guards `phase == ""` before this point), so pinning a
	// specific sentinel here would be a requirement the task never asked for.
	if got := phasecontract.ArtifactFilename(""); got == "" {
		t.Error("ArtifactFilename(\"\") returned \"\" — a caller joining this onto the workspace path would address the directory itself")
	}
}

// TestC1147_003_classify_prompt_echo_veto_finds_retro_deliverable drives the
// REAL production path end-to-end through the exported cycleclassify.Classify.
//
// The cycle-641/642 prompt-echo veto (classify.go:435 isPromptEchoSelfReport)
// suppresses a bogus infra_failure when an agent merely quoted its own prompt
// on a phase that PASSed and exited 0. Its condition (2) reads the phase's
// deliverable — at `phase+"-report.md"`. For retro that path never exists, so
// the veto can never fire and a retro prompt-echo is permanently misclassified
// as an infrastructure failure.
//
// This fixture satisfies all three veto conditions with the deliverable at its
// REGISTRY name. RED today (veto misses ⇒ ClassInfrastructure); GREEN once
// classify.go:446 resolves through the SSOT.
func TestC1147_003_classify_prompt_echo_veto_finds_retro_deliverable(t *testing.T) {
	ws := writeRetroEchoWorkspace(t, phasecontract.ArtifactFilename("retro"))

	got := cycleclassify.Classify(ws)
	if got.Class == cycleclassify.ClassInfrastructure && got.Marker == echoMarker {
		t.Errorf("Classify(retro prompt-echo workspace) = %+v; want the echo VETOED "+
			"(deliverable present at the registry name %q) — the veto is reading %q instead",
			got, phasecontract.ArtifactFilename("retro"), "retro-report.md")
	}
}

// TestC1147_004_classify_veto_declines_on_legacy_only_name is 003's negative
// twin and the anti-no-op signal. With the deliverable present ONLY at the
// legacy convention name (and absent at the registry name), the veto must NOT
// fire: the phase's real contracted deliverable is missing, so a clean exit is
// unproven and the infra signal must survive.
//
// This is what stops the fix from being "read both names" or "fail open" —
// either would green 003 while leaving two vocabularies in the tree.
func TestC1147_004_classify_veto_declines_on_legacy_only_name(t *testing.T) {
	ws := writeRetroEchoWorkspace(t, "retro-report.md")

	got := cycleclassify.Classify(ws)
	if got.Class != cycleclassify.ClassInfrastructure || got.Marker != echoMarker {
		t.Errorf("Classify(legacy-only-name workspace) = %+v; want ClassInfrastructure/%q — "+
			"the retro deliverable is absent at its registry name, so the veto must NOT fire",
			got, echoMarker)
	}
}

// TestC1147_005_callsites_carry_no_handrolled_report_literal is the
// duplication-absence half. Each of the three migrated files must construct its
// artifact filename by CALLING the SSOT, never by concatenating the convention
// itself. Paired with 001: deleting or renaming ArtifactFilename to satisfy
// this check fails 001, so the two cannot be gamed together.
func TestC1147_005_callsites_carry_no_handrolled_report_literal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// The concatenation fragment, assembled at runtime so this predicate file
	// does not itself contain the literal it forbids (a self-trip).
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

// TestC1147_006_docsfloor_gate_regression is the docs-floor task's regression
// predicate. Unlike 001-005 this is PRE-EXISTING GREEN: the gate, its policy
// dial, its consumers and ADR-0077 all landed already (see test-report.md's
// AC-Materialization table for the evidence). It is authored anyway so the
// committed task carries a binding predicate that fails loudly if the gate
// regresses inside this cycle.
//
// Behavioral: exercises docsfloor.Evaluate's real decision table plus the
// policy-injected compiled default.
func TestC1147_006_docsfloor_gate_regression(t *testing.T) {
	// The compiled default must arm the gate with no policy block present
	// (config-injected, never a Go literal at the call site).
	if stage := (policy.Policy{}).DocsFloorConfig().Stage; stage != "enforce" {
		t.Errorf("DocsFloorConfig().Stage = %q, want %q (compiled default must arm the floor)", stage, "enforce")
	}

	archOnly := []string{"go/internal/core/orchestrator.go"}
	archPlusDoc := []string{"go/internal/core/orchestrator.go", "docs/architecture/adr/0077-docs-floor-for-architecture-changes.md"}

	// Undocumented architecture change ⇒ WARN with a non-empty reason.
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

	// Paired doc ⇒ PASS.
	v = docsfloor.Evaluate(docsfloor.Config{Stage: "enforce"}, docsfloor.Input{
		ArchitectureLabeled: docsfloor.LabelArchitecture(archPlusDoc),
		ChangedFiles:        archPlusDoc,
	})
	if v.Status != docsfloor.StatusPass {
		t.Errorf("Evaluate(documented arch change).Status = %q, want %q", v.Status, docsfloor.StatusPass)
	}

	// Stage off ⇒ SKIP, never a vacuous PASS.
	v = docsfloor.Evaluate(docsfloor.Config{Stage: "off"}, docsfloor.Input{
		ArchitectureLabeled: true, ChangedFiles: archOnly,
	})
	if v.Status != docsfloor.StatusSkip {
		t.Errorf("Evaluate(stage=off).Status = %q, want %q — a disabled gate must not report a pass", v.Status, docsfloor.StatusSkip)
	}

	// The gate's design doc is part of the task's deliverable.
	acsassert.FileExists(t, filepath.Join(acsassert.RepoRoot(t),
		"docs/architecture/adr/0077-docs-floor-for-architecture-changes.md"))
}

// echoMarker is the infra marker the 003/004 fixture emits. Distinct enough
// that a match can only have come from this fixture's event line.
const echoMarker = "cycle1147 synthetic infra marker"

// echoExcerpt is the text the fixture's agent "quoted" from its own prompt —
// veto condition (1) requires it to be a verbatim substring of the prompt file.
const echoExcerpt = "cycle1147 verbatim prompt sentence the agent echoed back"

// writeRetroEchoWorkspace builds a workspace that satisfies all three
// prompt-echo veto conditions for the retro phase, with the deliverable written
// at deliverableName. 003 passes the registry name, 004 the legacy convention
// name — the ONLY difference between the two fixtures.
func writeRetroEchoWorkspace(t *testing.T, deliverableName string) string {
	t.Helper()
	ws := t.TempDir()

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// The infra_failure event the veto is meant to suppress.
	write("retro-events.ndjson",
		`{"kind":"infra_failure","source":{"phase":"retro"},"data":{"marker":`+
			quote(echoMarker)+`,"excerpt":`+quote(echoExcerpt)+`}}`+"\n")
	// Veto condition (1): the excerpt is verbatim in the phase's prompt.
	write("retro-prompt.txt", "preamble\n"+echoExcerpt+"\ntrailer\n")
	// Veto condition (2): the phase's deliverable declares PASS.
	write(deliverableName, "# Retrospective\n\n"+
		phasecontract.RenderVerdictSentinel("retro", "PASS")+"\n")
	// Veto condition (3): the driver exited 0.
	write("llm-calls.ndjson", `{"phase":"retro","exit_code":0}`+"\n")

	return ws
}

// quote renders s as a JSON string literal for the fixture's NDJSON lines.
func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
