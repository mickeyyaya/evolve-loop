package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func seedAuthAndQuotaBenches(t *testing.T, root, family string) *clihealth.Store {
	t.Helper()
	now := time.Now()
	store := clihealth.NewStore(root, nil)
	for _, e := range []clihealth.Entry{
		{Family: family, Reason: clihealth.CredentialPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
		{Family: family, Model: "quota-model", Reason: clihealth.UsageProbePattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
	} {
		if err := store.Bench(e); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func authBenchState(store *clihealth.Store, family string) (auth, quota bool) {
	benches, _ := store.Load()
	_, auth = benches[family]
	_, quota = benches[family+"\x00quota-model"]
	return auth, quota
}

func TestLiveSmokeTest_ASuccessfulProbeClearsTheAuthBenchOfItsFamily(t *testing.T) {
	t.Parallel()
	ws, root := t.TempDir(), t.TempDir()
	store := seedAuthAndQuotaBenches(t, root, "claude")
	signals, got := recordingSignals()
	deps := liveSmokeDeps(&liveArtifactTmux{
		FakeTmuxController: &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}},
		artifact:           filepath.Join(ws, LiveSmokeArtifact),
	})
	deps.Signals = signals
	stderr := new(strings.Builder)
	deps.Stderr = stderr

	if rc := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws, ProjectRoot: root}, deps).RC; rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}

	auth, quota := authBenchState(store, "claude")
	if auth || !quota {
		t.Fatalf("auth bench kept=%v quota bench kept=%v; want the auth bench cleared and the quota bench kept", auth, quota)
	}
	cleared := eventsWithCode(*got, clihealth.CodeAuthBenchCleared)
	if len(cleared) != 1 || cleared[0].Fields["family"] != "claude" || cleared[0].Fields["source"] != string(clihealth.SourceSmoke) {
		t.Fatalf("events=%+v; want one auth-bench-cleared signal for claude from the smoke probe", *got)
	}
	if !strings.Contains(stderr.String(), "the smoke of claude succeeded: the auth_recheck bench claude is cleared") {
		t.Fatalf("stderr=%q; want the clear on the log", stderr.String())
	}
}

func TestLiveSmokeTest_AWalledProbeKeepsTheAuthBench(t *testing.T) {
	t.Parallel()
	ws, root := t.TempDir(), t.TempDir()
	store := seedAuthAndQuotaBenches(t, root, "codex")
	wall := "■ You've hit your usage limit. Upgrade to Pro or try again at 6:11 AM."
	deps := liveSmokeDeps(&FakeTmuxController{CaptureFrames: []string{"›", "thinking", wall, wall, wall, wall, wall, wall}})

	if rc := LiveSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: ws, ProjectRoot: root}, deps).RC; rc == ExitOK {
		t.Fatal("the walled probe exited OK")
	}

	if auth, quota := authBenchState(store, "codex"); !auth || !quota {
		t.Fatalf("auth bench kept=%v quota bench kept=%v; a failed probe keeps every bench", auth, quota)
	}
}

func TestBootSmokeTest_ABootOnlySuccessKeepsTheAuthBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := seedAuthAndQuotaBenches(t, root, "claude")
	deps, _ := bootSmokeDeps(&fakeTmux{paneSeq: []string{"❯"}})

	if rc, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: t.TempDir(), ProjectRoot: root}, deps); rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}

	if auth, quota := authBenchState(store, "claude"); !auth || !quota {
		t.Fatalf("auth bench kept=%v quota bench kept=%v; a REPL that boots answers no prompt and proves no login", auth, quota)
	}
}

func TestClearAuthBench_AWriteFailureIsAWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedAuthAndQuotaBenches(t, root, "claude")
	stderr := new(strings.Builder)
	breakStoreDir(t, root)

	clearAuthBench(root, "claude-tmux", clihealth.SourceSmoke, Deps{Stderr: stderr})

	if !strings.Contains(stderr.String(), "WARN") || !strings.Contains(stderr.String(), "claude") {
		t.Fatalf("stderr=%q; want a WARN that names the family", stderr.String())
	}
}

func TestClearAuthBench_TheLogNamesTheKeyOfAScopedBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Now()
	store := clihealth.NewStore(root, nil)
	if err := store.Bench(clihealth.Entry{Family: "claude", Scope: "opus", Reason: clihealth.CredentialPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	stderr := new(strings.Builder)

	clearAuthBench(root, "claude-tmux", clihealth.SourceSmoke, Deps{Stderr: stderr})

	if !strings.Contains(stderr.String(), "the auth_recheck bench claude@opus is cleared") {
		t.Fatalf("stderr=%q; want the key of the scoped bench", stderr.String())
	}
}

func TestClearAuthBench_WithNoLogTheClearStillRuns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := seedAuthAndQuotaBenches(t, root, "claude")

	clearAuthBench(root, "claude-tmux", clihealth.SourceSmoke, Deps{})

	if auth, quota := authBenchState(store, "claude"); auth || !quota {
		t.Fatalf("auth bench kept=%v quota bench kept=%v", auth, quota)
	}
}

func TestClearAuthBench_NoProjectRootNeverClearsTheStoreOfTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	store := seedAuthAndQuotaBenches(t, dir, "claude")

	clearAuthBench("", "claude-tmux", clihealth.SourceSmoke, Deps{})

	if auth, _ := authBenchState(store, "claude"); !auth {
		t.Fatal("a success with no project root cleared the bench of the working directory")
	}
}

func TestEngineLaunch_ASuccessfulLaunchClearsTheAuthBenchAndAFailedOneKeepsIt(t *testing.T) {
	root := t.TempDir()
	store := seedAuthAndQuotaBenches(t, root, launchOutcomeFixtureCLI)
	deps, got, _ := recordingDeps(t)

	if _, _, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitBadFlags, "empty"), core.BridgeRequest{ProjectRoot: root}); err == nil {
		t.Fatal("exit 10 is an error")
	}
	if auth, _ := authBenchState(store, launchOutcomeFixtureCLI); !auth {
		t.Fatal("a failed launch cleared the auth bench")
	}

	if _, _, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitOK, ""), core.BridgeRequest{ProjectRoot: root}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	auth, quota := authBenchState(store, launchOutcomeFixtureCLI)
	if auth || !quota {
		t.Fatalf("auth bench kept=%v quota bench kept=%v; want the auth bench cleared and the quota bench kept", auth, quota)
	}
	cleared := eventsWithCode(*got, clihealth.CodeAuthBenchCleared)
	if len(cleared) != 1 || cleared[0].Fields["source"] != string(clihealth.SourceLaunch) || cleared[0].Fields["family"] != launchOutcomeFixtureCLI {
		t.Fatalf("events=%+v; want one auth-bench-cleared signal from the launch", cleared)
	}
}

func breakStoreDir(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}
