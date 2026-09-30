//go:build acs

package cycle8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func binPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "bin", "evolve")
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func minimalValidPlanJSON(t *testing.T) []byte {
	t.Helper()
	type todo struct {
		ID string `json:"id"`
	}
	type plan struct {
		Version int    `json:"version"`
		Goal    string `json:"goal"`
		Cycles  []todo `json:"cycles"`
	}
	data, err := json.Marshal(plan{
		Version: 1,
		Goal:    "test campaign goal for predicate",
		Cycles:  []todo{{ID: "c1"}},
	})
	if err != nil {
		t.Fatalf("minimalValidPlanJSON: %v", err)
	}
	return data
}

func runEvolve(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := binPath(t)
	quotedArgs := make([]string, len(args))
	for i, a := range args {
		quotedArgs[i] = "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
	}
	cmd := "cd " + "'" + root + "'" + " && " + "'" + bin + "'" + " " + strings.Join(quotedArgs, " ")
	out, errOut, c, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	return strings.TrimSpace(out + "\n" + errOut), c
}

func TestC1_001_CampaignEntryInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	registryPath := filepath.Join(root, "go", "cmd", "evolve", "registry.go")

	// acs-predicate: config-check
	if !acsassert.FileContains(t, registryPath, `"campaign"`) {
		t.Errorf("RED: registry.go missing \"campaign\" entry.\n"+
			"Builder must add {Name: \"campaign\", ...} row to go/cmd/evolve/registry.go.\n"+
			"File: %s", registryPath)
	}

	combined, _ := runEvolve(t, "campaign")
	if strings.Contains(combined, "unknown command") {
		t.Errorf("RED: `evolve campaign` printed \"unknown command\" — command is not registered.\n"+
			"Builder must add the campaign entry to registry.go.\nOutput:\n%s", combined)
	}
}

func TestC1_002_NoArgsExitsNonZeroWithCampaignUsage(t *testing.T) {
	combined, code := runEvolve(t, "campaign")
	if code == 0 {
		t.Errorf("RED: `evolve campaign` (no args) exited 0 — expected non-zero (usage error).\n"+
			"Output:\n%s", combined)
	}
	if !strings.Contains(combined, "study") {
		t.Errorf("RED: `evolve campaign` (no args) output does not mention \"study\".\n"+
			"Expected campaign-specific usage message with subcommand names; got:\n%s\n"+
			"Builder must implement cmd_campaign.go with proper usage output.", combined)
	}
}

func TestC1_003_StudySubcommandAccessible(t *testing.T) {
	combined, _ := runEvolve(t, "campaign", "study")
	if strings.Contains(combined, "unknown command") {
		t.Errorf("RED: `evolve campaign study` printed \"unknown command\" — "+
			"campaign command is not registered.\n"+
			"Builder must implement go/cmd/evolve/cmd_campaign.go with a study subcommand.\n"+
			"Output:\n%s", combined)
	}
}

func TestC1_004_ReplanSubcommandAccessible(t *testing.T) {
	combined, _ := runEvolve(t, "campaign", "replan", "--feedback", "some feedback text")
	if strings.Contains(combined, "unknown command") {
		t.Errorf("RED: `evolve campaign replan --feedback ...` printed \"unknown command\" — "+
			"campaign replan subcommand is not registered.\n"+
			"Builder must implement the replan subcommand in cmd_campaign.go.\n"+
			"Output:\n%s", combined)
	}
}

func TestC1_005_RunSimulateExitsZero(t *testing.T) {
	planData := minimalValidPlanJSON(t)
	planFile := filepath.Join(t.TempDir(), "campaign-plan.json")
	if err := os.WriteFile(planFile, planData, 0o644); err != nil {
		t.Fatalf("write plan fixture: %v", err)
	}
	repo := acsassert.RepoRoot(t)
	scratch := scratchWorktree(t, repo)
	headBefore := gitHeadAndStatus(t, repo)

	combined, code := runEvolve(t, "campaign", "run", "--plan", planFile, "--simulate", "--project-root", scratch)
	if headAfter := gitHeadAndStatus(t, repo); headAfter != headBefore {
		t.Errorf("RED: the simulate walk mutated the checkout (HEAD/status before vs after):\n%s\n---\n%s", headBefore, headAfter)
	}
	if entries, _ := os.ReadDir(filepath.Join(scratch, ".evolve")); len(entries) == 0 {
		t.Errorf("the walk must run in the scratch root (no .evolve written under %s)", scratch)
	}
	if code != 0 {
		t.Errorf("RED: `evolve campaign run --plan <valid> --simulate` exited %d.\n"+
			"Builder must implement cmd_campaign.go: load plan, verify, iterate waves with "+
			"--simulate flag passed to execCycleLaunch.\n"+
			"Output:\n%s", code, combined)
	}
}

func TestC1_005neg_RunWithInvalidPlanExitsNonZero(t *testing.T) {
	combined, code := runEvolve(t, "campaign", "run", "--plan", "/dev/null", "--simulate", "--project-root", t.TempDir())
	if code == 0 {
		t.Errorf("FAIL: `evolve campaign run --plan /dev/null --simulate` exited 0.\n"+
			"An empty/invalid plan must be rejected by campaign.Load/campaign.Verify.\n"+
			"Output:\n%s", combined)
	}
}

func TestC1_006_BuildAndVetPass(t *testing.T) {
	gd := goDir(t)

	buildOut, buildErr, buildCode, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", gd, "./cmd/evolve/...",
	)
	if buildCode != 0 {
		t.Errorf("RED: `go build ./cmd/evolve/...` failed (exit=%d).\n"+
			"Builder's cmd_campaign.go must compile cleanly.\n"+
			"stdout: %s\nstderr: %s", buildCode, buildOut, buildErr)
	}

	vetOut, vetErr, vetCode, _ := acsassert.SubprocessOutput(
		"go", "vet", "-C", gd, "./cmd/evolve/...",
	)
	if vetCode != 0 {
		t.Errorf("RED: `go vet ./cmd/evolve/...` failed (exit=%d).\n"+
			"Builder's cmd_campaign.go must pass go vet.\n"+
			"stdout: %s\nstderr: %s", vetCode, vetOut, vetErr)
	}
}

func TestC2_001_ADRFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk.\n"+
			"Builder must create docs/architecture/adr/0056-advisor-driven-preliminary-study-cycle.md.", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if code != 0 {
		t.Errorf("RED: %s exists on disk but is not git-tracked — may be gitignored (dropped at ship).\n"+
			"Builder must `git add` the ADR file.", rel)
	}
}

func TestC2_002_ADRHasRequiredSections(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	for _, section := range []string{"## Status", "## Context", "## Decision", "## Consequences"} {
		if !acsassert.FileContains(t, adrPath, section) {
			t.Errorf("RED: ADR-0056 is missing required section %q.\n"+
				"Builder must include all four standard ADR sections.\nFile: %s", section, adrPath)
		}
	}
}

func TestC2_003_ADRDescribesAllFourSlices(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	sliceMarkers := []struct {
		slice   string
		keyword string
	}{
		{"S1 (wave engine)", "dag.Levels"},
		{"S2 (preliminary-study phase)", "preliminary-study"},
		{"S3 (CLI driver)", "cmd_campaign"},
		{"S4 (documentation)", "S4"},
	}
	for _, m := range sliceMarkers {
		if !acsassert.FileContains(t, adrPath, m.keyword) {
			t.Errorf("RED: ADR-0056 does not mention %q (required keyword for %s).\n"+
				"Builder must describe all four slices in the ADR.\nFile: %s",
				m.keyword, m.slice, adrPath)
		}
	}
}

func TestC2_004_CitationsFileExistsAndNonEmpty(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "architecture", "campaign-planning-citations.md")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk.\n"+
			"Builder must create docs/architecture/campaign-planning-citations.md.", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if code != 0 {
		t.Errorf("RED: %s exists but is not git-tracked.\nBuilder must `git add` the citations file.", rel)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	lineCount := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lineCount++
		}
	}
	if lineCount < 5 {
		t.Errorf("RED: %s has only %d non-empty line(s); need ≥ 5.\n"+
			"Builder must populate the citations file with meaningful content.", rel, lineCount)
	}
}

func TestC2_005_NoPlaceholderURLsInADR(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	if !acsassert.FileNotContains(t, adrPath, "example.com") {
		t.Errorf("RED: ADR-0056 contains placeholder URL \"example.com\".\n"+
			"Builder must replace all placeholder URLs with verified-live citations.\nFile: %s", adrPath)
	}
	if !acsassert.FileNotContains(t, adrPath, "TODO") {
		t.Errorf("RED: ADR-0056 contains \"TODO\" — unresolved placeholder.\n"+
			"Builder must replace all TODO-tagged content with verified information.\nFile: %s", adrPath)
	}
}

func gitHeadAndStatus(t *testing.T, repo string) string {
	t.Helper()
	head, errOut, code, err := acsassert.SubprocessOutput("git", "-C", repo, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		t.Fatalf("git rev-parse HEAD in %s: exit %d %v: %s", repo, code, err, errOut)
	}
	status, errOut, code, err := acsassert.SubprocessOutput("git", "-C", repo, "status", "--porcelain")
	if err != nil || code != 0 {
		t.Fatalf("git status in %s: exit %d %v: %s", repo, code, err, errOut)
	}
	branches, errOut, code, err := acsassert.SubprocessOutput("git", "-C", repo, "branch", "--list", "cycle-*")
	if err != nil || code != 0 {
		t.Fatalf("git branch --list in %s: exit %d %v: %s", repo, code, err, errOut)
	}
	worktrees, errOut, code, err := acsassert.SubprocessOutput("git", "-C", repo, "worktree", "list", "--porcelain")
	if err != nil || code != 0 {
		t.Fatalf("git worktree list in %s: exit %d %v: %s", repo, code, err, errOut)
	}
	return strings.Join([]string{strings.TrimSpace(head), strings.TrimSpace(status), strings.TrimSpace(branches), strings.TrimSpace(worktrees)}, "\n")
}

func scratchWorktree(t *testing.T, repo string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "scratch")
	acsassert.SubprocessOutput("git", "-C", repo, "worktree", "prune")
	if _, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", repo, "worktree", "add", "--detach", "-q", dir, "HEAD"); code != 0 {
		t.Fatalf("scratch worktree: %s", errOut)
	}
	t.Cleanup(func() {
		acsassert.SubprocessOutput("git", "-C", repo, "worktree", "remove", "--force", dir)
		acsassert.SubprocessOutput("git", "-C", repo, "worktree", "prune")
	})
	return dir
}
