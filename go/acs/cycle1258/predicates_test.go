//go:build acs

// Package cycle1258 materialises the cycle-1258 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	artifact-ready-crosspoll-debounce — artifactDetector.poll must not declare a
//	phase complete on the FIRST tick it sees a non-empty deliverable; the
//	(path, size, mtime) key must be observed unchanged across artifactStableTicks
//	consecutive wait-loop ticks first.
//
// # Standing state this lane inherited (read this before diagnosing a red)
//
// The production change is ALREADY PRESENT in this worktree's base commit
// (29472ec3, the ADR-0076 continuation-on-fail salvage snapshot): completion.go
// carries artifactStableTicks and the cross-poll window, salvaged forward from
// the cycle-1233 → 1249 → 1252 → 1254 chain. Scout re-derived the gap from the
// deliverable.go:178-181 comment and did not observe that the fix had landed.
//
// That does NOT make these predicates ceremonial. They are the REGRESSION LOCK
// on a fix that has now been salvaged across four cycles without ever being
// bound by a cycle predicate — precisely the shape that lets a rebase, a
// re-salvage, or a "simplify the detector" refactor silently reopen cycle-1198.
// Predicate 005 is the genuine RED: the permanent eval entry that survives THIS
// cycle (.evolve/evals/artifact-ready-crosspoll-debounce.md) does not exist, so
// nothing caps a future audit when this behaviour breaks.
//
// # Predicate strategy
//
// artifactDetector, artifactStableTicks and runTmuxREPL are all UNEXPORTED in
// internal/bridge, so a predicate in this package cannot call them directly.
// Each behavioural predicate therefore drives the real production code through a
// NARROW, named-package `go test` subprocess (never a `./...` sweep, never a
// known-slow suite — see the flaky-predicate-shape rules), with cmd.Dir pinned
// to the worktree's go/ directory rather than inherited from process cwd.
//
//	001 → the stability window itself: positive + BOTH negative axes (size, mtime)
//	002 → the caller proof: the debounce is reached from the production wait loop
//	003 → no false-timeout regression + the window constant is a real window
//	004 → AC-3: the window gained no operator dial (registry is the SSOT)
//	005 → the permanent eval entry exists, is well formed, and its evidence RUNS
package cycle1258

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"gopkg.in/yaml.v3"
)

// evalRelPath is the permanent regression entry this cycle must leave behind.
// The slug is exact from scout-report.md / triage-report.md's top_n row.
const evalRelPath = ".evolve/evals/artifact-ready-crosspoll-debounce.md"

// bridgePkg is the ONE named package every behavioural predicate here drives.
// Named, never a `/...` sweep, and not one of the known 40s+ suites
// (./internal/core, ./cmd/evolve) — each invocation is further narrowed by -run.
const bridgePkg = "./internal/bridge"

// subprocessBudget bounds a single narrow `go test` invocation so a hung
// subprocess surfaces as a failure instead of wedging the predicate lane. It
// guards the CHILD PROCESS; it is not a wall-clock assertion about the code
// under test (which would stretch arbitrarily under fleet contention).
const subprocessBudget = 4 * time.Minute

// goTestDir returns the worktree's go/ directory — the cmd.Dir for every
// subprocess below. Resolved from acsassert.RepoRoot rather than process cwd,
// which differs between the main tree, this worktree, and each fleet lane.
func goTestDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("worktree go module not found at %s: %v", dir, err)
	}
	return dir
}

// runNarrowGoTest executes `go test -count=1 -run <pattern> <bridgePkg>` in the
// worktree and returns combined output plus whether it passed. -count=1 defeats
// the test cache, so a green here is a green NOW against the committed tree, not
// a replay of an earlier run.
func runNarrowGoTest(t *testing.T, pattern string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subprocessBudget)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", pattern, bridgePkg)
	cmd.Dir = goTestDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// assertNamedTestsRan fails when `go test -run` matched NOTHING. A -run pattern
// that matches no test exits 0 with "no tests to run" / "testing: warning" — so
// a predicate that only checks the exit code silently passes after someone
// renames or deletes the test it claims to enforce. This is the difference
// between "the contract holds" and "the contract is gone".
func assertNamedTestsRan(t *testing.T, out string, want ...string) {
	t.Helper()
	if strings.Contains(out, "no tests to run") || strings.Contains(out, "no test files") {
		t.Errorf("go test matched NO tests — the contract it enforces has been renamed or deleted:\n%s", out)
		return
	}
	for _, name := range want {
		if !strings.Contains(out, name) {
			t.Errorf("test %s did not appear in the run output — it was renamed or removed, "+
				"so this predicate is no longer enforcing anything:\n%s", name, out)
		}
	}
}

// verboseRun is runNarrowGoTest with -v, used where the predicate must PROVE the
// individual named tests executed (assertNamedTestsRan needs the -v RUN lines).
func verboseRun(t *testing.T, pattern string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subprocessBudget)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, bridgePkg)
	cmd.Dir = goTestDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// --- AC-1: the cross-poll stability window ----------------------------------

// TestC1258_001_ArtifactDebounceStabilityWindow is AC-1, all three axes at once.
// It drives the REAL artifactDetector against a real temp-dir artifact:
//
//   - positive — a settled file completes, but never on the tick it is first
//     seen (cycle-1198's truncated deliverable was a perfectly non-empty file on
//     exactly that tick);
//   - negative, SIZE axis — a still-growing deliverable never completes, however
//     many ticks pass. This is the anti-no-op assertion: a detector that keeps
//     firing on first sight passes the positive half and fails here;
//   - negative, MTIME axis — an equal-length fix-up Edit (a flipped verdict word)
//     must also reset the window. A size-only key silently degrades back to the
//     cycle-1198 bug for exactly that shape, which is why mtime is in the key.
func TestC1258_001_ArtifactDebounceStabilityWindow(t *testing.T) {
	const pattern = `^TestArtifactDetector_(ReadyOnlyAfterCrossPollStability|NotReadyWhileArtifactStillGrowing|NotReadyOnSameSizeRewrite)$`
	out, ok := verboseRun(t, pattern)
	assertNamedTestsRan(t, out,
		"TestArtifactDetector_ReadyOnlyAfterCrossPollStability",
		"TestArtifactDetector_NotReadyWhileArtifactStillGrowing",
		"TestArtifactDetector_NotReadyOnSameSizeRewrite",
	)
	if !ok {
		t.Errorf("the artifact cross-poll stability window is not honoured by artifactDetector.poll — "+
			"a deliverable is accepted while it is still being written (cycle-1198):\n%s", out)
	}
}

// --- AC-1 (wiring): the debounce is REACHED from production -----------------

// TestC1258_002_ArtifactDebounceWiredIntoWaitLoop is the CALLER PROOF, and it is
// the predicate that matters most. A stability window implemented on a struct
// that no production path reaches is dead code that greens 001 and changes
// nothing at runtime. The tests it drives go through Engine.LaunchArgs →
// runTmuxREPL → detector.poll — the real wait loop — and assert that a
// deliverable rewritten on every tick exits ExitArtifactTimeout rather than
// ExitOK.
//
// The hermetic twin is bound here deliberately rather than left to general
// hygiene: cycles 1252 and 1254 were both FAILed at audit by this exact caller
// proof going red inside the ACS gate (which shells `go test` with no cmd.Env
// and so inherits the orchestrator's EVOLVE_FLEET=1) while every interactive run
// was green. The invariant that fixture construction must not read the ambient
// environment is part of this task's contract, not an unrelated cleanup.
func TestC1258_002_ArtifactDebounceWiredIntoWaitLoop(t *testing.T) {
	const pattern = `^TestRunTmuxREPL_ArtifactDebounce(WiredIntoWaitLoop|HermeticUnderAmbientFleetEnv)$`
	out, ok := verboseRun(t, pattern)
	assertNamedTestsRan(t, out,
		"TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop",
		"TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv",
	)
	if !ok {
		t.Errorf("the cross-poll debounce is NOT reached from the production wait loop "+
			"(runTmuxREPL), or the driver fixtures read the ambient process environment again "+
			"(the cycle-1252/1254 green-locally/red-in-gate defect):\n%s", out)
	}
}

// --- AC-2: the debounce must not become a false-FAIL generator --------------

// TestC1258_003_DebounceCostsNoFalseTimeouts is AC-2 — the half of this task that
// is easy to lose. A debounce is trivial to "pass" by simply waiting longer; the
// acceptance bar is that it delays completion ONLY when the artifact is observed
// to CHANGE, and never turns a finished session into a timeout. Three contracts:
//
//   - the legacy TestArtifactDetector_Poll parity case (written once, stable from
//     first sight) still completes;
//   - the wait loop's ONE post-cancel final poll short-circuits the window —
//     otherwise every finished-at-the-buzzer session launders into
//     ExitArtifactTimeout, which is strictly worse than the truncated read this
//     task set out to fix — while still refusing to manufacture completion from
//     an absent artifact;
//   - artifactStableTicks is a MEANINGFUL window: a builder can green every other
//     predicate here by defining it as 1, which is arithmetically one observation
//     and no debounce at all.
func TestC1258_003_DebounceCostsNoFalseTimeouts(t *testing.T) {
	const pattern = `^(TestArtifactDetector_Poll|TestArtifactDetector_CtxCancelledShortCircuitsDebounce|TestArtifactStableTicks_IsAMeaningfulWindow)$`
	out, ok := verboseRun(t, pattern)
	assertNamedTestsRan(t, out,
		"TestArtifactDetector_Poll",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
		"TestArtifactStableTicks_IsAMeaningfulWindow",
	)
	if !ok {
		t.Errorf("the debounce regressed the no-false-timeout contract, or artifactStableTicks is "+
			"no longer a real window (< 2 observations is not a debounce; > 3 underruns the "+
			"short-ArtifactTimeoutS fixtures at ~2s per tick):\n%s", out)
	}
}

// --- AC-3: no new operator dial ---------------------------------------------

// dialSubstrings are the name fragments an artifact-stability dial would carry.
// Matched case-insensitively against every registry row name.
var dialSubstrings = []string{
	"ARTIFACT_STABLE",
	"STABLE_TICKS",
	"STABLE_POLL",
	"ARTIFACT_DEBOUNCE",
	"DEBOUNCE",
	"ARTIFACT_SETTLE",
}

// TestC1258_004_StabilityWindowIsNotConfigurable is AC-3. The window is a
// compiled constant on purpose, matching readGraceWindow's "deliberately NOT
// configurable" convention already established in this codebase and the standing
// no-feature-flags rule: cross-component timing behaviour belongs in the code
// path, not in an env dial an operator can quietly set to 1 and reopen
// cycle-1198 in production without changing a line of source.
//
// The load-bearing assertion executes flagregistry.All — the SSOT the
// control-flags doc is GENERATED from and that `evolve flags check` enforces —
// so a dial registered anywhere on any reader surface is caught. The source-form
// check that follows is a secondary net for a dial added WITHOUT registering it
// (which would itself fail the registry drift test in normal CI).
func TestC1258_004_StabilityWindowIsNotConfigurable(t *testing.T) {
	for _, f := range flagregistry.All {
		up := strings.ToUpper(f.Name)
		for _, frag := range dialSubstrings {
			if strings.Contains(up, frag) {
				t.Errorf("flag %q (status=%s) registers an artifact-stability dial — the cross-poll "+
					"window must stay a compiled constant (no_feature_flags; readGraceWindow's "+
					"\"deliberately NOT configurable\" convention)", f.Name, f.Status)
			}
		}
	}

	completion := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge", "completion.go")
	if !acsassert.FileMatchesRegex(t, completion, `(?m)^const artifactStableTicks = \d+$`) {
		t.Error("artifactStableTicks is no longer a package-level compiled const in completion.go — " +
			"a var or a Config field is a dial by another name")
	}
	// The detector must not grow its own env reader either.
	if !acsassert.FileNotContains(t, completion, "os.Getenv") {
		t.Error("completion.go now reads os.Getenv — the completion contract must be selected by " +
			"newCompletionDetector's mode argument, never by ambient environment")
	}
}

// --- AC-4: the permanent regression entry -----------------------------------

// scoreCapEntry mirrors one row of the eval frontmatter's score_cap list.
type scoreCapEntry struct {
	Criterion    string `yaml:"criterion"`
	MaxIfMissing int    `yaml:"max_if_missing"`
	Evidence     string `yaml:"evidence"`
}

type evalFrontmatter struct {
	ScoreCap []scoreCapEntry `yaml:"score_cap"`
}

// parseFrontmatter extracts and unmarshals the leading `---` YAML block.
func parseFrontmatter(t *testing.T, raw string) evalFrontmatter {
	t.Helper()
	if !strings.HasPrefix(raw, "---\n") {
		t.Fatalf("%s does not open with a `---` YAML frontmatter block", evalRelPath)
	}
	end := strings.Index(raw[4:], "\n---")
	if end < 0 {
		t.Fatalf("%s has an unterminated YAML frontmatter block", evalRelPath)
	}
	var fm evalFrontmatter
	if err := yaml.Unmarshal([]byte(raw[4:4+end]), &fm); err != nil {
		t.Fatalf("%s frontmatter is not valid YAML: %v", evalRelPath, err)
	}
	return fm
}

// TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns is this cycle's
// genuine RED, and the reason the lane is worth running at all.
//
// ACS predicates are CYCLE-SCOPED: this whole package is read once, by cycle
// 1258's audit, and never replayed. The debounce has now been carried forward by
// salvage across cycles 1233 → 1249 → 1252 → 1254 with no permanent artifact
// binding it, which is exactly how a fix survives four cycles and still has
// nothing to stop the fifth from reverting it. The eval entry under
// .evolve/evals/ is the durable half: it caps a FUTURE cycle's audit score when
// this evidence stops holding.
//
// The predicate does not merely assert the file exists — a plausible stub would
// pass that. It EXECUTES every `evidence` command the eval declares and requires
// each to exit 0, so an eval whose evidence is aspirational, misspelled, or
// pointing at a renamed test fails here rather than silently capping nothing
// forever.
func TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, evalRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("permanent eval entry %s is absent: %v — the cross-poll debounce has been salvaged "+
			"across four cycles with no durable regression entry; nothing caps a future audit when "+
			"artifactDetector loses its stability window again", evalRelPath, err)
	}
	fm := parseFrontmatter(t, string(raw))
	if len(fm.ScoreCap) == 0 {
		t.Fatalf("%s declares no score_cap entries — an eval with no cap enforces nothing", evalRelPath)
	}

	for i, e := range fm.ScoreCap {
		if strings.TrimSpace(e.Criterion) == "" {
			t.Errorf("score_cap[%d] has an empty criterion", i)
		}
		if e.MaxIfMissing < 1 || e.MaxIfMissing > 10 {
			t.Errorf("score_cap[%d] max_if_missing = %d, want an integer in 1..10", i, e.MaxIfMissing)
		}
		if strings.TrimSpace(e.Evidence) == "" {
			t.Errorf("score_cap[%d] declares no evidence command", i)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), subprocessBudget)
		cmd := exec.CommandContext(ctx, "sh", "-c", e.Evidence)
		cmd.Dir = root
		cmd.WaitDelay = 10 * time.Second
		out, runErr := cmd.CombinedOutput()
		cancel()
		if runErr != nil {
			t.Errorf("score_cap[%d] evidence %q did not exit 0: %v — an eval whose evidence cannot "+
				"run caps nothing, forever:\n%s", i, e.Evidence, runErr, string(out))
		}
	}

	// The entry must name the incident it descends from, so a future reader can
	// tell a load-bearing cap from a decorative one.
	if !strings.Contains(string(raw), "1198") {
		t.Errorf("%s does not cite the source incident (cycle-1198, the truncated deliverable "+
			"accepted on first sight) — an eval without its provenance gets deleted by the next "+
			"person who tidies up", evalRelPath)
	}
}
