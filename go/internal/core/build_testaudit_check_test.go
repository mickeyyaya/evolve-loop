package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	auditBaseModule  = "module example.com/m\n\ngo 1.23\n"
	auditBaseSource  = "package q\n\nfunc F() error { return nil }\n"
	auditCleanTest   = "package q\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F() != nil {\n\t\tt.Fatal(\"F failed\")\n\t}\n}\n"
	auditTextTest    = "package q\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestF(t *testing.T) {\n\tif err := F(); err != nil && !strings.Contains(err.Error(), \"x\") {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	auditRedTest     = "package q\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {\n\t_ = F()\n}\n"
	auditCentralRow  = `{"packages":["a"],"rows":[{"package":"a","test":"TestNothing","rule":"no-assertion"}]}`
	auditFakesJSON   = `{"rows":[]}`
	auditGuardsJSON  = `{"max_rows":1,"rows":[]}`
	auditNewRowHint  = "remove the weak pattern"
	auditStaleRowFix = "evolve test audit --rebaseline"
)

func seedTestAuditConfig(t *testing.T, worktree string) {
	t.Helper()
	writeFile(t, filepath.Join(worktree, "go", "internal", "testaudit", "fakes.json"), auditFakesJSON)
	writeFile(t, filepath.Join(worktree, "go", "internal", "testaudit", "guards.json"), auditGuardsJSON)
}

func auditRepo(t *testing.T, files map[string]string) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), auditBaseModule)
	seedTestAuditConfig(t, repo.Dir)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), auditBaseSource)
	for rel, body := range files {
		writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(rel)), body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo, repo.Git("rev-parse", "HEAD")
}

func auditInput(t *testing.T, repo *gittest.Repo, base string) ReviewInput {
	t.Helper()
	return floorOffInput(t, ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base, Stderr: &strings.Builder{}})
}

func auditLines(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, testAuditFloorPrefix) {
			out = append(out, l)
		}
	}
	return out
}

func TestStaticBuildFloorChecks_RefusesANewTestAuditFinding(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), auditTextTest)

	got := auditLines(StaticBuildFloorChecks(context.Background(), auditInput(t, repo, base)))

	if len(got) != 1 {
		t.Fatalf("test-audit lines = %q, want exactly one", got)
	}
	for _, want := range []string{"q", "TestF", "error-text-match", auditNewRowHint} {
		if !strings.Contains(got[0], want) {
			t.Errorf("line %q lacks %q", got[0], want)
		}
	}
}

func TestStaticBuildFloorChecks_PassesAChangedTestWithNoNewFinding(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), auditCleanTest)

	if got := auditLines(StaticBuildFloorChecks(context.Background(), auditInput(t, repo, base))); len(got) != 0 {
		t.Fatalf("test-audit lines = %q, want none", got)
	}
}

func TestStaticBuildFloorChecks_RefusesAStaleRowOfADeletedPackage(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, map[string]string{
		"go/internal/testaudit/unwritable-baselines.json": auditCentralRow,
		"go/a/a.go":      auditBaseSource,
		"go/a/a_test.go": auditRedTest,
	})
	repo.Git("rm", "-q", "-r", "go/a")

	got := auditLines(StaticBuildFloorChecks(context.Background(), auditInput(t, repo, base)))

	if len(got) != 1 || !strings.Contains(got[0], "TestNothing") || !strings.Contains(got[0], auditStaleRowFix+" a") {
		t.Fatalf("test-audit lines = %q, want one stale-row line for package a", got)
	}
}

func TestStaticBuildFloorChecks_RefusesAStaleRowOfAMovedPackage(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, map[string]string{
		"go/internal/testaudit/unwritable-baselines.json": auditCentralRow,
		"go/a/a.go":      auditBaseSource,
		"go/a/a_test.go": auditRedTest,
	})
	repo.Git("mv", "go/a", "go/b")

	got := auditLines(StaticBuildFloorChecks(context.Background(), auditInput(t, repo, base)))

	var stale, added int
	for _, l := range got {
		if strings.Contains(l, auditStaleRowFix+" a") {
			stale++
		}
		if strings.Contains(l, auditNewRowHint) {
			added++
		}
	}
	if stale != 1 || added != 1 {
		t.Fatalf("test-audit lines = %q, want one stale row for a and one new finding for b", got)
	}
}

func TestStaticBuildFloorChecks_IgnoresAChangedTestdataFixture(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "testdata", "fx", "fx_test.go"), auditRedTest)

	if got := auditLines(StaticBuildFloorChecks(context.Background(), auditInput(t, repo, base))); len(got) != 0 {
		t.Fatalf("test-audit lines = %q, want a testdata fixture ignored", got)
	}
}

func TestStaticBuildFloorChecks_IgnoresAChangeWithNoGoPackage(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "docs", "note.md"), "# Note\n")

	if got := auditLines(StaticBuildFloorChecks(context.Background(), auditInput(t, repo, base))); len(got) != 0 {
		t.Fatalf("test-audit lines = %q, want none", got)
	}
}

func TestDefaultBuildFloorChecks_IncludesTheTestAuditCheck(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), auditTextTest)

	got := auditLines(DefaultBuildFloorChecks(context.Background(), auditInput(t, repo, base)))

	if len(got) != 1 {
		t.Fatalf("test-audit lines = %q, want one from the default floor", got)
	}
}

func TestTestAuditFailures_NoWorktreeIsSilent(t *testing.T) {
	t.Parallel()
	if got := testAuditFailures(context.Background(), ReviewInput{}); got != nil {
		t.Fatalf("lines = %q, want nil", got)
	}
}

func TestTestAuditFailures_AGitErrorFailsClosedWithOneLine(t *testing.T) {
	t.Parallel()
	got := testAuditFailures(context.Background(), ReviewInput{Worktree: t.TempDir(), WorktreeBaseSHA: "deadbeef"})
	if len(got) != 1 || !strings.HasPrefix(got[0], testAuditFloorPrefix) || !strings.Contains(got[0], "changed paths") {
		t.Fatalf("lines = %q, want one fail-closed line", got)
	}
}

func TestTestAuditFailures_ARepoWithoutAuditConfigIsSilent(t *testing.T) {
	t.Parallel()
	repo, _ := auditRepo(t, nil)
	repo.Git("rm", "-q", "-r", "go/internal/testaudit")
	repo.Git("commit", "-q", "-m", "drop the audit")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), auditTextTest)

	if got := testAuditFailures(context.Background(), auditInput(t, repo, base)); got != nil {
		t.Fatalf("lines = %q, want a repo without the audit skipped", got)
	}
}

func TestTestAuditFailures_ABadConfigOfTheBaseIsOneFloorLine(t *testing.T) {
	t.Parallel()
	repo, _ := auditRepo(t, map[string]string{"go/internal/testaudit/guards.json": "{"})
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), auditCleanTest)

	got := testAuditFailures(context.Background(), auditInput(t, repo, base))

	if len(got) != 1 || !strings.Contains(got[0], "config") {
		t.Fatalf("lines = %q, want one config line", got)
	}
}

func TestTestAuditFailures_ABrokenTestFileIsReportedAsItself(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q_test.go"), "package q\nfunc {")

	got := testAuditFailures(context.Background(), auditInput(t, repo, base))

	if len(got) != 1 || strings.Contains(got[0], auditNewRowHint) || strings.Contains(got[0], auditStaleRowFix) {
		t.Fatalf("lines = %q, want one plain line with no row hint", got)
	}
}

func TestTestAuditFailures_ARuleChangeRunsTheAuditPackageInTheWorktree(t *testing.T) {
	repo, base := auditRepo(t, nil)
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "testaudit", "guards.json"), `{"max_rows":2,"rows":[]}`)

	ran := stubSelfCheckRunner(t, "--- FAIL: TestTreeRatchet\n", false)
	got := testAuditFailures(context.Background(), auditInput(t, repo, base))

	if len(*ran) != 1 || (*ran)[0] != testAuditPkg {
		t.Fatalf("runs = %v, want only %q", *ran, testAuditPkg)
	}
	if len(got) != 1 || !strings.Contains(got[0], "TestTreeRatchet") {
		t.Fatalf("lines = %q, want one line with the output head", got)
	}

	stubSelfCheckRunner(t, "ok\n", true)
	if got := testAuditFailures(context.Background(), auditInput(t, repo, base)); len(got) != 0 {
		t.Fatalf("lines = %q, want none for a green audit package", got)
	}
}
