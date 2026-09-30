//go:build acs

package cycle1441

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const salvageAppliedFile = "salvage-applied.jsonl"

const soleFencedPass = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

const multiViolationFenced = "## Summary\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n"

const ambiguousFenced = "## Verdict\n" +
	"```json\n" + `{"phase":"audit","verdict":"PASS"}` + "\n```\n" +
	"An earlier draft said:\n" +
	"```json\n" + `{"phase":"audit","verdict":"FAIL"}` + "\n```\n"

func reviewFixture(t *testing.T, content string) (string, string) {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write audit-report.md: %v", err)
	}
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	return ws, proj
}

func productionGate() core.DeliverableReviewer {
	return deliverable.NewReviewerWithCatalogStage(config.StageEnforce, phasespec.Catalog{}, config.StageEnforce)
}

func appliedRecords(t *testing.T, proj string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(proj, ".evolve", salvageAppliedFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read %s: %v", salvageAppliedFile, err)
	}
	var out []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("%s carries an unparseable record %q: %v", salvageAppliedFile, line, err)
		}
		if m["event_type"] == "salvage_applied" {
			out = append(out, m)
		}
	}
	return out
}

func TestC1441_001_ReviewSalvagesSoleRecoverableBadVerdict(t *testing.T) {
	ws, proj := reviewFixture(t, soleFencedPass)

	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if !got.Approve {
		t.Fatalf("contract gate BLOCKED a sole, unambiguous, recoverable bad_verdict (reason=%q) — the extraction "+
			"stage is not reached from Reviewer.Review. A salvage seam whose only caller is a test is dead code.", got.Reason)
	}
	if got.Demoted {
		t.Errorf("approval came from the breaker demotion path (Demoted=true, blocks=%d), not from salvage — a "+
			"demoted approval is the gate giving up, not the gate recovering a verdict", got.Blocks)
	}

	recs := appliedRecords(t, proj)
	if len(recs) != 1 {
		t.Fatalf("salvage-applied.jsonl holds %d salvage_applied record(s), want exactly 1 — every coercion must be "+
			"recorded for the operator (README §8), and exactly once", len(recs))
	}
	if got, want := recs[0]["pattern"], "fenced-json"; got != want {
		t.Errorf("recorded pattern = %v, want %q — the record must name the shape that was actually coerced", got, want)
	}
	if got, want := recs[0]["phase"], "audit"; got != want {
		t.Errorf("recorded phase = %v, want %q", got, want)
	}
}

func TestC1441_002_ReviewNeverSalvagesMultiViolation(t *testing.T) {
	ws, proj := reviewFixture(t, multiViolationFenced)

	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if got.Approve {
		t.Errorf("contract gate APPROVED a deliverable whose bad_verdict co-occurs with a missing required section — "+
			"salvage repairs the verdict and nothing else; approving here erases every other violation wholesale "+
			"(report-forgery bypass, cycle-1392 CRITICAL-1). reason=%q demoted=%v", got.Reason, got.Demoted)
	}
	if n := len(appliedRecords(t, proj)); n != 0 {
		t.Errorf("%d salvage_applied record(s) written for a multi-violation deliverable, want 0 — a refusal must "+
			"leave no coercion record", n)
	}
}

func TestC1441_003_ReviewRefusesAmbiguousCandidates(t *testing.T) {
	ws, proj := reviewFixture(t, ambiguousFenced)

	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if got.Approve {
		t.Errorf("contract gate APPROVED a deliverable carrying two disagreeing verdict candidates (PASS and FAIL) — "+
			"genuine ambiguity must be REFUSED, never resolved by picking a candidate. reason=%q demoted=%v",
			got.Reason, got.Demoted)
	}
	if n := len(appliedRecords(t, proj)); n != 0 {
		t.Errorf("%d salvage_applied record(s) written for an ambiguous deliverable, want 0", n)
	}
}

func TestC1441_004_SalvageVerdictFailsClosedOnUnresolvablePhase(t *testing.T) {
	in := deliverable.Result{
		Phase:        "no-such-phase-cycle1441",
		ArtifactPath: "/nonexistent/no-such-phase-report.md",
		Content:      soleFencedPass,
		Violations:   []deliverable.Violation{{Code: deliverable.CodeBadVerdict, Message: "seeded"}},
	}
	out, applied := deliverable.SalvageVerdict(in)
	if applied {
		t.Fatalf("SalvageVerdict claimed a salvage for a phase whose contract cannot be resolved — with no contract "+
			"there is nothing to re-verify the repaired bytes against, so the only safe answer is refusal (got OK=%v)", out.OK)
	}
	if out.OK {
		t.Errorf("refused salvage returned OK=true — a refusal must never approve")
	}
	if out.Content != in.Content || out.Phase != in.Phase || out.ArtifactPath != in.ArtifactPath ||
		len(out.Violations) != len(in.Violations) {
		t.Errorf("refused salvage mutated the Result\n got: %+v\nwant (byte-identical): %+v", out, in)
	}
}

func TestC1441_005_SalvageSummaryLineSurfacesRealSalvage(t *testing.T) {
	quiet := t.TempDir()
	if err := os.MkdirAll(filepath.Join(quiet, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if line := deliverable.SalvageSummaryLine(filepath.Join(quiet, ".evolve")); line != "" {
		t.Errorf("SalvageSummaryLine on an empty .evolve returned %q, want \"\" — zero salvages must render nothing", line)
	}

	ws, proj := reviewFixture(t, soleFencedPass)
	got := productionGate().Review(context.Background(), core.ReviewInput{
		Phase: "audit", Workspace: ws, ProjectRoot: proj,
	})
	if !got.Approve {
		t.Fatalf("precondition: a sole recoverable bad_verdict must salvage to Approve=true; got block (%s)", got.Reason)
	}

	line := deliverable.SalvageSummaryLine(filepath.Join(proj, ".evolve"))
	if want := "Salvaged verdicts: 1 (fenced-json=1)"; !strings.Contains(line, want) {
		t.Errorf("summary line must render the sidecar's real count and pattern breakdown\n want substring: %q\n got: %q",
			want, line)
	}
}

func TestC1441_006_PortedSalvageSuiteGreenAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", "Salvage", "./internal/deliverable")
	if err != nil || code != 0 {
		t.Fatalf("go test -run Salvage ./internal/deliverable: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}

	for _, rel := range []string{
		"go/internal/deliverable/salvage_extract.go",
		"go/internal/deliverable/salvage_extract_test.go",
		"go/internal/deliverable/reviewer_salvage_surface_test.go",
		"go/internal/deliverable/salvage_keycase_test.go",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Errorf("RED: %s missing — the ported extraction stage is incomplete", rel)
			continue
		}
		if _, _, c, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); c != 0 {
			t.Errorf("RED: %s is untracked — it will be dropped at ship", rel)
		}
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "evolve")
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve: exit=%d err=%v\n%s", code, err, stderr)
	}
	return bin
}

type savedReport struct {
	Total       int `json:"total"`
	Recoverable int `json:"recoverable"`
	Saved       int `json:"saved"`
	savedSeen   bool
}

func decodeSavedReport(t *testing.T, stdout string) savedReport {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", err, stdout)
	}
	var out savedReport
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("envelope does not decode: %v\n%s", err, stdout)
	}
	_, out.savedSeen = raw["saved"]
	return out
}

func TestC1441_007_SalvageReportExposesSavedCounter(t *testing.T) {
	bin := buildEvolve(t)

	proj := t.TempDir()
	evolveDir := filepath.Join(proj, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseline := strings.Join([]string{
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`,
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`,
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"trailing-comma"}`,
		`{"event_type":"bad_verdict_classified","recoverable":false,"pattern":""}`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(evolveDir, "bad-verdict-baseline.jsonl"), []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}
	applied := strings.Join([]string{
		`{"event_type":"salvage_applied","phase":"audit","pattern":"fenced-json","run":"111"}`,
		`{"event_type":"some_other_emitter","phase":"build","pattern":"fenced-json","run":"111"}`,
		``,
		`{"event_type":"salvage_applied","phase":"build","pattern":"trailing-comma","run":"222"}`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(evolveDir, salvageAppliedFile), []byte(applied), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if err != nil || code != 0 {
		t.Fatalf("evolve salvage report -json: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	got := decodeSavedReport(t, stdout)
	if !got.savedSeen {
		t.Fatalf("JSON envelope has no `saved` key — operators cannot tell measured POTENTIAL (recoverable) from "+
			"ACTUAL coercions. envelope:\n%s", stdout)
	}
	if got.Saved != 2 {
		t.Errorf("saved = %d, want 2 — count salvage_applied records only; the foreign emitter's line and the blank "+
			"lines must not enter the count", got.Saved)
	}
	if got.Recoverable != 3 {
		t.Errorf("recoverable = %d, want 3 — the existing baseline fold must not change", got.Recoverable)
	}
	if got.Saved == got.Recoverable {
		t.Errorf("saved == recoverable == %d on a fixture built to make them differ — the new counter must be read "+
			"from salvage-applied.jsonl, not aliased to the baseline's recoverable count", got.Saved)
	}
}

func TestC1441_008_SalvageReportSavedZeroWithoutAppliedSidecar(t *testing.T) {
	bin := buildEvolve(t)

	proj := t.TempDir()
	evolveDir := filepath.Join(proj, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "bad-verdict-baseline.jsonl"),
		[]byte(`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if err != nil || code != 0 {
		t.Fatalf("an absent salvage-applied.jsonl must not be an error: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
	got := decodeSavedReport(t, stdout)
	if !got.savedSeen {
		t.Fatalf("JSON envelope has no `saved` key even on the empty path — the key must always be present so "+
			"consumers need no special case. envelope:\n%s", stdout)
	}
	if got.Saved != 0 {
		t.Errorf("saved = %d with no salvage-applied.jsonl, want 0", got.Saved)
	}
	if got.Recoverable != 1 {
		t.Errorf("recoverable = %d, want 1 — the baseline fold must still run when the applied sidecar is absent",
			got.Recoverable)
	}
}
