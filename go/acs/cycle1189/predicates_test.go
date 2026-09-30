//go:build acs

package cycle1189

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const reconcileInstruction = "evolve sync-main"

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=acs", "GIT_AUTHOR_EMAIL=acs@example.com",
		"GIT_COMMITTER_NAME=acs", "GIT_COMMITTER_EMAIL=acs@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func commitPush(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	git(t, dir, "add", file)
	git(t, dir, "commit", "-m", "acs fixture "+file)
	git(t, dir, "push", "origin", "main")
}

func baseFixture(t *testing.T, behind bool) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatalf("mkdir origin: %v", err)
	}
	git(t, root, "init", "--bare", "--initial-branch=main", origin)

	work := filepath.Join(root, "work")
	git(t, root, "clone", origin, work)
	commitPush(t, work, "seed.txt", "seed\n")

	if behind {
		other := filepath.Join(root, "other")
		git(t, root, "clone", origin, other)
		commitPush(t, other, "ahead.txt", "ahead\n")
	}
	return work
}

func greenOptions(t *testing.T, projectRoot string) looppreflight.Options {
	t.Helper()
	return looppreflight.Options{
		ProjectRoot:   projectRoot,
		EvolveDir:     t.TempDir(),
		Stderr:        &bytes.Buffer{},
		SkipBoot:      true,
		SpinePhases:   []string{"build", "scout"},
		FactoryKnown:  func(string) bool { return true },
		ContractKnown: func(string) bool { return true },
		ProfileLister: func() ([]string, error) { return []string{"builder"}, nil },
		ProfileGetter: func(name string) (profiles.Profile, error) {
			return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
		},
		DriverKnown: func(string) bool { return true },
		ProbeCLI: func(bin string) (doctor.Result, error) {
			return doctor.Result{Tool: bin, Found: true, Path: "/usr/bin/" + bin, Method: "path"}, nil
		},
		HostProbe: func() preflight.Profile {
			return preflight.Profile{Sandbox: preflight.Sandbox{ExpectedToWork: true, SandboxExecAvailable: true}}
		},
		DirWritable:          func(string) bool { return true },
		DiskFreeBytes:        func(string) (uint64, error) { return 50 << 30, nil },
		OrphanKill:           func(context.Context, string) error { return nil },
		SelfUpdateEvidence:   func(string) (bool, string, error) { return false, "", nil },
		PinnedLister:         func() ([]string, error) { return nil, nil },
		VersionInventory:     func() map[string]string { return map[string]string{} },
		PhaseRoutingWarnings: func() []string { return nil },
	}
}

func haltText(r looppreflight.Result) string {
	var b strings.Builder
	for _, c := range r.Checks {
		if c.Level == looppreflight.LevelHalt {
			b.WriteString(c.Name + ": " + c.Message + "\n" + c.Detail + "\n")
		}
	}
	return b.String()
}

func TestC1189_001_BootHaltsWhenBaseDivergedFromOrigin(t *testing.T) {
	work := baseFixture(t, true)

	r, err := looppreflight.Run(greenOptions(t, work))
	if err != nil {
		t.Fatalf("Run against a behind-origin repo returned a harness error: %v", err)
	}
	if !r.Halted() {
		t.Fatalf("local main is BEHIND origin/main but preflight did not HALT (overall=%s); lanes would be based on a stale base (cycle-969 GIT_PUSH_REJECTED)", r.OverallLevel)
	}
	if txt := haltText(r); !strings.Contains(txt, reconcileInstruction) {
		t.Errorf("HALT does not name the reconcile instruction %q — the operator gets a stop with no next step.\nhalt text:\n%s", reconcileInstruction, txt)
	}
}

func TestC1189_002_BootDoesNotHaltWhenBaseInSync(t *testing.T) {
	work := baseFixture(t, false)

	r, err := looppreflight.Run(greenOptions(t, work))
	if err != nil {
		t.Fatalf("Run against an in-sync repo returned a harness error: %v", err)
	}
	if r.Halted() {
		t.Fatalf("local main is IN SYNC with origin/main but preflight HALTED — false positive would bench every healthy boot.\nhalt text:\n%s", haltText(r))
	}
}

const followTestName = "TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines"

const bridgeWatchTestPath = "go/cmd/evolve/cmd_bridge_watch_test.go"

const followRepeatCount = 25

func TestC1189_003_BridgeWatchFollowTestIsStableUnderRepeat(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	if _, err := os.Stat(goDir); err != nil {
		t.Fatalf("go module dir not found under %s: %v", root, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-race",
		"-count="+strconv.Itoa(followRepeatCount),
		"-timeout=7m",
		"-run", "^"+followTestName+"$",
		"./cmd/evolve/")
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("`go test -race -count=%d -run %s ./cmd/evolve/` failed (%v) — the follow wait is still time-window bound:\n%s",
			followRepeatCount, followTestName, err, out)
	}
}

func goFuncBody(t *testing.T, path, name string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(raw)
	idx := strings.Index(src, "func "+name+"(")
	if idx < 0 {
		t.Fatalf("func %s not found in %s", name, path)
	}
	open := strings.Index(src[idx:], "{")
	if open < 0 {
		t.Fatalf("func %s in %s has no body", name, path)
	}
	start := idx + open
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : i+1]
			}
		}
	}
	t.Fatalf("unbalanced braces walking func %s in %s", name, path)
	return ""
}

var secondsDeadlineRe = regexp.MustCompile(`context\.WithTimeout\([^,]+,\s*(\d+)\s*\*\s*time\.Second\s*\)`)

func TestC1189_004_BridgeWatchFollowWaitIsEventDriven(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), bridgeWatchTestPath)
	body := goFuncBody(t, path, followTestName)

	m := secondsDeadlineRe.FindStringSubmatch(body)
	if m == nil {
		t.Errorf("%s has no `context.WithTimeout(..., N*time.Second)` deadline — the sub-second window is the flake (acceptance: deadline >= 10s)", followTestName)
	} else if secs, _ := strconv.Atoi(m[1]); secs < 10 {
		t.Errorf("%s deadline is %ds; acceptance requires >= 10s", followTestName, secs)
	}

	if strings.Contains(body, "time.Sleep(") && !strings.Contains(body, "for ") {
		t.Errorf("%s still calls time.Sleep outside any loop — a bare fixed sleep is exactly the macOS flake; wait on an event (channel/poll loop) instead", followTestName)
	}
}

const sealKindPrefix = "reset-seal-"

func newFixtureLedger(t *testing.T) (*ledger.FileLedger, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	return ledger.New(dir), dir
}

func appendN(t *testing.T, l *ledger.FileLedger, n int, kind string) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1189, Kind: kind}); err != nil {
			t.Fatalf("append %s #%d: %v", kind, i, err)
		}
	}
}

func corruptLine(t *testing.T, evolveDir string, index int) {
	t.Helper()
	path := filepath.Join(evolveDir, "ledger.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if index < 0 || index >= len(lines) {
		t.Fatalf("corrupt index %d out of range (%d lines)", index, len(lines))
	}
	lines[index] = strings.Replace(lines[index], `"role":"scout"`, `"role":"TAMPERED"`, 1)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
}

func TestC1189_005_LedgerVerifyGreenWhenBreakPrecedesSealAnchor(t *testing.T) {
	l, dir := newFixtureLedger(t)
	appendN(t, l, 4, "phase")
	corruptLine(t, dir, 1)

	if err := l.Verify(context.Background()); err == nil {
		t.Fatalf("fixture invalid: an un-sealed pre-existing break must be BROKEN before the anchor is written (got nil)")
	}

	if err := l.Append(context.Background(), core.LedgerEntry{
		Role: "operator", Cycle: 1189, Kind: sealKindPrefix + "cycle1189",
		Message: "operator sign-off: historical damage preserved, chain resumes here",
	}); err != nil {
		t.Fatalf("append seal entry: %v", err)
	}
	appendN(t, l, 2, "phase")

	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("break precedes the last %s* operator entry, so Verify must report the sealed prefix as informational and return nil; got: %v", sealKindPrefix, err)
	}
}

func TestC1189_006_LedgerVerifyStillBrokenWhenBreakFollowsSealAnchor(t *testing.T) {
	l, dir := newFixtureLedger(t)
	appendN(t, l, 3, "phase")
	if err := l.Append(context.Background(), core.LedgerEntry{
		Role: "operator", Cycle: 1189, Kind: sealKindPrefix + "cycle1189",
		Message: "operator sign-off",
	}); err != nil {
		t.Fatalf("append seal entry: %v", err)
	}
	appendN(t, l, 4, "phase")
	corruptLine(t, dir, 5)

	err := l.Verify(context.Background())
	if err == nil {
		t.Fatalf("a break AFTER the last %s* anchor MUST stay BROKEN — sealing the past must not blanket-silence Verify", sealKindPrefix)
	}
	if !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("post-anchor break reported %v; want core.ErrLedgerChainBroken so the ship gate still trips", err)
	}
}
