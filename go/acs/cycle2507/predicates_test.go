//go:build acs

package cycle2507

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePath = "github.com/mickeyyaya/evolve-loop/go/"

var batchPackages = []string{
	"internal/campaign",
	"internal/codequality",
	"internal/fleet",
	"internal/gc",
	"internal/inboxbatch",
	"internal/inboxcurator",
}

var oldWordingFragments = []string{
	"failed after %d attempt",
	"could not read the tree",
	"commit them via",
	"could not be pinned",
	"would rank below every class",
	"explain the console route",
}

func lintOptions(t *testing.T) stelint.Options {
	t.Helper()
	words, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	if len(words) == 0 {
		t.Fatalf("the STE standard has an empty word table, so no lint result can be trusted")
	}
	return stelint.Options{Words: words}
}

func packageSources(t *testing.T, pkg string, wantTests bool) []string {
	t.Helper()
	dir := filepath.Join(acsassert.GoModuleDir(t), filepath.FromSlash(pkg))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") != wantTests {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	if len(out) == 0 {
		t.Fatalf("no go files found for %s", pkg)
	}
	return out
}

func TestC2507_001_EachBatchPackageIsCleanUnderStrictGoLint(t *testing.T) {
	opt := lintOptions(t)
	root := acsassert.RepoRoot(t)
	for _, pkg := range batchPackages {
		t.Run(pkg, func(t *testing.T) {
			for _, path := range packageSources(t, pkg, false) {
				rel, err := filepath.Rel(root, path)
				if err != nil || !stelint.InGoScope(rel) {
					continue
				}
				findings, err := stelint.LintFile(path, opt)
				if err != nil {
					t.Fatalf("lint %s: %v", rel, err)
				}
				for _, f := range findings {
					if f.IsSkipNotice() {
						continue
					}
					t.Errorf("%s:%d %s %s", rel, f.Line, f.Rule, f.Message)
				}
			}
		})
	}
}

func TestC2507_002_TheLinterStillFlagsTheOldWording(t *testing.T) {
	opt := lintOptions(t)
	oldMessages := map[string]string{
		"attempt":  `package p; import "fmt"; var _ = fmt.Errorf("cycle(s) %v failed after %d attempt(s)", 1, 2)`,
		"could":    `package p; import "fmt"; var _ = fmt.Errorf("gofmt could not read the tree")`,
		"via":      `package p; import "fmt"; var _ = fmt.Errorf("commit them via the console")`,
		"would":    `package p; import "errors"; var _ = errors.New("the item would rank below every class")`,
		"sentence": `package p; import "errors"; var _ = errors.New("no protected fix surface and no pipeline kind explain the console route; give the reason (--reason), or drop the route so a lane can take the item")`,
	}
	for name, src := range oldMessages {
		findings, err := stelint.CheckGo([]byte(src), opt)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(findings) == 0 {
			t.Errorf("%s: the linter accepts the old wording, so criterion 1 proves nothing", name)
		}
	}
}

func TestC2507_003_NoBatchTestPinsTheOldWording(t *testing.T) {
	for _, pkg := range batchPackages {
		for _, path := range packageSources(t, pkg, true) {
			for _, fragment := range oldWordingFragments {
				if !acsassert.FileNotContains(t, path, fragment) {
					t.Errorf("%s still pins the old wording %q: change the test in the same PR", path, fragment)
				}
			}
		}
	}
}

func TestC2507_004_TheGcPinTestFollowsTheRewrittenMessage(t *testing.T) {
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "test", "-count=1", "-v",
		"-run", "TestApplyDevWorktrees_ACheckpointThatCannotBeWrittenKeepsTheTree",
		modulePath+"internal/gc")
	combined := out + "\n" + errOut
	if code != 0 {
		t.Fatalf("the gc pin test is red (exit=%d):\n%s", code, acsassert.OutputTail(combined, 20))
	}
	if !strings.Contains(combined, "--- PASS: TestApplyDevWorktrees_ACheckpointThatCannotBeWrittenKeepsTheTree") {
		t.Errorf("the gc pin test did not run:\n%s", acsassert.OutputTail(combined, 20))
	}
}

func TestC2507_005_SentinelsAndTechnicalTokensSurviveTheRewrite(t *testing.T) {
	t.Run("inboxbatch sentinel", func(t *testing.T) {
		err := inboxbatch.RequirePriorityClass("  ")
		if !errors.Is(err, inboxbatch.ErrNoPriorityClass) {
			t.Fatalf("RequirePriorityClass(blank) = %v, want ErrNoPriorityClass", err)
		}
		if !strings.Contains(err.Error(), "priority_class") {
			t.Errorf("the field name priority_class left the message: %q", err)
		}
		if inboxbatch.RequirePriorityClass(inboxbatch.ClassHygiene) != nil {
			t.Errorf("a named class must pass")
		}
	})
	t.Run("inboxcurator sentinel", func(t *testing.T) {
		if !strings.Contains(inboxcurator.ErrUnexplainedRoute.Error(), "--reason") {
			t.Errorf("the --reason flag left the message: %q", inboxcurator.ErrUnexplainedRoute)
		}
		if errors.Is(inboxcurator.ErrUnexplainedRoute, inboxcurator.ErrIntake) {
			t.Errorf("ErrUnexplainedRoute must stay a separate sentinel from ErrIntake")
		}
	})
	t.Run("fleet preflight tokens", func(t *testing.T) {
		repo := gittest.Fixture(t)
		policy := filepath.Join(repo.Dir, ".evolve", "policy.json")
		if err := os.MkdirAll(filepath.Dir(policy), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(policy, []byte(`{"tampered":true}`), 0o644); err != nil {
			t.Fatal(err)
		}
		err := fleet.PreflightControlPlane(repo.Dir)
		if err == nil {
			t.Fatalf("a dirty control-plane file must be refused")
		}
		for _, token := range []string{".evolve/policy.json", "evolve ship --class manual", repo.Dir} {
			if !strings.Contains(err.Error(), token) {
				t.Errorf("token %q left the preflight message: %v", token, err)
			}
		}
	})
	t.Run("codequality gofmt error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "absent")
		_, err := codequality.UnformattedGoFiles(missing)
		if err == nil {
			t.Fatalf("an unreadable tree must be an error")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Errorf("the gofmt exit error is no longer wrapped: %v", err)
		}
		if !strings.Contains(err.Error(), "gofmt -l -s "+missing) {
			t.Errorf("the gofmt command and directory left the message: %v", err)
		}
	})
	t.Run("campaign retry error", func(t *testing.T) {
		waves := [][]fleet.CycleSpec{{{Scope: []string{"scope-alpha"}}}}
		failing := func(ctx context.Context, wave []fleet.CycleSpec) []fleet.Result {
			return []fleet.Result{{Index: 0, ExitCode: 1}}
		}
		err := campaign.RunWaves(context.Background(), waves, failing, campaign.RunOptions{
			ProgressPath: filepath.Join(t.TempDir(), "progress.json"),
			PlanSHA:      "plan",
			MaxRetries:   1,
		})
		if err == nil {
			t.Fatalf("a required cycle that fails every try must stop the wave")
		}
		if !strings.Contains(err.Error(), "scope-alpha") || !regexp.MustCompile(`\b2\b`).MatchString(err.Error()) {
			t.Errorf("the failed scope id and the try count left the message: %v", err)
		}
	})
}

func runPackageTests(t *testing.T, pkg string) {
	t.Helper()
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "test", "-count=1", modulePath+pkg)
	if code != 0 {
		t.Errorf("go test %s is red (exit=%d):\n%s", pkg, code, acsassert.OutputTail(out+"\n"+errOut, 30))
	}
}

func TestC2507_006_CampaignPackageTestsPass(t *testing.T) {
	runPackageTests(t, "internal/campaign")
}

func TestC2507_007_CodequalityPackageTestsPass(t *testing.T) {
	runPackageTests(t, "internal/codequality")
}

func TestC2507_008_FleetPackageTestsPass(t *testing.T) {
	runPackageTests(t, "internal/fleet")
}

func TestC2507_009_GcPackageTestsPass(t *testing.T) {
	runPackageTests(t, "internal/gc")
}

func TestC2507_010_InboxbatchPackageTestsPass(t *testing.T) {
	runPackageTests(t, "internal/inboxbatch")
}

func TestC2507_011_InboxcuratorPackageTestsPass(t *testing.T) {
	runPackageTests(t, "internal/inboxcurator")
}
