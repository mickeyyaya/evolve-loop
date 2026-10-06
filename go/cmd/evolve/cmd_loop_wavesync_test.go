package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func gitrun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// syncFixture builds a bare origin plus a runtime clone using real git repos,
// not mocks, because the failure modes under test (non-FF divergence, a
// missing remote, a detached branch) are git's own.
func syncFixture(t *testing.T) (origin, runtime string) {
	t.Helper()
	installCheckRunsGH(t, `{"total_count":1,"check_runs":[{"name":"build+test","status":"completed","conclusion":"success"}]}`, false)
	seed := t.TempDir()
	gitrun(t, seed, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, seed, "add", "f.txt")
	gitrun(t, seed, "commit", "-q", "-m", "c1")
	origin = filepath.Join(t.TempDir(), "origin.git")
	gitrun(t, seed, "clone", "-q", "--bare", ".", origin)
	runtime = filepath.Join(t.TempDir(), "runtime")
	gitrun(t, filepath.Dir(runtime), "clone", "-q", origin, runtime)
	return origin, runtime
}

// originAdvance lands a new commit on origin main via a scratch clone (the
// console-plane landing path).
func originAdvance(t *testing.T, origin string) {
	t.Helper()
	c := filepath.Join(t.TempDir(), "console")
	gitrun(t, filepath.Dir(c), "clone", "-q", origin, c)
	if err := os.WriteFile(filepath.Join(c, "g.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, c, "add", "g.txt")
	gitrun(t, c, "commit", "-q", "-m", "c2")
	gitrun(t, c, "push", "-q", "origin", "main")
}

func installCheckRunsGH(t *testing.T, response string, unavailable bool) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "called")
	body := "#!/bin/sh\nprintf called > \"$GH_CALL_MARKER\"\n"
	if unavailable {
		body += "printf unavailable >&2\nexit 1\n"
	} else {
		body += "printf '%s' \"$GH_CHECK_RUNS_RESPONSE\"\n"
	}
	path := filepath.Join(dir, "gh")
	fakeclitest.Install(t, path, body)
	t.Setenv("GH_CALL_MARKER", marker)
	t.Setenv("GH_CHECK_RUNS_RESPONSE", response)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func TestMainCIAtWaveBoundary_RedCheckRunHalts(t *testing.T) {
	_, runtime := syncFixture(t)
	installCheckRunsGH(t, `{"total_count":1,"check_runs":[{"name":"build+test","status":"completed","conclusion":"failure"}]}`, false)

	var warn bytes.Buffer
	_, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn)
	if halt == nil || !strings.Contains(halt.Error(), "main CI red") {
		t.Fatalf("completed failing origin/main check-run must halt, got halt=%v warn=%q", halt, warn.String())
	}
}

func TestMainCIAtWaveBoundary_GreenProceeds(t *testing.T) {
	_, runtime := syncFixture(t)
	marker := installCheckRunsGH(t, `{"total_count":1,"check_runs":[{"name":"build+test","status":"completed","conclusion":"success"}]}`, false)

	var warn bytes.Buffer
	_, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn)
	if halt != nil {
		t.Fatalf("green origin/main must proceed: %v", halt)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("wave boundary did not query origin/main check-runs: %v", err)
	}
}

func TestMainCIAtWaveBoundary_UnavailableWarnsWithoutFalseRed(t *testing.T) {
	_, runtime := syncFixture(t)
	installCheckRunsGH(t, "", true)

	var warn bytes.Buffer
	_, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn)
	if halt != nil {
		t.Fatalf("unavailable check-run API is not evidence main is red: %v", halt)
	}
	if !strings.Contains(warn.String(), "main CI status unavailable") {
		t.Fatalf("unavailable check-run API must warn distinctly, got %q", warn.String())
	}
}

func TestMainCIAtWaveBoundary_RedHaltsBeforeAnyLaneDispatch(t *testing.T) {
	_, runtime := syncFixture(t)
	evolveDir := filepath.Join(runtime, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	installCheckRunsGH(t, `{"total_count":1,"check_runs":[{"name":"build+test","status":"completed","conclusion":"failure"}]}`, false)

	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:      context.Background(),
		cfg:      loopConfig{ProjectRoot: runtime, EvolveDir: evolveDir},
		cycleEnv: map[string]string{"EVOLVE_CLI_HEALTH": "0"},
		result:   &loopResult{},
		stdout:   &stdout,
		stderr:   &stderr,
	}
	b.deps.Signals = newRootSignalCenter(runtime, evolveDir, &stderr)
	fleetConfig := policy.FleetConfig{Count: 2, Concurrency: 2, Scheduling: "wave"}
	waveBinary := ""
	decision := b.prepareIteration(0, &fleetConfig, &waveBinary, 0)
	b.deps.Signals.Flush()

	if decision.flow != batchReturn || decision.exitCode != 2 {
		t.Fatalf("red main must return before dispatch, got decision=%+v stderr=%q", decision, stderr.String())
	}
	if b.result.StopReason != "main_ci_red_halt" {
		t.Fatalf("stop reason = %q, want main_ci_red_halt", b.result.StopReason)
	}
	if waveBinary != "" {
		t.Fatalf("red-main boundary initialized a lane binary before halt: %q", waveBinary)
	}
}

func TestSyncMainAtWaveBoundary_FastForwards(t *testing.T) {
	origin, runtime := syncFixture(t)
	originAdvance(t, origin)
	var warn bytes.Buffer
	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); !synced {
		t.Fatalf("expected FF sync, got skip: %s", warn.String())
	}
	if _, err := os.Stat(filepath.Join(runtime, "g.txt")); err != nil {
		t.Fatalf("runtime tree not fast-forwarded to origin: %v", err)
	}
}

func TestSyncMainAtWaveBoundary_AlreadyCurrentIsQuietNoop(t *testing.T) {
	_, runtime := syncFixture(t)
	var warn bytes.Buffer
	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatal("no origin movement must report synced=false (nothing to do)")
	}
	if s := warn.String(); strings.Contains(s, "WARN") {
		t.Errorf("up-to-date must not WARN: %s", s)
	}
}

func TestSyncMainAtWaveBoundary_LocalAheadSkipsLoudly(t *testing.T) {
	origin, runtime := syncFixture(t)
	// Local commit not on origin (unpushed dossier shape) + origin also moves.
	if err := os.WriteFile(filepath.Join(runtime, "local.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, runtime, "add", "local.txt")
	gitrun(t, runtime, "commit", "-q", "-m", "local")
	originAdvance(t, origin)
	var warn bytes.Buffer
	synced, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn)
	if synced {
		t.Fatal("diverged history must never merge or rebase — FF-only")
	}
	if halt == nil || !strings.Contains(strings.ToLower(halt.Error()), "diverged") {
		t.Fatalf("a diverged plane must HALT the batch before any lane spends a phase (laneStartRef reads the same relation); got %v", halt)
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Errorf("a diverged sync must be loud: %q", warn.String())
	}
}

func TestSyncMainAtWaveBoundary_NotOnMainSkips(t *testing.T) {
	_, runtime := syncFixture(t)
	gitrun(t, runtime, "checkout", "-q", "-b", "feature")
	var warn bytes.Buffer
	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatal("a non-main checkout must never be synced")
	}
}

func TestSyncMainAtWaveBoundary_NoRemoteSkipsWithoutError(t *testing.T) {
	seed := t.TempDir()
	gitrun(t, seed, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, seed, "add", "f.txt")
	gitrun(t, seed, "commit", "-q", "-m", "c")
	var warn bytes.Buffer
	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), seed, &warn); synced {
		t.Fatal("no remote must be a quiet skip, not a sync")
	}
}

func TestSyncMainAtWaveBoundary_DirtyTrackedFileWarnsBlockedNotDiverged(t *testing.T) {
	origin, runtime := syncFixture(t)
	originAdvance(t, origin)
	// A local file at the path origin's new commit ADDS: git refuses the FF
	// with the would-be-overwritten error — the binary-churn shape.
	if err := os.WriteFile(filepath.Join(runtime, "g.txt"), []byte("local dirt"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatalf("FF over conflicting local changes must not report synced: %s", warn.String())
	}
	if s := warn.String(); !strings.Contains(s, "local tracked changes block") || strings.Contains(s, "diverged") || strings.Contains(s, "inbox file(s)") {
		t.Errorf("blocked-by-dirt must be named as such, never as divergence, and names no inbox file: %q", s)
	}
}

func TestSyncMainAtWaveBoundary_LocalAheadOnlyIsNotReportedAsFastForward(t *testing.T) {
	_, runtime := syncFixture(t)
	if err := os.WriteFile(filepath.Join(runtime, "local.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, runtime, "add", "local.txt")
	gitrun(t, runtime, "commit", "-q", "-m", "dossier: cycle-1616 closeout")
	var warn bytes.Buffer
	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatal("a local-ahead main did not move — it must not report synced=true")
	}
	s := warn.String()
	if strings.Contains(s, "fast-forwarded") {
		t.Errorf("local-ahead must not be reported as a fast-forward: %q", s)
	}
	if !strings.Contains(s, "AHEAD") || !strings.Contains(s, "1 commit") {
		t.Errorf("local-ahead must be named with its count: %q", s)
	}
}

func commitOnOrigin(t *testing.T, origin, rel, body string) {
	t.Helper()
	c := filepath.Join(t.TempDir(), "console")
	gitrun(t, filepath.Dir(c), "clone", "-q", origin, c)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(c, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, c, "add", rel)
	gitrun(t, c, "commit", "-q", "-m", "curate "+rel)
	gitrun(t, c, "push", "-q", "origin", "main")
}

func TestSyncMainAtWaveBoundary_DiscardsInboxStampsOriginSupersededAndFastForwards(t *testing.T) {
	origin, runtime := syncFixture(t)
	item := filepath.Join(".evolve", "inbox", "routed.json")
	retired := filepath.Join(".evolve", "inbox", "retired.json")
	untouched := filepath.Join(".evolve", "inbox", "untouched.json")
	for _, rel := range []string{item, retired, untouched} {
		commitOnOrigin(t, origin, rel, `{"id":"`+filepath.Base(rel)+`"}`)
	}
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	for _, rel := range []string{item, retired, untouched} {
		stamped := `{"id":"` + filepath.Base(rel) + `","route":"console-manual"}`
		if err := os.WriteFile(filepath.Join(runtime, rel), []byte(stamped), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitOnOrigin(t, origin, item, `{"id":"routed","weight":0.5}`)
	retireOnOrigin(t, origin, retired)
	var warn bytes.Buffer

	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); !synced {
		t.Fatalf("stamps origin superseded must not block the fast-forward: %s", warn.String())
	}

	s := warn.String()
	for _, said := range []string{
		"dropped 1 plane-side inbox stamp(s) on items origin retired: .evolve/inbox/retired.json",
		"replayed 1 plane-side inbox stamp(s) onto origin's edits, uncommitted until evolve sync-main: .evolve/inbox/routed.json",
	} {
		if !strings.Contains(s, said) {
			t.Errorf("warn = %q, want it to say %q", s, said)
		}
	}
	for rel, want := range map[string]string{item: `{"id":"routed","route":"console-manual","weight":0.5}`, untouched: `{"id":"untouched.json","route":"console-manual"}`} {
		if got, _ := os.ReadFile(filepath.Join(runtime, rel)); string(got) != want {
			t.Errorf("%s = %s, want %s: origin's edit with the loop's route replayed, and the untouched stamp kept", rel, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(runtime, retired)); !os.IsNotExist(err) {
		t.Errorf("%s still in the root after the sync (%v), want origin's retirement applied", retired, err)
	}
}

func retireOnOrigin(t *testing.T, origin, rel string) {
	t.Helper()
	c := filepath.Join(t.TempDir(), "console")
	gitrun(t, filepath.Dir(c), "clone", "-q", origin, c)
	consumed := filepath.Join(filepath.Dir(rel), "consumed", filepath.Base(rel))
	if err := os.MkdirAll(filepath.Join(c, filepath.Dir(consumed)), 0o755); err != nil {
		t.Fatal(err)
	}
	gitrun(t, c, "mv", rel, consumed)
	gitrun(t, c, "commit", "-q", "-m", "retire "+rel)
	gitrun(t, c, "push", "-q", "origin", "main")
}

func TestSyncMainAtWaveBoundary_NonInboxDirtNamesNoInboxFile(t *testing.T) {
	origin, runtime := syncFixture(t)
	policy := filepath.Join(".evolve", "policy.json")
	commitOnOrigin(t, origin, policy, `{}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	if err := os.WriteFile(filepath.Join(runtime, policy), []byte(`{"local":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	commitOnOrigin(t, origin, policy, `{"fleet":{}}`)
	var warn bytes.Buffer

	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatalf("dirt blocks the fast-forward: %s", warn.String())
	}
	if s := warn.String(); !strings.Contains(s, "local tracked changes block") || strings.Contains(s, "inbox file(s)") {
		t.Errorf("dirt outside the inbox is never named as inbox stamps: %q", s)
	}
}

func TestSyncMainAtWaveBoundary_APlaneCreatedFileIsNamedAPlaneDeletedOneIsNot(t *testing.T) {
	origin, runtime := syncFixture(t)
	moved := filepath.Join(".evolve", "inbox", "moved.json")
	commitOnOrigin(t, origin, moved, `{"id":"moved"}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	if err := os.Remove(filepath.Join(runtime, moved)); err != nil {
		t.Fatal(err)
	}
	filed := filepath.Join(".evolve", "inbox", "filed.json")
	if err := os.WriteFile(filepath.Join(runtime, filed), []byte(`{"id":"filed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	commitOnOrigin(t, origin, filed, `{"id":"filed","weight":0.5}`)
	commitOnOrigin(t, origin, moved, `{"id":"moved","weight":0.5}`)
	var warn bytes.Buffer

	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatalf("the untracked file blocks the fast-forward: %s", warn.String())
	}
	if s := warn.String(); !strings.Contains(s, "1 plane-side inbox file(s) block it: .evolve/inbox/filed.json") {
		t.Errorf("the plane-created file is named, the file the plane's mover moved away is not: %q", s)
	}
}

func TestSyncMainAtWaveBoundary_WarnsAndKeepsTheStampsWhenTheyCannotBePrepared(t *testing.T) {
	origin, runtime := syncFixture(t)
	item := filepath.Join(".evolve", "inbox", "routed.json")
	commitOnOrigin(t, origin, item, `{"id":"routed"}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	stamped := `{"id":"routed","route":"console-manual"}`
	if err := os.WriteFile(filepath.Join(runtime, item), []byte(stamped), 0o644); err != nil {
		t.Fatal(err)
	}
	commitOnOrigin(t, origin, item, `{"id":"routed","weight":0.5}`)
	gitrun(t, runtime, "fetch", "-q", "origin")
	if err := os.WriteFile(filepath.Join(runtime, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatalf("a locked index must not report synced: %s", warn.String())
	}

	if !strings.Contains(warn.String(), "the loop's inbox stamps stay as they are") {
		t.Errorf("warn = %q, want the failed preparation named", warn.String())
	}
	if got, _ := os.ReadFile(filepath.Join(runtime, item)); string(got) != stamped {
		t.Errorf("%s = %s, want the loop's stamp kept after a failed preparation", item, got)
	}
}

func TestSyncMainAtWaveBoundary_ABlockedFastForwardRestoresThePreparedStamps(t *testing.T) {
	origin, runtime := syncFixture(t)
	item := filepath.Join(".evolve", "inbox", "routed.json")
	commitOnOrigin(t, origin, item, `{"id":"routed"}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	stamped := `{"id":"routed","route":"console-manual"}`
	if err := os.WriteFile(filepath.Join(runtime, item), []byte(stamped), 0o644); err != nil {
		t.Fatal(err)
	}
	commitOnOrigin(t, origin, item, `{"id":"routed","weight":0.5}`)
	commitOnOrigin(t, origin, "incoming.txt", "from origin")
	if err := os.WriteFile(filepath.Join(runtime, "incoming.txt"), []byte("a plane-side untracked file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatalf("an untracked file at an incoming path must block the fast-forward: %s", warn.String())
	}

	if got, _ := os.ReadFile(filepath.Join(runtime, item)); string(got) != stamped && !strings.Contains(string(got), "console-manual") {
		t.Errorf("%s = %s, want the prepared stamp restored after the blocked fast-forward", item, got)
	}
	if !strings.Contains(warn.String(), "restored 1 plane-side inbox stamp(s) after the blocked fast-forward: .evolve/inbox/routed.json") {
		t.Errorf("warn = %q, want the restore named", warn.String())
	}
}
