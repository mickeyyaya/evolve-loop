//go:build acs

package cycle1798

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		root := acsassert.RepoRoot(t)
		dir, err := os.MkdirTemp("", "evolve-acs-1798-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/evolve")
		cmd.Dir = filepath.Join(root, "go")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildFailure{out: string(out), err: err}
		}
	})
	if buildErr != nil {
		t.Fatalf("evolve binary build failed: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct {
	out string
	err error
}

func (b *buildFailure) Error() string { return b.err.Error() + ": " + b.out }

type runResult struct {
	stdout, stderr string
	code           int
}

func runEvolve(t *testing.T, env []string, cwd string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), env...)
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run evolve %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return runResult{so.String(), se.String(), code}
}

func fakeTmuxEnv(t *testing.T) (env []string, callLog string) {
	t.Helper()
	bin := t.TempDir()
	callLog = filepath.Join(t.TempDir(), "tmux-calls.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + callLog + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}, callLog
}

func tmuxCalls(t *testing.T, callLog string) string {
	t.Helper()
	b, err := os.ReadFile(callLog)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func TestC1798_001_BareGCRefusesBeforeAnyReaperRuns(t *testing.T) {
	env, callLog := fakeTmuxEnv(t)
	res := runEvolve(t, env, t.TempDir(), "gc")
	if res.code != 1 {
		t.Errorf("bare gc exit=%d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "mutating run refused") {
		t.Errorf("bare gc stderr lacks refusal: %q", res.stderr)
	}
	if calls := tmuxCalls(t, callLog); calls != "" {
		t.Errorf("bare gc reached tmux before refusing: %q", calls)
	}
	if strings.Contains(res.stdout, "orphan session") || strings.Contains(res.stdout, "socket") {
		t.Errorf("bare gc reported reaper output before refusing: %q", res.stdout)
	}
}

func TestC1798_002_GCHelpDoesNotPromiseACwdDefault(t *testing.T) {
	res := runEvolve(t, nil, t.TempDir(), "gc", "-h")
	help := res.stdout + res.stderr
	if !strings.Contains(help, "-project-root") {
		t.Fatalf("gc help lacks -project-root: %q", help)
	}
	if strings.Contains(help, "default = current directory") {
		t.Errorf("gc help still promises a cwd default for -project-root: %q", help)
	}
	if !strings.Contains(strings.ToLower(help), "refused") {
		t.Errorf("gc help does not say a non-dry run without -project-root is refused: %q", help)
	}
}

func TestC1798_003_GCWithExplicitRootStillReachesTheReapers(t *testing.T) {
	env, callLog := fakeTmuxEnv(t)
	runEvolve(t, env, t.TempDir(), "gc", "--dry-run", "--project-root", t.TempDir())
	if tmuxCalls(t, callLog) == "" {
		t.Errorf("gc --dry-run with an explicit root never reached tmux; validation-first must not skip the reapers")
	}
}

type fixtureEntry struct {
	Cycle          int    `json:"cycle"`
	Classification string `json:"classification"`
	Summary        string `json:"summary"`
	RecordedAt     string `json:"recordedAt"`
	ExpiresAt      string `json:"expiresAt"`
}

func writeState(t *testing.T, entries []fixtureEntry) (root, statePath string) {
	t.Helper()
	root = t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(evolveDir, "state.json")
	raw, err := json.Marshal(map[string]any{"failedApproaches": entries, "lastCycleNumber": 9})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, statePath
}

func mixedEntries() []fixtureEntry {
	return []fixtureEntry{
		{1, "infrastructure-systemic", "tmux wedged", "2026-10-01T00:00:00Z", "2099-01-01T00:00:00Z"},
		{2, "code-build-fail", "build red", "2026-10-02T00:00:00Z", "2099-01-01T00:00:00Z"},
		{3, "ship-gate-config", "gate cfg", "2026-10-03T00:00:00Z", "2099-01-01T00:00:00Z"},
		{4, "code-audit-fail", "stale audit", "2020-01-01T00:00:00Z", "2020-02-01T00:00:00Z"},
	}
}

func approachClasses(t *testing.T, statePath string) []string {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		FailedApproaches []fixtureEntry `json:"failedApproaches"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range st.FailedApproaches {
		out = append(out, e.Classification)
	}
	return out
}

func TestC1798_004_FailuresListJSONEmitsEveryEntryAndLeavesStateUntouched(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	before, _ := os.ReadFile(statePath)
	res := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", root, "--json")
	if res.code != 0 {
		t.Fatalf("failures list exit=%d stderr=%q", res.code, res.stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("list --json is not a JSON array: %v: %q", err, res.stdout)
	}
	if len(got) != 4 {
		t.Fatalf("list --json returned %d entries, want 4", len(got))
	}
	for _, key := range []string{"recordedAt", "classification", "summary"} {
		if _, ok := got[0][key]; !ok {
			t.Errorf("entry lacks key %q: %v", key, got[0])
		}
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Errorf("failures list mutated state.json")
	}
}

func TestC1798_005_FailuresListAbsentStateIsAnEmptyArray(t *testing.T) {
	res := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", t.TempDir(), "--json")
	if res.code != 0 {
		t.Fatalf("exit=%d stderr=%q", res.code, res.stderr)
	}
	if strings.TrimSpace(res.stdout) != "[]" {
		t.Errorf("absent state.json must list as [], got %q", res.stdout)
	}
}

func TestC1798_006_FailuresListClassFilterAndUnknownClass(t *testing.T) {
	root, _ := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", root, "--json", "--class", "code-build-fail")
	var got []map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil || len(got) != 1 {
		t.Errorf("--class code-build-fail: err=%v entries=%d stdout=%q", err, len(got), res.stdout)
	}
	bad := runEvolve(t, nil, t.TempDir(), "failures", "list", "--project-root", root, "--class", "no-such-class")
	if bad.code != 10 || !strings.Contains(bad.stderr, "unknown class") {
		t.Errorf("unknown class: exit=%d stderr=%q, want 10 + 'unknown class'", bad.code, bad.stderr)
	}
}

func TestC1798_007_FailuresUsageErrorsExitTen(t *testing.T) {
	for _, args := range [][]string{{"failures"}, {"failures", "frobnicate"}} {
		res := runEvolve(t, nil, t.TempDir(), args...)
		if res.code != 10 || !strings.Contains(res.stderr, "evolve failures: usage:") {
			t.Errorf("%v: exit=%d stderr=%q, want 10 + usage", args, res.code, res.stderr)
		}
	}
}

func TestC1798_008_FailuresMutatingVerbsRefuseWithoutProjectRoot(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	before, _ := os.ReadFile(statePath)
	for _, sub := range []string{"reset", "prune"} {
		res := runEvolve(t, nil, root, "failures", sub)
		if res.code != 1 || !strings.Contains(res.stderr, "mutating run refused") {
			t.Errorf("%s without --project-root: exit=%d stderr=%q", sub, res.code, res.stderr)
		}
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Errorf("a refused run mutated state.json")
	}
}

func TestC1798_009_FailuresResetDropsOnlyInfrastructureClassesAndIsIdempotent(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root)
	if res.code != 0 {
		t.Fatalf("reset exit=%d stderr=%q", res.code, res.stderr)
	}
	got := strings.Join(approachClasses(t, statePath), ",")
	if got != "code-build-fail,code-audit-fail" {
		t.Errorf("after reset classes=%q, want code-build-fail,code-audit-fail", got)
	}
	again := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root)
	if again.code != 0 || strings.Join(approachClasses(t, statePath), ",") != got {
		t.Errorf("second reset not idempotent: exit=%d", again.code)
	}
}

func TestC1798_010_FailuresResetFingerprintIsAcknowledged(t *testing.T) {
	root, _ := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "reset", "--project-root", root, "--fingerprint", "fp-acs-1798")
	if res.code != 0 {
		t.Fatalf("exit=%d stderr=%q", res.code, res.stderr)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "resolved-fingerprints.json"))
	if err != nil || !strings.Contains(string(raw), "fp-acs-1798") {
		t.Errorf("fingerprint not acked in resolved-fingerprints.json: err=%v body=%q", err, raw)
	}
}

func TestC1798_011_FailuresPruneRemovesOnlyExpiredEntries(t *testing.T) {
	root, statePath := writeState(t, mixedEntries())
	res := runEvolve(t, nil, t.TempDir(), "failures", "prune", "--project-root", root)
	if res.code != 0 {
		t.Fatalf("prune exit=%d stderr=%q", res.code, res.stderr)
	}
	got := strings.Join(approachClasses(t, statePath), ",")
	if got != "infrastructure-systemic,code-build-fail,ship-gate-config" {
		t.Errorf("after prune classes=%q, want only the expired code-audit-fail removed", got)
	}
}
