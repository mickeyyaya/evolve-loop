package bridge

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/selfsha"
)

func pathValue(env []string) string {
	value := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			value = v
		}
	}
	return value
}

func firstPathEntry(env []string) string {
	first, _, _ := strings.Cut(pathValue(env), ":")
	return first
}

func pathFixture(t *testing.T, stage string) (Deps, string, string) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "evolve-real")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "cell-bin")
	deps := Deps{EnvStage: stage, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil), ExecutablePath: func() (string, error) { return target, nil }, CellBinRoot: fixedRoot(root)}
	return deps, root, target
}

func assertEntryIsEvolve(t *testing.T, where string, env []string, root, target string) {
	t.Helper()
	entry := firstPathEntry(env)
	if !strings.HasPrefix(entry, root+string(filepath.Separator)) {
		t.Errorf("%s: PATH starts with %q, want a directory under %s (PATH=%q)", where, entry, root, pathValue(env))
		return
	}
	got, err := os.Readlink(filepath.Join(entry, "evolve"))
	if err != nil || got != target {
		t.Errorf("%s: %s/evolve -> %q (err %v), want %q", where, entry, got, err, target)
	}
	if !strings.HasSuffix(pathValue(env), ":/usr/bin:/bin") {
		t.Errorf("%s: the parent PATH entries must follow the cell-bin entry, got %q", where, pathValue(env))
	}
}

func TestCellEnv_PathHasEvolveAtEachStage(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	for _, stage := range []string{"enforce", "shadow", "off"} {
		deps, root, target := pathFixture(t, stage)
		cfg := &Config{CLI: "claude-tmux", Agent: "triage"}
		env, _, err := deps.cellEnv(cfg)
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		assertEntryIsEvolve(t, stage+" cellEnv", env, root, target)
		headless, err := dispatchEnv(deps, &Config{CLI: "codex", Agent: "triage"})
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		assertEntryIsEvolve(t, stage+" dispatchEnv", headless, root, target)
		probe, err := probeEnv(deps, "claude")
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		assertEntryIsEvolve(t, stage+" probeEnv", probe, root, target)
	}
}

func TestPaneEnv_HermeticPathHasEvolveAndSoftStageStaysNil(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	deps, root, target := pathFixture(t, "enforce")
	env, err := deps.paneEnv(&Config{CLI: "claude-tmux", Agent: "triage"})
	if err != nil {
		t.Fatal(err)
	}
	assertEntryIsEvolve(t, "enforce paneEnv", env, root, target)
	deps.EnvStage = "shadow"
	if env, err := deps.paneEnv(&Config{CLI: "claude-tmux", Agent: "triage"}); env != nil || err != nil {
		t.Errorf("shadow paneEnv = %v, %v; want nil, nil", env, err)
	}
}

func TestPrimePaneShell_ExportsTheCellBinPath(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	for _, stage := range []string{"enforce", "shadow", "off"} {
		deps, root, _ := pathFixture(t, stage)
		tm := &fakeTmux{}
		deps.Tmux = tm
		if err := primePaneShell(context.Background(), deps, &Config{CLI: "claude-tmux", Agent: "triage"}, "s", "/wt"); err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		i := slices.IndexFunc(tm.sentKeys, func(k string) bool { return strings.HasPrefix(k, "export PATH=") })
		if i < 0 {
			t.Fatalf("%s: the pane keys have no PATH export: %q", stage, tm.sentKeys)
		}
		line := tm.sentKeys[i]
		if !strings.HasSuffix(line, `:"$PATH"`) || !strings.Contains(line, root) {
			t.Errorf("%s: PATH export %q must put a directory under %s before $PATH", stage, line, root)
		}
	}
}

func TestCellEnv_AResolverFailureStopsEveryCaller(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	boom := errors.New("no executable")
	callers := map[string]func(Deps) error{
		"cellEnv":  func(d Deps) error { _, _, err := d.cellEnv(&Config{CLI: "claude-tmux"}); return err },
		"paneEnv":  func(d Deps) error { _, err := d.paneEnv(&Config{CLI: "claude-tmux"}); return err },
		"dispatch": func(d Deps) error { _, err := dispatchEnv(d, &Config{CLI: "codex"}); return err },
		"probeEnv": func(d Deps) error { _, err := probeEnv(d, "claude"); return err },
		"primePane": func(d Deps) error {
			d.Tmux = &fakeTmux{}
			return primePaneShell(context.Background(), d, &Config{CLI: "claude-tmux"}, "s", "/wt")
		},
		"reusedPane": func(d Deps) error {
			return d.checkReusedSessionEnv(context.Background(), &Config{CLI: "claude-tmux"}, "s")
		},
		"startPane": func(d Deps) error {
			return d.startPaneSession(context.Background(), &Config{CLI: "claude-tmux"}, "s", "/wt")
		},
		"launchLLM": func(d Deps) error {
			_, err := LaunchLLM(context.Background(), d, LLMExec{Purpose: PurposeModelQuery, Binary: "claude"})
			return err
		},
	}
	for name, call := range callers {
		for _, stage := range []string{"enforce", "shadow", "off"} {
			deps, _, _ := pathFixture(t, stage)
			deps.ExecutablePath = func() (string, error) { return "", boom }
			deps.Runner = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
				t.Errorf("%s/%s: the runner started after the resolver failed", name, stage)
				return 0, nil
			}
			if err := call(deps); !errors.Is(err, boom) {
				t.Errorf("%s/%s: error = %v, want it to wrap the resolver error", name, stage, err)
			}
		}
	}
}

func TestCellBinDir_AtomicSingleEntry(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	target := "/opt/evolve/go/bin/evolve"
	var wg sync.WaitGroup
	dirs := make([]string, 16)
	errs := make([]error, 16)
	for i := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dirs[i], errs[i] = ensureCellBin(root, target)
		}()
	}
	wg.Wait()
	for i := range dirs {
		if errs[i] != nil || dirs[i] != dirs[0] {
			t.Fatalf("launch %d: dir %q err %v, want %q and no error", i, dirs[i], errs[i], dirs[0])
		}
	}
	entries, err := os.ReadDir(dirs[0])
	if err != nil || len(entries) != 1 || entries[0].Name() != "evolve" {
		t.Fatalf("directory entries = %v (err %v), want exactly evolve", entries, err)
	}
	if got, _ := os.Readlink(filepath.Join(dirs[0], "evolve")); got != target {
		t.Errorf("evolve -> %q, want %q", got, target)
	}
	other, err := ensureCellBin(root, "/opt/evolve2/evolve")
	if err != nil || other == dirs[0] {
		t.Fatalf("a new target must get a new directory: %q vs %q (err %v)", other, dirs[0], err)
	}
	if got, _ := os.Readlink(filepath.Join(dirs[0], "evolve")); got != target {
		t.Errorf("the old directory changed: evolve -> %q", got)
	}
}

func TestCellBinDir_RebuildsPastLeftoversAndForeignEntries(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	target := "/opt/evolve/go/bin/evolve"
	dir, err := ensureCellBin(root, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "evolve")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/bin/false", filepath.Join(dir, "evolve")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".link-killed-lane")); err != nil {
		t.Fatal(err)
	}
	again, err := ensureCellBin(root, target)
	if err != nil || again != dir {
		t.Fatalf("rebuild gave %q, %v; want %q and no error", again, err, dir)
	}
	if got, _ := os.Readlink(filepath.Join(dir, "evolve")); got != target {
		t.Errorf("evolve -> %q after the rebuild, want %q", got, target)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Errorf("directory entries = %v (err %v), want exactly evolve: a foreign git must not shadow a system tool", entries, err)
	}
	if left, _ := filepath.Glob(filepath.Join(root, ".aside-*")); len(left) != 0 {
		t.Errorf("leftover directories %v, want the swapped-out directory removed", left)
	}
}

func TestCellBinDir_FailsWhenTheLinkOrTheRootCannotBeMade(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	if _, err := ensureCellBin(root, "a\x00b"); err == nil {
		t.Error("a link target the kernel refuses must fail the launch, got nil")
	}
	if left, _ := filepath.Glob(filepath.Join(root, ".stage-*")); len(left) != 0 {
		t.Errorf("staged directories %v, want a failed link to clean up its stage", left)
	}
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureCellBin(filepath.Join(file, "cell-bin"), "/opt/a/evolve"); err == nil {
		t.Error("an unusable root must fail the launch, got nil")
	}
}

func TestCellBinDir_ManyConcurrentLaunchesOnAFreshRootAllSucceed(t *testing.T) {
	t.Parallel()
	for round := range 150 {
		root := filepath.Join(t.TempDir(), "cell-bin")
		var wg sync.WaitGroup
		errs := make([]error, 12)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = ensureCellBin(root, "/opt/x/evolve")
			}()
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d launch %d failed: %v, want every concurrent launch to succeed", round, i, err)
			}
		}
	}
}

func TestCellBinDir_ManyConcurrentLaunchesRepairAPlantedDirectoryAllSucceed(t *testing.T) {
	t.Parallel()
	target := "/opt/x/evolve"
	for round := range 150 {
		root := filepath.Join(t.TempDir(), "cell-bin")
		dir, err := ensureCellBin(root, target)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		errs := make([]error, 12)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = ensureCellBin(root, target)
			}()
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d launch %d failed: %v, want every concurrent repair to succeed", round, i, err)
			}
		}
		if !cellBinExact(dir, target) {
			t.Fatalf("round %d: directory is not exactly the evolve link after the repair", round)
		}
		if left, _ := filepath.Glob(filepath.Join(root, ".*")); len(left) != 0 {
			t.Fatalf("round %d: leftover stage or aside directories %v", round, left)
		}
	}
}

func TestCellBinDir_APlantedFileBesideTheLinkIsRebuiltToExactlyTheLink(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	target := "/opt/x/evolve"
	dir, err := ensureCellBin(root, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if cellBinExact(dir, target) {
		t.Fatal("a directory holding the link plus a planted file must not count as exact")
	}
	if _, err := ensureCellBin(root, target); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "evolve" {
		t.Errorf("entries = %v (err %v), want exactly the evolve link", entries, err)
	}
}

func TestCellBinDir_TheRootAndTheDirectoryAreOwnerOnly(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	dir, err := ensureCellBin(root, "/opt/x/evolve")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, dir} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %v (err %v), want 0700", path, info.Mode().Perm(), err)
		}
	}
}

func TestInstallCellBin_AnExactDirectoryIsKeptAndAMissingStageFailsTheSwap(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	target := "/opt/a/evolve"
	dir, err := ensureCellBin(root, target)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := stageCellBin(root, target)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := installCellBin(staged, dir, target); err != nil || !cellBinExact(dir, target) {
		t.Errorf("install over an exact directory = %v, exact %v; want nil and the directory kept", err, cellBinExact(dir, target))
	}
	if _, err := os.Stat(staged); err == nil {
		t.Error("the stage of the losing launch must be removed")
	}
	if after, err := os.Stat(dir); err != nil || !os.SameFile(before, after) {
		t.Errorf("an exact directory must be kept in place, not swapped out (err %v)", err)
	}
	if err := installCellBin(filepath.Join(root, ".stage-missing"), filepath.Join(root, "other"), target); err == nil {
		t.Error("a swap with no stage must fail, got nil")
	}
}

func TestPrependPath_Cases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"parent entries follow in order", []string{"A=1", "PATH=/x:/y"}, "/cb:/x:/y"},
		{"unset parent gets the fallback", []string{"A=1"}, "/cb:/usr/bin:/bin"},
		{"empty parent gets the fallback", []string{"PATH="}, "/cb:/usr/bin:/bin"},
		{"the last duplicate wins and one entry stays", []string{"PATH=/first", "A=1", "PATH=/last"}, "/cb:/last"},
	}
	for _, tc := range cases {
		got := prependPath(tc.env, "/cb")
		count := 0
		for _, kv := range got {
			if strings.HasPrefix(kv, "PATH=") {
				count++
			}
		}
		if pathValue(got) != tc.want || count != 1 {
			t.Errorf("%s: PATH=%q with %d entries, want %q with 1", tc.name, pathValue(got), count, tc.want)
		}
		if !slices.Contains(got, "A=1") && slices.Contains(tc.env, "A=1") {
			t.Errorf("%s: another variable was lost: %v", tc.name, got)
		}
	}
}

func TestPrependPath_DoesNotMutateItsInput(t *testing.T) {
	t.Parallel()
	in := []string{"PATH=/x", "A=1"}
	prependPath(in, "/cb")
	if !slices.Equal(in, []string{"PATH=/x", "A=1"}) {
		t.Errorf("input mutated: %v", in)
	}
}

func TestCellBinDir_ARootResolverFailureOrAnUnusableRootStopsTheLaunch(t *testing.T) {
	t.Parallel()
	boom := errors.New("no home")
	deps := Deps{ExecutablePath: func() (string, error) { return "/opt/evolve", nil }, CellBinRoot: func() (string, error) { return "", boom }}
	if dir, err := deps.cellBinDir(&Config{}); !errors.Is(err, boom) || dir != "" {
		t.Errorf("a root resolver failure gave %q, %v; want no directory and the wrapped error", dir, err)
	}
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	deps.CellBinRoot = fixedRoot(filepath.Join(file, "cell-bin"))
	cfg := &Config{}
	if dir, err := deps.cellBinDir(cfg); err == nil || dir != "" || cfg.cellBin != "" {
		t.Errorf("an unusable root gave %q, %v and cached %q; want an error and no cache", dir, err, cfg.cellBin)
	}
}

func TestCellBinDir_TheDirectoryIsCachedOnTheConfigAndUsesTheRealExecutableByDefault(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "cell-bin")
	deps := Deps{CellBinRoot: fixedRoot(root)}
	cfg := &Config{}
	dir, err := deps.cellBinDir(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want, err := selfsha.Path()
	if got, _ := os.Readlink(filepath.Join(dir, "evolve")); err != nil || got != want {
		t.Fatalf("evolve -> %q, want the running executable %q (%v)", got, want, err)
	}
	deps.CellBinRoot = func() (string, error) { return "", errors.New("must not resolve again") }
	if again, err := deps.cellBinDir(cfg); err != nil || again != dir {
		t.Errorf("the cached directory = %q, %v; want %q", again, err, dir)
	}
}

func TestCellBinDir_AnUnwritableRootFailsTheLink(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a root user can write to a read-only directory")
	}
	root := filepath.Join(t.TempDir(), "cell-bin")
	dir, err := ensureCellBin(root, "/opt/a/evolve")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "evolve")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	if _, err := ensureCellBin(root, "/opt/a/evolve"); err == nil {
		t.Error("a root that refuses a new link must fail the launch, got nil")
	}
}

func TestCellBinRootOf_IsUnderTheHomeAndFailsLoudly(t *testing.T) {
	t.Parallel()
	got, err := cellBinRootOf(func() (string, error) { return "/home/u", nil })
	if err != nil || got != "/home/u/.evolve/cell-bin" {
		t.Errorf("root = %q, %v; want /home/u/.evolve/cell-bin", got, err)
	}
	boom := errors.New("no passwd")
	if got, err := cellBinRootOf(func() (string, error) { return "", boom }); !errors.Is(err, boom) || got != "" {
		t.Errorf("a home failure gave %q, %v; want no root and the error", got, err)
	}
}

func TestRunDispatch_AnEnvFailureNeverStartsTheRunnerAndARunnerFailureIsAMissingBinary(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	deps, root, target := pathFixture(t, "shadow")
	var gotEnv []string
	deps.Runner = func(_ context.Context, _, _ string, _ []string, env []string, _ io.Reader, _, _ io.Writer) (int, error) {
		gotEnv = env
		return 7, nil
	}
	rc, err := deps.runDispatch(context.Background(), &Config{CLI: "codex"}, dispatchRun{name: "codex", stdout: io.Discard, stderr: io.Discard, extraEnv: []string{"EXTRA=1"}})
	if rc != 7 || err != nil || !slices.Contains(gotEnv, "EXTRA=1") {
		t.Fatalf("rc %d err %v env has EXTRA %v; want 7, nil, true", rc, err, slices.Contains(gotEnv, "EXTRA=1"))
	}
	assertEntryIsEvolve(t, "runDispatch", gotEnv, root, target)
	boom := errors.New("spawn failed")
	deps.Runner = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, boom
	}
	if rc, err := deps.runDispatch(context.Background(), &Config{CLI: "codex"}, dispatchRun{name: "codex", stdout: io.Discard, stderr: io.Discard}); rc != ExitMissingBinary || !errors.Is(err, boom) {
		t.Errorf("a runner failure gave rc %d, %v; want ExitMissingBinary and the error", rc, err)
	}
	deps.ExecutablePath = func() (string, error) { return "", boom }
	if rc, err := deps.runDispatch(context.Background(), &Config{CLI: "codex"}, dispatchRun{name: "codex", stdout: io.Discard, stderr: io.Discard}); rc != ExitBadFlags || !errors.Is(err, boom) {
		t.Errorf("an env failure gave rc %d, %v; want ExitBadFlags and the error", rc, err)
	}
}
