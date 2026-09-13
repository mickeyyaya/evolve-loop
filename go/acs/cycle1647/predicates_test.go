//go:build acs

// Package cycle1647 materializes the acceptance criteria of this fleet lane's
// single committed task:
//
//   - overlay-family-name-transport-ambiguity (triage `## top_n`; the lane
//     pin in lane-scope.json names the same id) — a routing overlay naming a
//     bare FAMILY must not cross transport when the phase's chain holds a
//     non-default driver of that family, on the ADVISOR projection path.
//
// The acceptance is the claimed inbox record's `acceptance` array, verbatim
// (.evolve/inbox/processing/cycle-1647/2026-07-30T23-05-00Z-overlay-family-
// name-transport-ambiguity.json). The task-contract block rendered the record
// as unreadable because triage had already moved it from pending to
// processing/; the array is unchanged by the move.
//
// # What is already built, and what this package adds
//
// Commit 797b8518 landed the RESOLVER half: llmroute.ApplySoftOverlay resolves
// ov.CLI in three rungs (exact chain entry > bare-family same-family entry >
// defaultDriverForFamily), and overlay_family_transport_test.go pins it at the
// helper. Verified here by driving the callers, not by reading code:
//
//  1. THE PACKAGE DOC DOES NOT CARRY THE DECISION. `go doc ./internal/llmroute`
//     prints the package comment from llmroute.go, which still describes the
//     pre-overlay precedence table and says nothing about family-vs-driver
//     selector semantics; the decision lives only on the ApplySoftOverlay
//     function comment. AC1 asks for one sentence in the PACKAGE doc — the
//     ambiguity was the defect, so the decision must be discoverable at the
//     package boundary. 001 is RED.
//  2. THE PRODUCTION ADVISOR CALLER IS UNPROVEN. runner.resolveDispatchPlan
//     (internal/phases/runner/routing.go:71) is where the advisor projection
//     reaches ApplySoftOverlay. No runner test drives a bare-family overlay
//     over a headless chain, and none drives an explicit driver over a
//     same-family headless entry: `go test -run
//     '^TestResolveRouting_AdvisorOverlayPreservesFamilyTransport$'
//     ./internal/phases/runner` reports "no tests to run". 004 is RED.
//  3. THE BEHAVIOR ITSELF, THROUGH THE RUNNER, IS ALREADY CORRECT. 002 and
//     003 drive runner.New(...).Run — the real BaseRunner over an on-disk
//     profile and a recording bridge — so resolveDispatchPlan is reached from
//     its production entry point, and they observe the dispatched CLI. Both
//     are GREEN on this tree (the resolver fix is merged and the runner passes
//     the advisor's string through un-normalized). They stay as the
//     anti-gaming floor: a Builder who satisfies 004 with a trivially-passing
//     named test still cannot regress the transport contract without 002/003
//     going red, and a future caller that normalizes ModelRoutingCLI before
//     the seam (the scout's beyond-the-ask hypothesis) trips 002 directly.
//
// Adversarial axes (skills/adversarial-testing §6): NEGATIVE — 002 forbids
// claude-tmux ever being dispatched for the bare overlay (the historical
// crossing); 003 forbids the naive family-match answer (claude-p first) for an
// explicit claude-tmux. EDGE — 003 exercises the fallback walk (exit 80 on the
// explicit driver) to prove the headless entry is RETAINED, not replaced.
// SEMANTIC — family identity preserves transport (002), explicit driver
// changes it (003), the decision is documented at the package boundary (001),
// the projection path is covered and its caller named (004), the package
// stays race-clean with every export exercised (005), the predicate package
// and eval are git-tracked (006): six distinct behaviors, not one restated.
//
// # Audit round 1 (same cycle) — continuation hygiene, 007-008
//
// Round 1 passed 001-006 and FAILED the shipping tree: go/acs/cycle1638 (added
// by this diff, named as evidence by the tracked unified-synthesis eval)
// resolved its explanation by a cycle-1638-*.md glob the host archives on
// every continuation, so TestC1638_010/011 were RED and nobody ran them (H1);
// the 1647 explanation repeated the two defects they pin (M1) and cited the
// two root inbox records at `:1` although the diff deletes them (correction
// 3). Reconciled at the TDD seam: cycle1638's helper now resolves the record
// the tree ships; 007 runs every inherited go/acs/cycle* package the diff
// adds (derived from git, one named package each) so the harness lane reaches
// them; 008 forbids a `path:line` citation into a deleted path. All three go
// GREEN with edits to the DOCUMENT only (probed at RED).
//
// Flaky-shape hygiene: every `go` subprocess names ONE package (./internal/
// phases/runner with -run narrowed; ./internal/llmroute; ./internal/router —
// measured 1.3s / 1.9s -race / 1.3s), no `/...` sweep, no ./internal/core or
// ./cmd/evolve (the AC5 `-race ./internal/core` half is a whole-suite shape
// the predicate lint bans; it is delegated to CI + the Builder's pasted run,
// see test-report.md), no wall-clock bounds, no literal PIDs, every git call
// is -C rooted, every go call sets cmd.Dir, no load generators. 007's nested
// runs each name ONE inherited ./acs/cycle<N> package (measured 1.0s / 1.8s /
// 3.1s), never ./acs/... .
package cycle1647

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	taskSlug = "overlay-family-name-transport-ambiguity"
	// focusedRunnerTest is the scout's VerifiableBy name: the ONE focused
	// runner test the Builder authors against the production advisor caller.
	focusedRunnerTest = "TestResolveRouting_AdvisorOverlayPreservesFamilyTransport"
	// contractEscalationTest is the pre-existing runner test for the OTHER
	// ApplySoftOverlay projection (core's correction ladder sets
	// ModelRoutingCLI on the re-dispatch). AC4 requires the advisor path to be
	// covered in addition to this one, and the callers named for each.
	contractEscalationTest = "TestRunner_ContractEscalation_RedispatchesOnEscalatedCLIWithDirective"
)

// goDir is <repo>/go — the module root every go subprocess below runs in via
// cmd.Dir (never process cwd: main tree, worktree and fleet lanes differ).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runGo runs ONE go subcommand rooted at dir and returns its combined output
// and exit code. A non-exit error (go missing from PATH) is fatal.
func runGo(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	return "", -1
}

// advisorHooks is the minimal runner.Hooks a dispatch needs: a phase name (the
// profile file), the agent doc to load, and a PASS classification of whatever
// the recording bridge wrote. It is deliberately NOT the runner package's
// fakeHooks — that lives in a _test.go the predicate cannot import — so the
// predicate reaches BaseRunner.Run exactly the way a real phase does.
type advisorHooks struct{ phase, agent string }

func (h advisorHooks) PhaseName() string                              { return h.phase }
func (h advisorHooks) AgentPromptName() string                        { return h.agent }
func (h advisorHooks) ArtifactFilename(core.PhaseRequest) string      { return h.phase + "-report.md" }
func (h advisorHooks) DefaultModel() string                           { return "sonnet" }
func (h advisorHooks) ComposePrompt(string, core.PhaseRequest) string { return "x" }
func (h advisorHooks) Classify(string, core.PhaseRequest, core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, ""
}

// recordingBridge records every CLI the runner dispatches, in order. A CLI in
// `trigger` returns that exit code with an error — a fallback trigger (80 =
// REPL boot timeout is in llmroute's default set) — so the walk advances to
// the next chain entry, which is how 003 observes the RETAINED fallback.
type recordingBridge struct {
	calls   []string
	trigger map[string]int
}

func (b *recordingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req.CLI)
	if code, ok := b.trigger[req.CLI]; ok {
		return core.BridgeResponse{ExitCode: code, Stderr: "scripted: REPL boot timeout"}, errors.New("bridge: launch exit=" + strconv.Itoa(code))
	}
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok-from-"+req.CLI), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok-from-" + req.CLI}, nil
}

func (b *recordingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// writeProfile materializes a per-phase profile under a throwaway project
// root: profile.cli is the RESOLVED primary (a headless driver here, the
// shape CI macOS runs), cli_fallback the rest of the resolved chain.
func writeProfile(t *testing.T, agent, cli string, fallback []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	fb := ""
	if len(fallback) > 0 {
		fb = `,"cli_fallback":["` + strings.Join(fallback, `","`) + `"]`
	}
	body := `{"name":"` + agent + `","cli":"` + cli + `","model_tier_default":"sonnet"` + fb + `}`
	name := strings.TrimPrefix(agent, "evolve-") + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// dispatchThroughRunner drives the PRODUCTION advisor projection: a
// PhaseRequest whose ModelRoutingCLI is the advisor's overlay, through
// runner.New(...).Run → resolveDispatchPlan (routing.go) →
// llmroute.ApplySoftOverlay → the tiered chain walk → the bridge. Returns the
// CLIs the bridge saw, in dispatch order.
func dispatchThroughRunner(t *testing.T, root, overlayCLI string, trigger map[string]int) []string {
	t.Helper()
	// The runner's primary resolution reads EVOLVE_CLI from the PROCESS env
	// as a fallback tier (envchain.Resolve); an operator shell that exports it
	// would override the profile and turn this predicate into a test of the
	// shell. Pin it empty for the duration of the test.
	t.Setenv("EVOLVE_CLI", "")
	hooks := advisorHooks{phase: "scout", agent: "evolve-scout"}
	bridge := &recordingBridge{trigger: trigger}
	loader := prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-scout.md": &fstest.MapFile{Data: []byte("---\nname: evolve-scout\n---\nx")},
	})
	r := runner.New(runner.Options{Hooks: hooks, Bridge: bridge, Prompts: loader})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot:     root,
		Workspace:       t.TempDir(),
		ModelRoutingCLI: overlayCLI,
	}); err != nil {
		t.Fatalf("runner.Run(overlay=%q): %v", overlayCLI, err)
	}
	return bridge.calls
}

// TestC1647_001_PackageDocDecidesOverlayFamilyVsDriverSemantics — AC1: "Decide
// and DOCUMENT whether an overlay CLI is a family selector, a driver selector,
// or both-with-a-precedence — one sentence in llmroute's package doc".
//
// Runs the real `go doc` over the package and inspects ONLY the package
// comment (the output is cut at the first exported-symbol synopsis line, so
// the ApplySoftOverlay FUNCTION comment — which already carries the decision
// — cannot satisfy it). The decision vocabulary is the AC's own: the sentence
// must name both selector kinds and the overlay they apply to.
//
// acs-predicate: config-check — this criterion IS a documentation-presence
// requirement; the load-bearing artifact is the package doc `go doc` emits.
func TestC1647_001_PackageDocDecidesOverlayFamilyVsDriverSemantics(t *testing.T) {
	out, code := runGo(t, goDir(t), "doc", "./internal/llmroute")
	if code != 0 {
		t.Fatalf("go doc ./internal/llmroute exit=%d:\n%s", code, out)
	}
	pkgDoc := packageCommentOnly(out)
	lower := strings.ToLower(pkgDoc)
	for _, want := range []string{"overlay", "family selector", "driver selector"} {
		if !strings.Contains(lower, want) {
			t.Errorf("RED: llmroute PACKAGE doc does not contain %q — the family-vs-driver decision is not documented at the package boundary (only on the ApplySoftOverlay function comment).\n--- go doc package comment ---\n%s", want, pkgDoc)
		}
	}
}

// packageCommentOnly returns the `go doc` output up to the first symbol
// synopsis line (func/type/var/const), i.e. the package comment alone.
func packageCommentOnly(goDocOut string) string {
	var kept []string
	for _, line := range strings.Split(goDocOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "var ") || strings.HasPrefix(trimmed, "const ") {
			break
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// TestC1647_002_RunnerAdvisorBareFamilyOverlayKeepsHeadlessTransport — AC2:
// "chain [claude-p codex], overlay 'claude'" must not cross transport.
//
// The resolver half is the AC's literally-named llmroute test; the production
// half drives BaseRunner.Run with profile.cli=claude-p, cli_fallback=[codex]
// and the advisor's ModelRoutingCLI="claude" (a bare family name, exactly how
// the router projection most often arrives). The dispatched CLI must be the
// chain's own claude-p — never claude-tmux, the defaultDriverForFamily rewrite
// that moved a headless phase onto tmux (exit=10 on a host without it).
func TestC1647_002_RunnerAdvisorBareFamilyOverlayKeepsHeadlessTransport(t *testing.T) {
	t.Run("resolver-named-test", func(t *testing.T) {
		name := "TestApplySoftOverlay_BareFamilyDoesNotCrossTransportWhenTheChainHoldsANonDefaultDriver"
		out, code := runGo(t, goDir(t), "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/llmroute")
		if code != 0 || !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass in ./internal/llmroute (exit=%d):\n%s", name, code, out)
		}
	})
	t.Run("runner-production-path", func(t *testing.T) {
		root := writeProfile(t, "evolve-scout", "claude-p", []string{"codex"})
		calls := dispatchThroughRunner(t, root, "claude", nil)
		if len(calls) == 0 {
			t.Fatal("RED: the runner dispatched nothing")
		}
		if calls[0] != "claude-p" {
			t.Errorf("RED: runner dispatched %q first for bare overlay \"claude\" over chain [claude-p codex]; want claude-p — the advisor projection (resolveDispatchPlan, routing.go) must preserve the phase's resolved headless transport (dispatch order %v)", calls[0], calls)
		}
		for _, c := range calls {
			if c == "claude-tmux" {
				t.Errorf("RED: claude-tmux was dispatched (%v) — the bare-family overlay was normalized onto the family's default TMUX driver, the exact transport crossing this task forbids", calls)
			}
		}
	})
}

// TestC1647_003_RunnerAdvisorExplicitDriverOverlayWinsAndKeepsHeadlessFallback
// — AC3: "an explicit driver-qualified overlay must still win over a
// same-family chain entry (overlay 'claude-tmux' vs chain [claude-p]) — the
// case that forbids naive family matching".
//
// Through the same production path: profile.cli=claude-p (no fallback), the
// advisor asks for claude-tmux by name. The FIRST dispatch must be claude-tmux
// (a naive family match would promote claude-p and satisfy an explicit
// transport request with its opposite). The scripted claude-tmux exit 80 then
// proves the SOFT contract: the chain's own claude-p is retained as fallback,
// not replaced.
func TestC1647_003_RunnerAdvisorExplicitDriverOverlayWinsAndKeepsHeadlessFallback(t *testing.T) {
	t.Run("resolver-named-test", func(t *testing.T) {
		name := "TestApplySoftOverlay_DriverQualifiedOverlayWinsOverSameFamilyChainEntry"
		out, code := runGo(t, goDir(t), "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/llmroute")
		if code != 0 || !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass in ./internal/llmroute (exit=%d):\n%s", name, code, out)
		}
	})
	t.Run("runner-production-path", func(t *testing.T) {
		root := writeProfile(t, "evolve-scout", "claude-p", nil)
		calls := dispatchThroughRunner(t, root, "claude-tmux", map[string]int{"claude-tmux": 80})
		if len(calls) == 0 {
			t.Fatal("RED: the runner dispatched nothing")
		}
		if calls[0] != "claude-tmux" {
			t.Errorf("RED: runner dispatched %q first for explicit overlay \"claude-tmux\" over chain [claude-p]; want claude-tmux — an explicit driver request was satisfied by naive family matching (dispatch order %v)", calls[0], calls)
		}
		if len(calls) < 2 || calls[1] != "claude-p" {
			t.Errorf("RED: after claude-tmux exited 80 the walk did not reach claude-p (dispatch order %v) — the explicit overlay must be SOFT: the chain's headless entry is retained as fallback, not replaced", calls)
		}
	})
}

// TestC1647_004_FocusedRunnerRegressionCoversAdvisorProjectionAndNamesCaller
// — AC4: "The advisor/router projection path is covered, not just the
// contract-escalation path — name the production caller for each".
//
// Load-bearing: ONE narrowed `go test` over ./internal/phases/runner must
// report `--- PASS` for BOTH the scout's focused advisor-path test (absent on
// this tree → "no tests to run", exit 0 — which is why the PASS line, not the
// exit code, is asserted) and the pre-existing contract-escalation test.
// Auxiliary: the file that defines the focused test names the production
// caller (resolveDispatchPlan in routing.go), distinguishes it from the
// contract-escalation projection, and drives the AC's literal inputs.
func TestC1647_004_FocusedRunnerRegressionCoversAdvisorProjectionAndNamesCaller(t *testing.T) {
	dir := goDir(t)
	out, code := runGo(t, dir, "test", "-count=1", "-v",
		"-run", "^("+focusedRunnerTest+"|"+contractEscalationTest+")$", "./internal/phases/runner")
	for _, name := range []string{focusedRunnerTest, contractEscalationTest} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not PASS in ./internal/phases/runner (exit=%d)", name, code)
		}
	}
	if code != 0 {
		t.Errorf("RED: go test ./internal/phases/runner (narrowed) exit=%d:\n%s", code, out)
	}
	if t.Failed() {
		t.Logf("--- go test output ---\n%s", out)
	}

	file := fileDefiningTest(t, filepath.Join(dir, "internal", "phases", "runner"), focusedRunnerTest)
	if file == "" {
		t.Fatalf("RED: no *_test.go under internal/phases/runner defines func %s", focusedRunnerTest)
	}
	for _, want := range []string{"resolveDispatchPlan", "routing.go", "ModelRoutingCLI", `"claude-p"`, `"codex"`, `"claude-tmux"`} {
		if !acsassert.FileContains(t, file, want) {
			t.Errorf("RED: %s does not mention %q — the focused test must name the production caller and drive the AC's literal inputs", filepath.Base(file), want)
		}
	}
	if !acsassert.FileContainsAny(file, "ContractEscalation", "contract-escalation", "contract_escalation") {
		t.Errorf("RED: %s does not distinguish the advisor projection from the contract-escalation projection (both reach llmroute.ApplySoftOverlay through resolveDispatchPlan; AC4 asks the caller to be named for each)", filepath.Base(file))
	}
}

// fileDefiningTest returns the _test.go under pkgDir that declares
// `func <name>(`, or "" when none does.
func fileDefiningTest(t *testing.T, pkgDir, name string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", pkgDir, err)
	}
	decl := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if decl.Match(raw) {
			return f
		}
	}
	return ""
}

// TestC1647_005_LlmrouteRouterRaceCleanAndApicoverEnforceClean — AC5, the
// half a cycle predicate may run: `go test -race` over ./internal/llmroute and
// ./internal/router (ONE package per invocation) and the repo's own
// apicover -enforce recipe (Makefile apicover-enforce) scoped to
// internal/llmroute, which must report zero uncovered and zero false-green
// exports. ./internal/core is the banned whole-suite shape and is delegated
// to CI (see the package doc).
func TestC1647_005_LlmrouteRouterRaceCleanAndApicoverEnforceClean(t *testing.T) {
	dir := goDir(t)
	for _, pkg := range []string{"./internal/llmroute", "./internal/router"} {
		pkg := pkg
		t.Run("race-"+strings.TrimPrefix(pkg, "./internal/"), func(t *testing.T) {
			out, code := runGo(t, dir, "test", "-race", "-count=1", pkg)
			if code != 0 {
				t.Errorf("RED: go test -race %s exit=%d:\n%s", pkg, code, out)
			}
		})
	}
	t.Run("apicover-enforce-llmroute", func(t *testing.T) {
		tmp := t.TempDir()
		bin := filepath.Join(tmp, "apicover")
		if out, code := runGo(t, dir, "build", "-o", bin, "./cmd/apicover"); code != 0 {
			t.Fatalf("build apicover exit=%d:\n%s", code, out)
		}
		profile := filepath.Join(tmp, "coverage.txt")
		if out, code := runGo(t, dir, "test", "-count=1", "-tags", "integration", "-coverprofile="+profile, "./internal/llmroute"); code != 0 {
			t.Fatalf("coverprofile run exit=%d:\n%s", code, out)
		}
		funcOut, code := runGo(t, dir, "tool", "cover", "-func="+profile)
		if code != 0 {
			t.Fatalf("go tool cover exit=%d:\n%s", code, funcOut)
		}
		funcFile := filepath.Join(tmp, "coverage.func.txt")
		if err := os.WriteFile(funcFile, []byte(funcOut), 0o644); err != nil {
			t.Fatal(err)
		}
		pkgDir := filepath.Join(dir, "internal", "llmroute")
		cmd := exec.Command(bin, "-enforce", "-cover", funcFile, pkgDir)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("RED: apicover -enforce internal/llmroute failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "0 uncovered, 0 false-green") {
			t.Errorf("RED: apicover -enforce internal/llmroute is not clean:\n%s", out)
		}
	})
}

// TestC1647_006_PredicatePackageAndEvalAreGitTracked — cycle-93 lesson: the
// audit's predicate tree and the ship tree must agree. Disk presence alone
// passes for a gitignored file that is dropped at ship; pair it with an
// index check (`git ls-files --error-unmatch`), -C rooted at the worktree.
func TestC1647_006_PredicatePackageAndEvalAreGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"go/acs/cycle1647/predicates_test.go",
		".evolve/evals/" + taskSlug + ".md",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Errorf("RED: %s missing on disk", rel)
			continue
		}
		if _, stderr, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is not git-tracked (may be gitignored — dropped at ship): %s", rel, strings.TrimSpace(stderr))
		}
	}
}

// ---------------------------------------------------------------------------
// 007-008: continuation hygiene — cycle-1647 audit round 1 (H1, M1 corr. 3)
// ---------------------------------------------------------------------------

// thisPackage is the acs package these predicates live in; 007 must never
// re-run itself.
const thisPackage = "cycle1647"

// shippingExplanation resolves the ONE docs/explain/builds/cycle-*.md record
// this tree adds and returns its repo-relative path and body. On a
// continuation the host archives every unshipped predecessor under
// docs/private/research/archived-*/ (explanationdocs.
// ArchiveUnpublishedContinuationRecords), so the deliverable is the
// index/working-tree addition on a pre-commit lane, or the record HEAD's most
// recent record-adding commit introduced once the cycle has landed. Resolved
// from git, never from a cycle number or a report.
func shippingExplanation(t *testing.T) (string, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	const dir = "docs/explain/builds"
	isRecord := func(p string) bool {
		return strings.HasPrefix(p, dir+"/cycle-") && strings.HasSuffix(p, ".md")
	}
	var found []string
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all", "--", dir)
	if err != nil || code != 0 {
		t.Fatalf("git status -- %s: exit=%d err=%v stderr=%s", dir, code, err, stderr)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if i := strings.LastIndex(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		if isRecord(path) {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		out, stderr, code, err = acsassert.SubprocessOutput("git", "-C", root, "log", "-1", "--diff-filter=A", "--name-only", "--format=", "--", dir+"/cycle-*.md")
		if err != nil || code != 0 {
			t.Fatalf("git log -- %s: exit=%d err=%v stderr=%s", dir, code, err, stderr)
		}
		for _, line := range strings.Split(out, "\n") {
			if path := strings.TrimSpace(line); isRecord(path) {
				found = append(found, path)
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one shipping build explanation under %s/ (the record this tree adds; unshipped predecessors are archived under docs/private/); got %v", dir, found)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(found[0])))
	if err != nil {
		t.Fatalf("read %s: %v", found[0], err)
	}
	return found[0], string(raw)
}

// explanationBase reads the `- Base SHA:` line of the record's `## Build
// Binding`, so the diff these predicates check is the base-bound diff the
// host bound (explanationdocs.validateDocument pins it to the binding).
func explanationBase(t *testing.T, body string) string {
	t.Helper()
	in := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "## ")), "Build Binding")
			continue
		}
		if !in {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if rest, ok := strings.CutPrefix(line, "Base SHA:"); ok {
			if sha := strings.Trim(strings.TrimSpace(rest), "`"); sha != "" {
				return sha
			}
		}
	}
	t.Fatal("the build explanation's `## Build Binding` must carry a `- Base SHA:` line")
	return ""
}

// diffPaths is `git diff --name-only [--diff-filter=F] <base> -- <pathspec>`,
// -C rooted at the worktree: the base-bound diff read from git, not from any
// report that could agree with a wrong answer.
func diffPaths(t *testing.T, root, base, filter, pathspec string) []string {
	t.Helper()
	var tail []string
	if filter != "" {
		tail = append(tail, "--diff-filter="+filter)
	}
	tail = append(tail, base, "--", pathspec)
	out, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root, "diff", "--name-only"}, tail...)...)
	if err != nil || code != 0 {
		t.Fatalf("git diff --name-only %s -- %s: exit=%d err=%v stderr=%s", base, pathspec, code, err, stderr)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

// failLines keeps the lines of a nested `go test -v` run that carry the
// verdict, so a RED package reports its reasons without its whole log.
func failLines(out string) string {
	var keep []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "FAIL") || strings.Contains(line, "RED:") || strings.Contains(line, "expected exactly one") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// TestC1647_007 pins cycle-1647 audit H1: a continuation ships every inherited
// go/acs/cycle* package its base-bound diff ADDS, and the tracked eval
// (.evolve/evals/triage-unified-solution-synthesis.md) names their tests as
// evidence — so a RED inherited package is a RED shipping tree, whether or not
// the harness's own lane (`./acs/cycle1647` only, acssuite.goLanePatterns)
// happens to run it. The set is derived from the diff, never hard-coded; each
// package is run as ONE named `go test` (no `/...` sweep). RED on this tree:
// go/acs/cycle1638 TestC1638_010/011 fail against the shipping explanation
// (phase-registry.json unnamed; this diff's own package attributed to
// history — the audit's M1). The Builder's fix is the DOCUMENT, never the
// tests.
func TestC1647_007_InheritedPredicatePackagesAddedByThisDiffStayGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := goDir(t)
	_, body := shippingExplanation(t)
	base := explanationBase(t, body)
	seen := map[string]bool{}
	var pkgs []string
	for _, path := range diffPaths(t, root, base, "A", "go/acs/") {
		pkg := strings.Split(strings.TrimPrefix(path, "go/acs/"), "/")[0]
		if !strings.HasPrefix(pkg, "cycle") || pkg == thisPackage || seen[pkg] {
			continue
		}
		seen[pkg] = true
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		t.Fatalf("precondition: the base-bound diff against %s adds no inherited go/acs/cycle* package -- on a continuation tree that means the resolution is broken, not that the contract is vacuously met", base[:8])
	}
	for _, pkg := range pkgs {
		t.Run(pkg, func(t *testing.T) {
			out, code := runGo(t, dir, "test", "-tags", "acs", "-count=1", "-v", "./acs/"+pkg+"/")
			if code != 0 {
				t.Errorf("RED: inherited predicate package go/acs/%s is red on the shipping tree (exit=%d):\n%s", pkg, code, failLines(out))
			}
		})
	}
}

// TestC1647_008 pins cycle-1647 audit correction (3): a `path:line` citation
// is a promise the reader can open that file at that line on the shipped
// tree. The explanation cites `.evolve/inbox/…-overlay-family-name-transport-
// ambiguity.json:1` and `…-triage-unified-solution-synthesis.json:1` as its
// source records, yet this diff DELETES both root copies (they live on under
// .evolve/inbox/consumed/), so the citations resolve only through `git show
// <base>:…`. Deleted paths may still be NAMED (`## Changed Areas` explains
// the removal); they may not be cited at a line. The deleted set is derived
// from git (`--diff-filter=D`), so any future dead line-citation fails the
// same way; a diff that deletes nothing has nothing to assert and says so.
func TestC1647_008_ExplanationCitesNoLineIntoAPathThisDiffDeletes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath, body := shippingExplanation(t)
	deleted := diffPaths(t, root, explanationBase(t, body), "D", ".")
	if len(deleted) == 0 {
		t.Logf("the base-bound diff deletes no path; no line citation can be dead")
		return
	}
	for _, path := range deleted {
		re := regexp.MustCompile(regexp.QuoteMeta(path) + `:[0-9]+`)
		for _, cite := range re.FindAllString(body, -1) {
			t.Errorf("RED: %s cites `%s`, but this diff DELETES %s -- the line is unreadable on the shipped tree; cite the surviving copy (e.g. its .evolve/inbox/consumed/ record) at a real line instead", docPath, cite, path)
		}
	}
}
