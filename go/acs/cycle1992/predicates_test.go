//go:build acs

package cycle1992

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const ghScript = `#!/bin/sh
for a in "$@"; do printf '%s\n' "$a" >> "$FAKE_GH_LOG"; done
printf -- '--\n' >> "$FAKE_GH_LOG"
case "$1 $2" in
"pr list") printf '%s\n' "[]" ;;
"pr create") printf '%s\n' "https://github.com/acme/widgets/pull/4217" ;;
*) echo "fake gh: unexpected call: $*" >&2; exit 99 ;;
esac
`

type shipRun struct {
	rc     int
	stdout string
	stderr string
	ghLog  string
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goDir(t)
	cmd.WaitDelay = 10 * time.Second
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("RED: go build ./cmd/evolve: %v\n%s", err, out)
	}
	return bin
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("RED: evolve did not run: %v", err)
	}
	return exit.ExitCode()
}

func runShipPrAt(t *testing.T, branch string, shipArgs ...string) shipRun {
	t.Helper()
	bin := buildEvolve(t)
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "chore: seed")
	if branch != "main" {
		repo.Git("checkout", "-b", branch)
	}

	ghDir := t.TempDir()
	ghLog := filepath.Join(ghDir, "gh-args.log")
	if err := os.WriteFile(filepath.Join(ghDir, "gh"), []byte(ghScript), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := append([]string{"ship"}, shipArgs...)
	args = append(args, "--project-root", repo.Dir, "feat: add the widget")
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = repo.Dir
	cmd.WaitDelay = 10 * time.Second
	cmd.Env = append(os.Environ(), "PATH="+ghDir+string(os.PathListSeparator)+os.Getenv("PATH"), "FAKE_GH_LOG="+ghLog)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()

	logged, _ := os.ReadFile(ghLog)
	return shipRun{
		rc:     exitCode(t, err),
		stdout: stdout.String(),
		stderr: stderr.String(),
		ghLog:  string(logged),
	}
}

func runNamedCmdTests(t *testing.T, names ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", "./cmd/evolve")
	cmd.Dir = goDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("RED: go test ./cmd/evolve for %v failed: %v\n%s", names, err, out)
		return
	}
	for _, name := range names {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass\n%s", name, out)
		}
	}
}

func TestC1992_001_ShipPrCreatesThePullRequestAndPrintsItsNumber(t *testing.T) {
	runNamedCmdTests(t, "TestRunShipCmd_Pr_CreatesThePullRequestAndPrintsItsNumber", "TestRunShipCmd_Pr_BodyFileReachesGhAndRequiresPr")
}

func TestC1992_002_ABranchWithAnOpenPullRequestReportsItInsteadOfCreatingASecond(t *testing.T) {
	runNamedCmdTests(t, "TestRunShipCmd_Pr_ReportsAnOpenPullRequestInsteadOfCreatingASecond")
}

func TestC1992_003_ShipPrOnMainExits10AndTouchesNothing(t *testing.T) {
	run := runShipPrAt(t, "main", "--class", "manual", "--pr")

	if run.rc != 10 {
		t.Errorf("RED: evolve ship --class manual --pr on main exited %d, want 10\nstderr: %s", run.rc, run.stderr)
	}
	if run.ghLog != "" {
		t.Errorf("RED: gh was called for --pr on main: %q", run.ghLog)
	}
	if strings.Contains(run.stdout, "4217") {
		t.Errorf("RED: stdout reports a PR for main: %q", run.stdout)
	}
}

func TestC1992_004_OnlyTheManualClassMayAskForAPullRequest(t *testing.T) {
	run := runShipPrAt(t, "feature/widget", "--class", "cycle", "--pr")

	if run.rc != 10 {
		t.Errorf("RED: evolve ship --class cycle --pr exited %d, want 10\nstderr: %s", run.rc, run.stderr)
	}
	if run.ghLog != "" {
		t.Errorf("RED: gh was called for a non-manual --pr: %q", run.ghLog)
	}
}

func TestC1992_005_AGhFailureExits2AndARefusedShipOpensNoPullRequest(t *testing.T) {
	runNamedCmdTests(t,
		"TestRunShipCmd_Pr_AGhFailureExits2AfterShipKeptTheCommitPushed",
		"TestRunShipCmd_Pr_ARefusedShipOpensNoPullRequest",
		"TestRunShipCmd_Pr_ADryRunOpensNoPullRequest",
		"TestRunShipCmd_Pr_OnlyTheManualClassMayOpenAPullRequest",
		"TestRunShipCmd_Pr_OnMainExits10BeforeShipOrGhRun",
	)
}
