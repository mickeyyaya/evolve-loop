//go:build acs

package cycle1389

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func hasBadVerdict(res deliverable.Result) bool {
	for _, v := range res.Violations {
		if v.Code == deliverable.CodeBadVerdict {
			return true
		}
	}
	return false
}

const fencedJSONBadVerdictContent = "## Verdict\n" +
	"```json\n" +
	`{"phase":"audit","verdict":"PASS"}` + "\n" +
	"```\n"

func TestC1389_001_ClassifyBadVerdict_FencedJSON_Recoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", fencedJSONBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if !got.Recoverable {
		t.Errorf("want Recoverable=true for fenced-JSON content, got %+v", got)
	}
	if got.Pattern != deliverable.SalvagePatternFencedJSON {
		t.Errorf("want Pattern=%q, got %q (classification=%+v)", deliverable.SalvagePatternFencedJSON, got.Pattern, got)
	}
	if got.Reason == "" {
		t.Errorf("want a non-empty Reason (audit-visible log entry per coercion, README §3.3) — silent classification is not observability")
	}
}

const trailingCommaBadVerdictContent = "## Verdict\n" +
	`<!-- evolve-verdict: {"phase":"audit","verdict":"PASS",} -->` + "\n"

func TestC1389_002_ClassifyBadVerdict_TrailingComma_Recoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", trailingCommaBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if !got.Recoverable {
		t.Errorf("want Recoverable=true for trailing-comma content, got %+v", got)
	}
	if got.Pattern != deliverable.SalvagePatternTrailingComma {
		t.Errorf("want Pattern=%q, got %q (classification=%+v)", deliverable.SalvagePatternTrailingComma, got.Pattern, got)
	}
}

const displacedBadVerdictContent = "## Verdict\n" +
	"The agent's own reasoning trails off here, then states:\n" +
	`{"phase":"audit","verdict":"PASS"}` + "\n" +
	"...and nothing else follows.\n"

func TestC1389_003_ClassifyBadVerdict_Displaced_Recoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", displacedBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if !got.Recoverable {
		t.Errorf("want Recoverable=true for displaced bare-JSON content, got %+v", got)
	}
	if got.Pattern != deliverable.SalvagePatternDisplaced {
		t.Errorf("want Pattern=%q, got %q (classification=%+v)", deliverable.SalvagePatternDisplaced, got.Pattern, got)
	}
}

const absentBadVerdictContent = "## Verdict\n" +
	"inconclusive musings, no token, no structure of any kind here at all\n"

func TestC1389_004_ClassifyBadVerdict_GenuinelyAbsent_NotRecoverable(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", absentBadVerdictContent)
	res, err := deliverable.VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBadVerdict(res) {
		t.Fatalf("fixture must be a genuine bad_verdict at StageEnforce; got %+v", res.Violations)
	}

	got := deliverable.ClassifyBadVerdict(res.Content)
	if got.Recoverable {
		t.Errorf("want Recoverable=false for a genuinely absent verdict, got %+v — a classifier that always says recoverable is a no-op (SKILL §6 negative axis)", got)
	}
	if got.Pattern != deliverable.SalvagePatternNone {
		t.Errorf("want Pattern=%q (none) for not-recoverable, got %q", deliverable.SalvagePatternNone, got.Pattern)
	}
}

func TestC1389_005_ExistingDeliverableSuite_ZeroMutation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "./internal/deliverable")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./internal/deliverable/... must stay green (zero Result.OK/Violations mutation from the new classifier); err=%v\n%s", err, out)
	}
}

func TestC1389_006_ReviewerWiring_LogsClassificationOnBadVerdict(t *testing.T) {
	ws, pr := t.TempDir(), t.TempDir()
	writeFile(t, ws, "audit-report.md", trailingCommaBadVerdictContent)

	r := deliverable.NewReviewerWithCatalogStageReportSize(
		config.StageEnforce, phasespec.Catalog{}, config.StageEnforce, config.StageOff, 0)
	got := r.Review(context.Background(), core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: pr})

	if got.Approve {
		t.Fatalf("bad_verdict at StageEnforce must still BLOCK (instrumentation is observability-only, never a waiver); got Approve=true")
	}

	baselinePath := filepath.Join(pr, ".evolve", "bad-verdict-baseline.jsonl")
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("want a baseline JSONL record appended at %s by Reviewer.Review on bad_verdict (Task 2 wiring), got read error: %v", baselinePath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	last := lines[len(lines)-1]

	var rec map[string]any
	if err := json.Unmarshal([]byte(last), &rec); err != nil {
		t.Fatalf("baseline record is not valid JSON: %v\nline=%q", err, last)
	}
	if rec["phase"] != "audit" {
		t.Errorf("want phase=%q in the baseline record, got %+v", "audit", rec)
	}
	if rec["recoverable"] != true {
		t.Errorf("this fixture (trailing-comma) is classifier-recoverable; want recoverable=true in the baseline record, got %+v", rec)
	}
	if rec["pattern"] != string(deliverable.SalvagePatternTrailingComma) {
		t.Errorf("want pattern=%q in the baseline record, got %+v", deliverable.SalvagePatternTrailingComma, rec)
	}
}

const readmeRelPath = "docs/research/deliverable-alignment-2026-08/README.md"

var section7RE = regexp.MustCompile(`(?m)^## 7\.`)

func TestC1389_007_ReadmeGainsBaselineSectionWithWiringExcerpt(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, readmeRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", readmeRelPath, err)
	}
	content := string(data)

	if !section7RE.MatchString(content) {
		t.Errorf("%s has no top-level \"## 7.\" heading — append the baseline section (operating-policy 3.8 issue/gap/solution format)", readmeRelPath)
	}
	if !strings.Contains(content, "ClassifyBadVerdict") {
		t.Errorf("%s §7 must carry a wiring-proof excerpt naming ClassifyBadVerdict (the classifier actually invoked from Reviewer.Review, not a dead helper)", readmeRelPath)
	}
	if !strings.Contains(content, "bad-verdict-baseline.jsonl") {
		t.Errorf("%s §7 must name the baseline sidecar file the counts were pulled from", readmeRelPath)
	}
}
