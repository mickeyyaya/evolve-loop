//go:build acs

// Package cycle274 materializes the cycle-274 acceptance criteria for the three
// committed top_n tasks (triage-report.md):
//
//	T1  bridge-transport-manifest         — callers stop branching on CLI-name
//	                                          strings; transport is manifest data
//	G   inserted-phase-treediff-guard-gap — a phase that writes the main tree
//	                                          outside its worktree FAILs the cycle
//	                                          REGARDLESS of phase identity
//	C   bridge-coverage-95                — go/internal/bridge total >= 95%
//
// These predicates are BEHAVIORAL (cycle-85 lesson): the load-bearing
// correctness checks (C274_003..C274_007) RUN the system under test as a real
// subprocess — `go test` over the bridge / core packages and a `go tool cover`
// total — and assert on the real `--- PASS:` lines, sub-case counts, and the
// coverage number. A magic string in a .go file cannot produce a named PASS
// line nor move the coverage total, so none of these is gameable by source
// editing alone.
//
// Two structural predicates carry explicit waivers because they assert an
// INVARIANT over source rather than a magic-string presence:
//   - C274_001 is a structural-ABSENCE check (the refactor must REMOVE the
//     `HasSuffix(..,"-tmux")` leak sites) — un-gameable by adding text, it can
//     only pass once the leaks are gone. This is scout T1's exact verifiableBy.
//   - C274_002 is a config-presence check (the manifest data file declares the
//     new single-source `transport` field) — the allowed-with-waiver category.
//
// AC map (1:1 with the architecture-design Requirements R1–R9):
//
//	R1            → TestC274_001 (no CLI-name transport leaks outside bridge)
//	R2,R4         → TestC274_002 (manifests declare transport: single source)
//	R2,R3         → TestC274_003 (IsTmuxDriver/IsTmux classify + preserve ""→false)
//	R9            → TestC274_004 (extracted isLegitimateMainTreePath classifier)
//	R5,R6         → TestC274_005 (cycle-270 replay: inserted/untracked leak FAILs)
//	R7            → TestC274_006 (no-fire companion: legit .evolve workspace write)
//	R8            → TestC274_007 (bridge total statement coverage >= 95%)
package cycle274

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// --- shared one-shot subprocess runners (one `go test` per package, reused) ---

var (
	bridgeOnce sync.Once
	bridgeOut  string
	coreOnce   sync.Once
	coreOut    string
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runBridgeSuite runs the bridge package suite (verbose, cache-defeated) ONCE
// per predicate process and returns combined stdout+stderr. The builder's new
// IsTmux/IsTmuxDriver table tests land here.
func runBridgeSuite(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	bridgeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/bridge/")
		bridgeOut = stdout + "\n" + stderr
	})
	return bridgeOut
}

// runCoreSuite runs the orchestrator (core) guard tests ONCE per process. The
// builder's cycle-270 replay + classifier + no-fire companion land here.
func runCoreSuite(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	coreOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestIsLegitimateMainTreePath|TestGuardCatchesInsertedPhaseLeak|TestGuardIgnoresLegitimateWorkspaceWrite",
			"./internal/core/")
		coreOut = stdout + "\n" + stderr
	})
	return coreOut
}

var (
	topPassRe  = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	covTotalRe = regexp.MustCompile(`(?m)^total:\s+\(statements\)\s+([0-9.]+)%`)
)

// topLevelPassed reports whether a column-0 `--- PASS: <name>` line is present.
func topLevelPassed(out, name string) bool {
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

// subPasses returns the distinct passing subtest names under `parent` (indented
// `--- PASS: <parent>/<sub>` lines). Two distinct passing sub-cases prove a
// table-driven test exercised more than one row — the lever that forces the
// adversarial negative alongside the positive (skills/adversarial-testing §6);
// a positive-only test is gameable by a no-op.
func subPasses(out, parent string) map[string]bool {
	re := regexp.MustCompile(`(?m)^\s+--- PASS: ` + regexp.QuoteMeta(parent) + `/(\S+)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	return seen
}

// ============================ T1 — transport manifest ========================

// --- C274_001 (R1): no package outside bridge branches on a CLI-name string ---
//
// acs-predicate: structural-absence — this is NOT a magic-string presence grep
// (the cycle-85 ban); it asserts the refactor REMOVED the four
// `HasSuffix(..,"-tmux")` leak sites + the `isTmuxFamilyCLI` helper from
// swarm/adapters/looppreflight. Adding text can never make it pass — only
// deleting the leaks can. Scout T1's exact verifiableBy. RED at baseline: 4
// HasSuffix sites + the helper definition still present.
func TestC274_001_NoCLINameTransportLeaks(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// git grep searches tracked working-tree files (incl. the builder's
	// uncommitted edits); :(exclude) drops the legitimate _test.go matches.
	stdout, _, _, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "grep", "-nE", `HasSuffix\([^)]*-tmux`,
		"--", "go/internal/swarm", "go/internal/adapters", "go/internal/looppreflight",
		":(exclude)*_test.go")
	if leaks := nonEmptyLines(stdout); len(leaks) > 0 {
		t.Errorf("RED: %d CLI-name transport leak site(s) remain (must route through bridge.IsTmuxDriver):\n%s",
			len(leaks), stdout)
	}
	// The observer's private isTmuxFamilyCLI helper is the same leak by another
	// name — its definition must be gone (body replaced by bridge.IsTmuxDriver).
	defOut, _, _, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "grep", "-n", "func isTmuxFamilyCLI",
		"--", "go/internal/adapters")
	if strings.TrimSpace(defOut) != "" {
		t.Errorf("RED: isTmuxFamilyCLI still defined — delete it and route through bridge.IsTmuxDriver:\n%s", defOut)
	}
}

// --- C274_002 (R2,R4): transport is single-source manifest DATA ---
//
// acs-predicate: config-check — asserts the new `transport` field exists in the
// manifest data files (the single home, R2), classified correctly: the four
// `*-tmux` manifests = "tmux", the three headless = "headless". RED at baseline:
// no manifest declares a transport field.
func TestC274_002_ManifestsDeclareTransport(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, "go", "internal", "bridge", "manifests")
	tmuxManifests := []string{"agy-tmux", "claude-tmux", "codex-tmux", "ollama-tmux"}
	headlessManifests := []string{"agy", "claude-p", "codex"}
	for _, name := range tmuxManifests {
		p := filepath.Join(dir, name+".json")
		if !acsassert.FileMatchesRegex(t, p, `"transport"\s*:\s*"tmux"`) {
			t.Errorf("RED: %s.json must declare \"transport\": \"tmux\" (single-source transport data)", name)
		}
	}
	for _, name := range headlessManifests {
		p := filepath.Join(dir, name+".json")
		if !acsassert.FileMatchesRegex(t, p, `"transport"\s*:\s*"headless"`) {
			t.Errorf("RED: %s.json must declare \"transport\": \"headless\" (single-source transport data)", name)
		}
	}
}

// --- C274_003 (R2,R3): IsTmuxDriver / Manifest.IsTmux classify + preserve
// empty/unknown-CLI -> non-tmux semantics ---
//
// Behavioral: the builder's in-package `TestIsTmuxDriver` must run+PASS, proving
// the helper exists and projects the manifest field. Requiring >= 3 passing
// sub-cases forces the full diversity the R3 contract demands: a *-tmux positive,
// a headless negative, AND the empty/unknown -> false edge (the
// core_adapter default-tmux-vs-raw-empty contract, K2). `TestManifestIsTmux`
// pins the method projection on the struct itself. RED: tests absent (no PASS
// lines).
func TestC274_003_IsTmuxDriverBehavior(t *testing.T) {
	out := runBridgeSuite(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: bridge suite has a FAIL line:\n%s", out)
	}
	if !topLevelPassed(out, "TestIsTmuxDriver") {
		t.Errorf("RED: TestIsTmuxDriver did not run+PASS — bridge.IsTmuxDriver(cli) is not implemented/tested")
	}
	if subs := subPasses(out, "TestIsTmuxDriver"); len(subs) < 3 {
		t.Errorf("RED: TestIsTmuxDriver has %d passing sub-cases, want >= 3 "+
			"(a *-tmux positive, a headless negative, and the empty/unknown->false edge — R3)", len(subs))
	}
	if !topLevelPassed(out, "TestManifestIsTmux") {
		t.Errorf("RED: TestManifestIsTmux did not run+PASS — Manifest.IsTmux() projection is not implemented/tested")
	}
}

// ===================== G — inserted-phase tree-diff guard ====================

// --- C274_004 (R9): the legitimate-main-tree-path classifier is extracted and
// shared (no duplicated allowlist) ---
//
// Behavioral: the builder extracts the `.evolve/**` + build-artifact carve-out
// into one predicate `isLegitimateMainTreePath` consulted by BOTH the
// build-leak-recover path and the new every-boundary check. `TestIsLegitimate
// MainTreePath` must run+PASS with >= 2 sub-cases (a legit path AND a real
// source path that is NOT legit — the negative is the anti-no-op signal). RED:
// classifier + test absent.
func TestC274_004_LegitMainTreePathClassifierExtracted(t *testing.T) {
	out := runCoreSuite(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: core guard suite has a FAIL line:\n%s", out)
	}
	if !topLevelPassed(out, "TestIsLegitimateMainTreePath") {
		t.Errorf("RED: TestIsLegitimateMainTreePath did not run+PASS — the shared legitimate-path classifier (R9) is not extracted/tested")
	}
	if subs := subPasses(out, "TestIsLegitimateMainTreePath"); len(subs) < 2 {
		t.Errorf("RED: TestIsLegitimateMainTreePath has %d passing sub-cases, want >= 2 "+
			"(a legit `.evolve/**`/build-artifact path AND a real source path that must NOT be legit)", len(subs))
	}
}

// --- C274_005 (R5,R6): cycle-270 replay — an INSERTED, non-worktree phase that
// writes a NEW UNTRACKED main-tree file FAILs the cycle with the path named ---
//
// THE core gap. Behavioral: the builder's `TestGuardCatchesInsertedPhaseLeak`
// drives the orchestrator (real git repo + a fake inserted/non-worktree runner
// that writes an untracked .go file into the main tree) and asserts the cycle
// aborts with the leaked path named. Requiring >= 2 passing sub-cases forces
// BOTH the untracked-file leak (R6, porcelain granularity — the tracked-only
// `git diff --name-only HEAD` baseline missed it) AND the inserted-phase-
// identity dimension (R5 — the phase is neither tdd/build nor a WritesSource
// catalog phase, the exact cycle-270 escape). RED: test absent.
func TestC274_005_GuardCatchesInsertedUntrackedLeak(t *testing.T) {
	out := runCoreSuite(t)
	if !topLevelPassed(out, "TestGuardCatchesInsertedPhaseLeak") {
		t.Errorf("RED: TestGuardCatchesInsertedPhaseLeak did not run+PASS — an inserted/non-worktree phase's untracked main-tree leak still slips past the guard (cycle-270 defect, R5+R6)")
	}
	if subs := subPasses(out, "TestGuardCatchesInsertedPhaseLeak"); len(subs) < 2 {
		t.Errorf("RED: TestGuardCatchesInsertedPhaseLeak has %d passing sub-cases, want >= 2 "+
			"(an untracked-file leak detected [R6] AND an inserted-phase-identity leak detected [R5])", len(subs))
	}
}

// --- C274_006 (R7): no-fire companion — a non-worktree phase that writes ONLY
// its legitimate `.evolve/runs/cycle-N/<phase>-report.md` workspace artifact
// does NOT trip the guard ---
//
// The adversarial NEGATIVE for the guard: now that the guard runs for every
// phase boundary (not just worktree phases), it must NOT false-fire on the
// workspace writes legitimate non-worktree phases always make. Behavioral: the
// builder's `TestGuardIgnoresLegitimateWorkspaceWrite` must run+PASS. Without
// this pin, a guard that simply aborts on ANY main-tree change would pass
// C274_005 while breaking every real cycle (R7). RED: test absent.
func TestC274_006_GuardIgnoresLegitimateWorkspaceWrite(t *testing.T) {
	out := runCoreSuite(t)
	if !topLevelPassed(out, "TestGuardIgnoresLegitimateWorkspaceWrite") {
		t.Errorf("RED: TestGuardIgnoresLegitimateWorkspaceWrite did not run+PASS — the every-boundary guard must NOT false-fire on legitimate `.evolve/` workspace writes (R7)")
	}
}

// ========================= C — bridge coverage >= 95% ========================

// --- C274_007 (R8): go/internal/bridge total statement coverage >= 95% ---
//
// Load-bearing behavioral/objective: the number is produced by REALLY running
// the bridge suite over the package with -coverprofile, so it can only clear 95%
// once the builder's new BootSmoke nil-cfg / ResolveStage branch tests actually
// exercise the previously-uncovered lines. Un-gameable by source editing. RED at
// baseline: 94.5%.
func TestC274_007_BridgeCoverageAtLeast95(t *testing.T) {
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-short", "-count=1",
		"-coverprofile="+prof, "./internal/bridge/...")
	funcOut, funcErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	cov := parseCoverTotal(funcOut)
	if cov < 0 {
		t.Fatalf("RED: no `total: (statements) N%%` line — coverage profile not produced.\ntest stderr:\n%s\ncover stderr:\n%s\ncover stdout:\n%s",
			stderr, funcErr, funcOut)
	}
	if cov < 95.0 {
		t.Errorf("RED: go/internal/bridge total coverage = %.1f%%, want >= 95.0%% (baseline 94.5%%)", cov)
	}
}

// --- small helpers ---

func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

func parseCoverTotal(out string) float64 {
	m := covTotalRe.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}
