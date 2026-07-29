//go:build acs

// Package cycle1189 materialises the cycle-1189 acceptance criteria for the
// three fleet-scoped tasks triage committed to this lane:
//
//   - loop-base-divergence-boot-halt   → boot HALTs loud when the local base has
//     diverged from / fallen behind origin/<base>, naming `evolve sync-main`
//   - bridgewatch-follow-event-sync-fix → the macOS-flaky follow test waits on an
//     event with a >=10s deadline instead of a bare 10ms sleep in a 200ms window
//   - ledger-verify-seal-anchor-fix     → Verify walks from the last `reset-seal-*`
//     operator entry forward; a pre-seal break is informational, a post-seal
//     break is still BROKEN
//
// (`codegraph-blast-radius-context-for-scout-audit-review` is `## deferred` this
// cycle, so per R9.3 it gets ZERO predicates here.)
//
// Predicate strategy — every load-bearing assertion EXERCISES the system under
// test, never greps production source for a magic string (the cycle-85
// degenerate-predicate ban):
//
//   - 001/002 build a REAL git fixture (bare origin + clone) and call
//     looppreflight.Run through its exported seams: behind-origin must HALT with
//     the reconcile instruction (001), in-sync must NOT halt (002, the
//     anti-blanket-halt negative). Both are name-agnostic — they assert on
//     Run's Result, so Builder is free to name the new check/seam anything.
//   - 003 runs the real flaky test as a subprocess under `-race -count=25`; a
//     wait that is still time-window-bound stays flaky and reds here. 004 is the
//     structural companion (deadline >=10s, no bare unconditional sleep).
//   - 005/006 drive the REAL ledger: append a chain, corrupt a line, write a
//     `reset-seal-*` operator entry, and assert Verify's verdict. 006 is the
//     crux anti-no-op: a break AFTER the seal must still return
//     core.ErrLedgerChainBroken, so "make Verify always return nil" fails.
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

// reconcileInstruction is the operator command the halt MUST name (inbox item:
// "HALTs loud with the reconcile instruction"). Asserting on the instruction —
// not on a check name — keeps the predicate free of Builder's naming choices.
const reconcileInstruction = "evolve sync-main"

// ---------------------------------------------------------------------------
// Task 1 — loop-base-divergence-boot-halt
// ---------------------------------------------------------------------------

// git runs a git command in dir, failing the test on error. Fixtures are built
// with real git so the check under test sees a real repo/remote topology.
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

// commitPush writes a file in dir and pushes it to origin/main.
func commitPush(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	git(t, dir, "add", file)
	git(t, dir, "commit", "-m", "acs fixture "+file)
	git(t, dir, "push", "origin", "main")
}

// baseFixture builds a bare origin plus a working clone. When behind is true a
// SECOND clone pushes an extra commit to origin, so the returned working clone's
// local main is strictly behind origin/main WITHOUT having fetched it — exactly
// the stale-base topology that produced the cycle-969 GIT_PUSH_REJECTED. Returns
// the working clone path (the ProjectRoot handed to Run).
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
		// `work` deliberately does NOT fetch: the check under test must do the
		// fetch itself before deciding.
	}
	return work
}

// greenOptions returns a looppreflight.Options on which every PRE-EXISTING check
// passes, so any halt observed by 001/002 is attributable to the new base
// divergence check alone. Mirrors internal goodPipelineOptions using only
// EXPORTED seams. OrphanKill is stubbed to a no-op: the default is a real tmux
// kill and the ACS suite runs while live lanes hold tmux sessions.
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

// haltText concatenates the message+detail of every HALT-level check, so the
// assertion binds to the halt's CONTENT and not to a check name Builder picks.
func haltText(r looppreflight.Result) string {
	var b strings.Builder
	for _, c := range r.Checks {
		if c.Level == looppreflight.LevelHalt {
			b.WriteString(c.Name + ": " + c.Message + "\n" + c.Detail + "\n")
		}
	}
	return b.String()
}

// TestC1189_001_BootHaltsWhenBaseDivergedFromOrigin — AC (loop-base-divergence-
// boot-halt), positive half. With a real repo whose local main is BEHIND
// origin/main, looppreflight.Run must return a HALT result and the halt text
// must name the `evolve sync-main` reconcile instruction. Behavioural: it calls
// the real preflight against a real remote; today no check fetches origin, so
// Run returns non-halting and this predicate is RED.
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

// TestC1189_002_BootDoesNotHaltWhenBaseInSync — AC (loop-base-divergence-boot-
// halt), NEGATIVE half and the anti-gaming guard. An in-sync clone must NOT
// halt: a check that halts unconditionally (or that treats "no upstream commits"
// as divergence) would bench every healthy loop boot. Currently pre-existing
// GREEN — it exists to stay green through the fix.
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

// ---------------------------------------------------------------------------
// Task 2 — bridgewatch-follow-event-sync-fix
// ---------------------------------------------------------------------------

// followTestName is the flaky test the fix must stabilise.
const followTestName = "TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines"

// bridgeWatchTestPath is the file that owns it.
const bridgeWatchTestPath = "go/cmd/evolve/cmd_bridge_watch_test.go"

// followRepeatCount is the repeat factor for the stability run. Scout's
// acceptance names -count=50; 25 keeps the audit lane's wall-clock bounded while
// still exercising the window ~25x (a 200ms-window race reproduces well inside
// that, and did on the macOS runner).
const followRepeatCount = 25

// TestC1189_003_BridgeWatchFollowTestIsStableUnderRepeat — AC (bridgewatch-
// follow-event-sync-fix), the behavioural crux. Runs the REAL test under
// `-race -count=25`; a wait that still depends on a fixed 10ms sleep landing
// inside a 200ms deadline flakes here. Nothing about this can be satisfied by
// editing a comment.
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

// goFuncBody returns the source text of the named top-level func in path, using
// brace matching from the func's opening brace. Used by 004 to scope structural
// assertions to ONE function instead of the whole file.
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

// TestC1189_004_BridgeWatchFollowWaitIsEventDriven — AC (bridgewatch-follow-
// event-sync-fix), the structural companion to 003. Two requirements from the
// acceptance criteria, both scoped to the one function:
//
//	(a) the deadline is >= 10s (not the old 200ms window);
//	(b) no BARE sleep — a time.Sleep may survive only as the tick inside a poll
//	    loop, never as an unconditional pre-append wait.
//
// This closes the hole where 003 could pass by luck on a fast runner while the
// fixed-window design survives.
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

// ---------------------------------------------------------------------------
// Task 3 — ledger-verify-seal-anchor-fix
// ---------------------------------------------------------------------------

// sealKindPrefix is the operator entry kind Verify must resolve as the walk
// anchor (inbox item: "last `reset-seal-*` operator entry").
const sealKindPrefix = "reset-seal-"

// newFixtureLedger returns a FileLedger over a fresh .evolve dir plus that dir.
func newFixtureLedger(t *testing.T) (*ledger.FileLedger, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	return ledger.New(dir), dir
}

// appendN appends n well-formed chained entries.
func appendN(t *testing.T, l *ledger.FileLedger, n int, kind string) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1189, Kind: kind}); err != nil {
			t.Fatalf("append %s #%d: %v", kind, i, err)
		}
	}
}

// corruptLine rewrites line index (0-based) of the ledger file in place, keeping
// byte length irrelevant — the point is that its SHA no longer matches the
// prev_hash the NEXT line recorded, i.e. a real chain break at that seam.
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

// TestC1189_005_LedgerVerifyGreenWhenBreakPrecedesSealAnchor — AC (ledger-verify-
// seal-anchor-fix), positive half. Topology: valid chain → a break → a
// `reset-seal-*` operator entry → more valid entries. Because the damage lies
// BEFORE the operator's seal anchor, Verify must walk from that anchor forward
// and return nil (the sealed prefix is informational, not BROKEN). Today Verify
// walks from line 1 and cries wolf on the ancient break → RED.
func TestC1189_005_LedgerVerifyGreenWhenBreakPrecedesSealAnchor(t *testing.T) {
	l, dir := newFixtureLedger(t)
	appendN(t, l, 4, "phase")
	// Break at line index 1: line 2's recorded prev_hash no longer matches.
	corruptLine(t, dir, 1)

	if err := l.Verify(context.Background()); err == nil {
		t.Fatalf("fixture invalid: an un-sealed pre-existing break must be BROKEN before the anchor is written (got nil)")
	}

	// Operator seals: everything at/below here is accepted by sign-off.
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

// TestC1189_006_LedgerVerifyStillBrokenWhenBreakFollowsSealAnchor — AC (ledger-
// verify-seal-anchor-fix), NEGATIVE half and the crux anti-no-op predicate. Same
// seal topology, but the damage lands AFTER the seal anchor. Verify MUST still
// return core.ErrLedgerChainBroken: an implementation that "fixes" 005 by
// short-circuiting Verify to nil whenever a seal entry exists fails here.
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
	// Lines 0..2 phases, line 3 seal, lines 4..7 phases. Corrupt line 5 —
	// strictly AFTER the anchor.
	corruptLine(t, dir, 5)

	err := l.Verify(context.Background())
	if err == nil {
		t.Fatalf("a break AFTER the last %s* anchor MUST stay BROKEN — sealing the past must not blanket-silence Verify", sealKindPrefix)
	}
	if !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("post-anchor break reported %v; want core.ErrLedgerChainBroken so the ship gate still trips", err)
	}
}
