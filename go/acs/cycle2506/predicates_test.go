//go:build acs

package cycle2506

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg       = "internal/core"
	baselineShard = "/testdata/testaudit-baseline.json"
)

var tautologyRules = []string{"no-assertion", "log-only", "asserts-own-setup", "both-branches-pass"}

var flaggedCoreTests = []string{
	"TestAppendPhaseIOShadowMismatch_LedgerFailureIsNonFatal",
	"TestBridgePort_SatisfiedByFakeAndLaunches",
	"TestBuildSelfCheck_EmptyWorktreeIsNoOp",
	"TestCheckpointInterruptedPhase_CheckpointFailureWarns",
	"TestEmitPhaseOutputsSignal_UnreadableWorkspace",
	"TestFailureLearning_LiteralOrchestratorGetsTheEngineOnce",
	"TestGuardPort_DecideBindsInputAndDecision",
	"TestMaybeRemediate_FixDispatchBaselineExcludesASiblingWriteDuringTheGate",
	"TestNormalizeBuildGofmt_EmptyWorktreeIsNoOp",
	"TestNormalizeDerivedProjections_EmptyWorktree_NoOp",
	"TestRecordDebuggerDecision_LedgerFailureIsNonFatal",
	"TestRecordPhaseOutcome_NonAbortGradeIsAnnounced",
	"TestRecordPhasePlan_LedgerFailIsSwallowed",
	"TestRecordRoutingDecision_LedgerFailIsSwallowed",
	"TestReleaseDeclinedBinding_NilAndEmptyAreNoops",
	"TestRetireSupersededAuditArtifacts_MissingFilesAndWorkspaceAreQuiet",
	"TestStartRunLease_EmptyWorkspace_NoOp",
	"TestWithFailurePolicy_InjectsResolvedPolicy",
}

const guardPortTest = "TestGuardPort_DecideBindsInputAndDecision"

var buildTagsOf = map[string]string{
	"TestMaybeRemediate_FixDispatchBaselineExcludesASiblingWriteDuringTheGate": "integration",
}

type sourceMutant struct {
	label        string
	file         string
	old          string
	new          string
	compileKills bool
}

func coreFindings(t *testing.T, goDir string) []string {
	t.Helper()
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit.DefaultConfig: %v", err)
	}
	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(goDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prevDir); err != nil {
			t.Errorf("restore the working directory: %v", err)
		}
	})
	found, err := testaudit.Analyze(corePkg, cfg)
	if err != nil {
		t.Fatalf("testaudit.Analyze: %v", err)
	}
	var rows []string
	for _, f := range found {
		if slices.Contains(tautologyRules, f.Rule) {
			rows = append(rows, fmt.Sprintf("%s %s %s", f.Package, f.Test, f.Rule))
		}
	}
	return rows
}

func coreShardRows(t *testing.T, goDir string) []string {
	t.Helper()
	shards, err := acsassert.WalkModule(goDir, func(rel string) bool {
		return strings.HasPrefix(rel, corePkg+"/") && strings.HasSuffix("/"+rel, baselineShard)
	})
	if err != nil {
		t.Fatalf("scan baseline shards: %v", err)
	}
	var rows []string
	for _, rel := range shards {
		raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		var shard struct {
			Rows []testaudit.Finding `json:"rows"`
		}
		if err := json.Unmarshal(raw, &shard); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, r := range shard.Rows {
			if slices.Contains(tautologyRules, r.Rule) {
				rows = append(rows, fmt.Sprintf("%s: %s %s %s", rel, r.Package, r.Test, r.Rule))
			}
		}
	}
	return rows
}

func TestC2506_001_NoTautologicalFindingLeftInCore(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	if left := coreFindings(t, goDir); len(left) > 0 {
		t.Errorf("RED: %d tautological finding(s) remain in internal/core:\n%s", len(left), strings.Join(left, "\n"))
	}
	if rows := coreShardRows(t, goDir); len(rows) > 0 {
		t.Errorf("RED: %d tautology row(s) sit in an internal/core baseline shard:\n%s", len(rows), strings.Join(rows, "\n"))
	}
}

func testDeclaredInCore(t *testing.T, goDir, name string) bool {
	t.Helper()
	files, err := acsassert.WalkModule(goDir, func(rel string) bool {
		return filepath.Dir(rel) == corePkg && strings.HasSuffix(rel, "_test.go")
	})
	if err != nil {
		t.Fatalf("list the core test files: %v", err)
	}
	declaration := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(`)
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if declaration.Match(raw) {
			return true
		}
	}
	return false
}

func TestC2506_002_FlaggedCoreTestsAreRewrittenNotDeleted(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	for _, name := range flaggedCoreTests {
		if name == guardPortTest {
			continue
		}
		if !testDeclaredInCore(t, goDir, name) {
			t.Errorf("RED: %s was deleted or renamed instead of rewritten; a deleted test leaves the baseline without ever failing a mutant", name)
		}
	}
}

func compileCoreTests(t *testing.T, goDir, overlay, tags string) (string, string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "core.test")
	args := []string{"test", "-c", "-o", bin}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	if overlay != "" {
		args = append(args, "-overlay="+overlay)
	}
	args = append(args, "./"+corePkg)
	cmd := exec.Command("go", args...)
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(out)
	}
	return bin, ""
}

func dirEntries(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}

func runCoreTest(t *testing.T, bin, dir, name string) (string, int) {
	t.Helper()
	before := dirEntries(t, dir)
	t.Cleanup(func() {
		for entry := range dirEntries(t, dir) {
			if !before[entry] {
				if err := os.RemoveAll(filepath.Join(dir, entry)); err != nil {
					t.Errorf("remove the %s that a mutant left in the package directory: %v", entry, err)
				}
			}
		}
	})
	cmd := exec.Command(bin, "-test.run", "^"+name+"$", "-test.count=1", "-test.v", "-test.timeout=120s")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run %s: %v", name, err)
	}
	return string(out), exitErr.ExitCode()
}

func overlayFor(t *testing.T, goDir string, m sourceMutant) string {
	t.Helper()
	original := filepath.Join(goDir, filepath.FromSlash(corePkg), m.file)
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), m.old); n != 1 {
		t.Fatalf("the mutation anchor for %q matches %d times in %s, want 1; the production code drifted and the mutant is stale", m.label, n, m.file)
	}
	mutated := filepath.Join(t.TempDir(), m.file)
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(raw), m.old, m.new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	doc, err := json.Marshal(map[string]map[string]string{"Replace": {original: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, doc, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func requireKillsSourceMutants(t *testing.T, name string, mutants ...sourceMutant) {
	t.Helper()
	goDir := acsassert.GoModuleDir(t)
	pkgDir := filepath.Join(goDir, filepath.FromSlash(corePkg))
	if name == guardPortTest && !testDeclaredInCore(t, goDir, name) {
		return
	}
	bin, failure := compileCoreTests(t, goDir, "", buildTagsOf[name])
	if bin == "" {
		t.Fatalf("the unmutated core tests do not compile:\n%s", acsassert.OutputTail(failure, 20))
	}
	out, code := runCoreTest(t, bin, pkgDir, name)
	if code != 0 || !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("%s does not pass on the unmutated core package (exit %d):\n%s", name, code, acsassert.OutputTail(out, 20))
	}
	for _, m := range mutants {
		t.Run(m.file+"/"+m.label, func(t *testing.T) {
			mutantBin, failure := compileCoreTests(t, goDir, overlayFor(t, goDir, m), buildTagsOf[name])
			if mutantBin == "" {
				if m.compileKills {
					return
				}
				t.Fatalf("the mutant %q does not compile, so it proves nothing:\n%s", m.label, acsassert.OutputTail(failure, 20))
			}
			out, code := runCoreTest(t, mutantBin, pkgDir, name)
			if code == 0 {
				t.Errorf("RED: %s still passes when %s", name, m.label)
				return
			}
			if !strings.Contains(out, "--- FAIL: "+name) {
				t.Errorf("%s exits %d on the mutant without a reported failure of its own:\n%s", name, code, acsassert.OutputTail(out, 20))
			}
		})
	}
}

func TestC2506_003_ShadowMismatchTestFailsWhenTheLedgerAppendIsSkippedOrMislabelled(t *testing.T) {
	requireKillsSourceMutants(t, "TestAppendPhaseIOShadowMismatch_LedgerFailureIsNonFatal",
		sourceMutant{
			label: "a non-empty mismatch list never reaches the ledger",
			file:  "phaseio_shadow.go",
			old:   "\tif len(ms) == 0 {\n\t\treturn\n\t}\n\tif err := l.Append(ctx, LedgerEntry{",
			new:   "\tif len(ms) >= 0 {\n\t\treturn\n\t}\n\tif err := l.Append(ctx, LedgerEntry{",
		},
		sourceMutant{
			label: "the ledger entry carries the wrong kind",
			file:  "phaseio_shadow.go",
			old:   "Kind:    \"phaseio_shadow_mismatch\",",
			new:   "Kind:    \"phaseio_shadow_mismatch_x\",",
		})
}

func TestC2506_004_BridgePortTestFailsWhenTheFreshSessionLaunchDropsTheRequest(t *testing.T) {
	requireKillsSourceMutants(t, "TestBridgePort_SatisfiedByFakeAndLaunches",
		sourceMutant{
			label: "LaunchWithFreshSession launches an empty request",
			file:  "launch_recoverable.go",
			old:   "return LaunchRecoverable(ctx, b.Launch, req, SameRequest(req))",
			new:   "return LaunchRecoverable(ctx, b.Launch, BridgeRequest{}, SameRequest(req))",
		})
}

func TestC2506_005_GuardPortTestFailsWhenTheGuardPortGainsAMethod(t *testing.T) {
	requireKillsSourceMutants(t, guardPortTest,
		sourceMutant{
			label:        "the Guard port requires a method the fake lacks",
			file:         "ports.go",
			old:          "\tName() string\n\tDecide(ctx context.Context, in GuardInput) GuardDecision\n}",
			new:          "\tName() string\n\tClose() error\n\tDecide(ctx context.Context, in GuardInput) GuardDecision\n}",
			compileKills: true,
		})
}

func TestC2506_006_SelfCheckTestFailsWhenAnEmptyWorktreeIsNotSkipped(t *testing.T) {
	requireKillsSourceMutants(t, "TestBuildSelfCheck_EmptyWorktreeIsNoOp",
		sourceMutant{
			label: "buildSelfCheck runs against the working directory when the worktree is empty",
			file:  "phase_bindings_selfcheck.go",
			old:   "func (o *Orchestrator) buildSelfCheck(ctx context.Context, worktree string) {\n\tif worktree == \"\" {",
			new:   "func (o *Orchestrator) buildSelfCheck(ctx context.Context, worktree string) {\n\tif false && worktree == \"\" {",
		})
}

func TestC2506_007_CheckpointTestFailsWhenTheCheckpointIsNotTakenOrLosesItsRoot(t *testing.T) {
	requireKillsSourceMutants(t, "TestCheckpointInterruptedPhase_CheckpointFailureWarns",
		sourceMutant{
			label: "an interrupted phase never takes its checkpoint",
			file:  "cyclerun_epilogue.go",
			old:   "if cr.ctx.Err() == nil || cr.cycleCompletedNormally || ResumeBoundaryCheckpointer == nil {",
			new:   "if true || cr.ctx.Err() == nil || cr.cycleCompletedNormally || ResumeBoundaryCheckpointer == nil {",
		},
		sourceMutant{
			label: "the checkpoint is taken for an empty project root",
			file:  "cyclerun_epilogue.go",
			old:   "ResumeBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now())",
			new:   "ResumeBoundaryCheckpointer(cr.cs, \"\", cr.o.now())",
		})
}

func TestC2506_008_PhaseOutputsTestFailsWhenAnUnreadableWorkspaceStillEmitsTheSurvey(t *testing.T) {
	requireKillsSourceMutants(t, "TestEmitPhaseOutputsSignal_UnreadableWorkspace",
		sourceMutant{
			label: "an unreadable workspace no longer ends the survey",
			file:  "phaseoutputs_signal.go",
			old:   "phase-outputs survey skipped for cycle %d: %v\\n\", cycle, err)\n\t\treturn\n",
			new:   "phase-outputs survey skipped for cycle %d: %v\\n\", cycle, err)\n",
		},
		sourceMutant{
			label: "an empty workspace no longer ends the survey",
			file:  "phaseoutputs_signal.go",
			old:   "func emitPhaseOutputsSignal(workspace string, cycle int, completed []string, resolver phasecontract.Resolver) {\n\tif workspace == \"\" {",
			new:   "func emitPhaseOutputsSignal(workspace string, cycle int, completed []string, resolver phasecontract.Resolver) {\n\tif false && workspace == \"\" {",
		})
}

func TestC2506_009_FailureLearningTestFailsOnASnapshotClockOrARebuiltEngine(t *testing.T) {
	requireKillsSourceMutants(t, "TestFailureLearning_LiteralOrchestratorGetsTheEngineOnce",
		sourceMutant{
			label: "the engine clock is a snapshot of o.now",
			file:  "failure_learning_engine.go",
			old:   "failurelearning.New(func() time.Time { return o.now() }, o.carryover(),",
			new:   "failurelearning.New(func() func() time.Time { snapshot := o.now(); return func() time.Time { return snapshot } }(), o.carryover(),",
		},
		sourceMutant{
			label: "the engine is rebuilt on every call",
			file:  "failure_learning_engine.go",
			old:   "\tif o.learn == nil {\n\t\to.learn = o.wiredFailureLearning()",
			new:   "\tif true {\n\t\to.learn = o.wiredFailureLearning()",
		})
}

func TestC2506_010_RemediationTestFailsWhenTheFixBaselineIsTheGateBaseline(t *testing.T) {
	requireKillsSourceMutants(t, "TestMaybeRemediate_FixDispatchBaselineExcludesASiblingWriteDuringTheGate",
		sourceMutant{
			label: "the fix dispatch judges the tree against the gate baseline",
			file:  "cyclerun_remediate.go",
			old:   "treeGuard: fixGuard, beforeDirty: fixBefore, remediation: true,",
			new:   "treeGuard: fixGuard, beforeDirty: func() []string { _ = fixBefore; return dr.beforeDirty }(), remediation: true,",
		})
}

func TestC2506_011_GofmtTestFailsWhenAnEmptyWorktreeFormatsTheWorkingDirectory(t *testing.T) {
	requireKillsSourceMutants(t, "TestNormalizeBuildGofmt_EmptyWorktreeIsNoOp",
		sourceMutant{
			label: "normalizeBuildGofmt formats the working directory when the worktree is empty",
			file:  "phase_bindings.go",
			old:   "func normalizeBuildGofmt(worktree string) {\n\tif worktree == \"\" {",
			new:   "func normalizeBuildGofmt(worktree string) {\n\tif false && worktree == \"\" {",
		})
}

func TestC2506_012_DerivedProjectionsTestFailsWhenAnEmptyWorktreeIsNotSkipped(t *testing.T) {
	requireKillsSourceMutants(t, "TestNormalizeDerivedProjections_EmptyWorktree_NoOp",
		sourceMutant{
			label: "normalizeDerivedProjections regenerates in the working directory when the worktree is empty",
			file:  "phase_bindings.go",
			old:   "func (o *Orchestrator) normalizeDerivedProjections(ctx context.Context, worktree string) {\n\tif worktree == \"\" {",
			new:   "func (o *Orchestrator) normalizeDerivedProjections(ctx context.Context, worktree string) {\n\tif false && worktree == \"\" {",
		})
}

func TestC2506_013_DebuggerDecisionTestFailsOnAMislabelledLedgerEntry(t *testing.T) {
	requireKillsSourceMutants(t, "TestRecordDebuggerDecision_LedgerFailureIsNonFatal",
		sourceMutant{
			label: "the debugger ledger entry carries the wrong kind",
			file:  "decision_branch.go",
			old:   "Kind: \"debugger_decision\", ExitCode: 0,",
			new:   "Kind: \"debugger_decision_x\", ExitCode: 0,",
		})
}

func TestC2506_014_PhaseOutcomeTestFailsWhenTheGradeAnnouncementOrTheRecordIsLost(t *testing.T) {
	requireKillsSourceMutants(t, "TestRecordPhaseOutcome_NonAbortGradeIsAnnounced",
		sourceMutant{
			label: "the non-abort grade is no longer announced",
			file:  "failure_learning.go",
			old:   "tier != failuregrade.TierAbort {",
			new:   "tier == failuregrade.TierAbort {",
		},
		sourceMutant{
			label: "the outcome is never recorded",
			file:  "failure_learning.go",
			old:   "\to.recorder().Record(result, timings, workspace, out)\n",
			new:   "\t_ = o.recorder()\n",
		})
}

func TestC2506_015_PhasePlanTestFailsWhenTheLedgerFailureIsNotReportedOrTheKindIsWrong(t *testing.T) {
	requireKillsSourceMutants(t, "TestRecordPhasePlan_LedgerFailIsSwallowed",
		sourceMutant{
			label: "the ledger append failure is no longer reported",
			file:  "decision_branch.go",
			old:   "fmt.Fprintf(o.errOut(), \"[orchestrator] WARN phase_%s ledger append: %v\\n\", kind, err)",
			new:   "_ = err",
		},
		sourceMutant{
			label: "the phase plan ledger entry carries the wrong kind",
			file:  "decision_branch.go",
			old:   "Kind: \"phase_\" + kind,",
			new:   "Kind: \"phase_x\" + kind,",
		})
}

func TestC2506_016_RoutingDecisionTestFailsWhenTheLedgerFailureIsNotReportedOrSkipsAreDropped(t *testing.T) {
	requireKillsSourceMutants(t, "TestRecordRoutingDecision_LedgerFailIsSwallowed",
		sourceMutant{
			label: "the routing decision ledger failure is no longer reported",
			file:  "decision_branch.go",
			old:   "fmt.Fprintf(o.errOut(), \"[orchestrator] WARN routing_decision ledger append: %v\\n\", err)",
			new:   "_ = err",
		},
		sourceMutant{
			label: "the skipped phases never reach the ledger",
			file:  "decision_branch.go",
			old:   "for _, sp := range dec.SkipPhases {",
			new:   "for _, sp := range dec.SkipPhases[:0] {",
		})
}

func TestC2506_017_ReleaseTestFailsWhenTheWrongCycleIsReleased(t *testing.T) {
	requireKillsSourceMutants(t, "TestReleaseDeclinedBinding_NilAndEmptyAreNoops",
		sourceMutant{
			label: "the release targets the cycle after the declined one",
			file:  "continuation_stamp.go",
			old:   "continuation.DeleteRegistryEntryIfCycle(projectRoot, id, declined.Cycle)",
			new:   "continuation.DeleteRegistryEntryIfCycle(projectRoot, id, declined.Cycle+1)",
		},
		sourceMutant{
			label: "a nil declined continuation is dereferenced",
			file:  "continuation_stamp.go",
			old:   "\tif declined == nil {\n\t\treturn\n\t}\n\tfor _, id := range scopeIDs {",
			new:   "\tif false && declined == nil {\n\t\treturn\n\t}\n\tfor _, id := range scopeIDs {",
		})
}

func TestC2506_018_RetireTestFailsWhenGuardsOrTheArchiveMoveAreLost(t *testing.T) {
	requireKillsSourceMutants(t, "TestRetireSupersededAuditArtifacts_MissingFilesAndWorkspaceAreQuiet",
		sourceMutant{
			label: "an empty workspace or a round below one is no longer skipped",
			file:  "audit_round_artifacts.go",
			old:   "if workspace == \"\" || round < 1 {",
			new:   "if false {",
		},
		sourceMutant{
			label: "a superseded artifact is never archived",
			file:  "audit_round_artifacts.go",
			old:   "err := os.Rename(src, dst)",
			new:   "err := error(nil)",
		})
}

func TestC2506_019_RunLeaseTestFailsWhenAnEmptyWorkspaceStillWritesALease(t *testing.T) {
	requireKillsSourceMutants(t, "TestStartRunLease_EmptyWorkspace_NoOp",
		sourceMutant{
			label: "startRunLease writes a lease into the working directory when the workspace is empty",
			file:  "runlease_hook.go",
			old:   "\tif workspace == \"\" {\n\t\treturn func() {}\n\t}",
			new:   "\tif false && workspace == \"\" {\n\t\treturn func() {}\n\t}",
		})
}

func TestC2506_020_FailurePolicyTestFailsWhenTheOptionIgnoresThePolicy(t *testing.T) {
	requireKillsSourceMutants(t, "TestWithFailurePolicy_InjectsResolvedPolicy",
		sourceMutant{
			label: "WithFailurePolicy drops the policy",
			file:  "orchestrator.go",
			old:   "return func(o *Orchestrator) { o.failurePolicy = fp }",
			new:   "return func(o *Orchestrator) { _ = fp }",
		})
}

func TestC2506_021_RewritesKeepEveryCoreExportNamed(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	dir := filepath.Join(goDir, filepath.FromSlash(corePkg))
	syms, err := apicover.Enumerate(context.Background(), dir)
	if err != nil {
		t.Fatalf("enumerate %s: %v", corePkg, err)
	}
	named, err := apicover.NamesReferencedInTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("scan the tests of %s: %v", corePkg, err)
	}
	for _, s := range apicover.Classify(syms, named, nil).Uncovered {
		t.Errorf("RED: %s (%s) is named by no test, so the apicover gate of ./internal/core fails", s.Name, s.File)
	}
}
