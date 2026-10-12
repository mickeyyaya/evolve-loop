package core

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestRecordRoutingDecision_AnUnencodableDecisionIsLedgeredWithoutAnArtifact(t *testing.T) {
	ws := t.TempDir()
	ledger := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil))
	dec := router.RouterDecision{NextPhase: "build", Reason: "scored", Evidence: map[string]interface{}{"score": math.NaN()}}

	log := captureStderr(t, func() { o.recordRoutingDecision(context.Background(), 4, CycleState{WorkspacePath: ws}, 2, dec) })

	if !strings.Contains(log, "WARN routing-decision marshal: ") {
		t.Fatalf("stderr must name the marshal failure:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(ws, "routing-decision-2.json")); !os.IsNotExist(err) {
		t.Fatalf("no artifact may be written for an unencodable decision: %v", err)
	}
	var found bool
	for _, e := range ledger.entries {
		if e.Kind == "routing_decision" {
			found = true
			if e.ArtifactPath != "" || e.ArtifactSHA256 != "" {
				t.Fatalf("the decision row must not bind a missing artifact: %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("the decision must still be ledgered: %+v", ledger.entries)
	}
}

func TestRecordPhasePlanKind_AMarshalFailureLedgersThePlanWithoutAnArtifact(t *testing.T) {
	orig := phasePlanMarshal
	phasePlanMarshal = func(any, string, string) ([]byte, error) { return nil, errors.New("encoder refused") }
	t.Cleanup(func() { phasePlanMarshal = orig })
	ws := t.TempDir()
	ledger := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil))

	log := captureStderr(t, func() {
		o.recordPhasePlanKind(context.Background(), 6, CycleState{WorkspacePath: ws}, fullSpinePlan(), nil, "plan")
	})

	if !strings.Contains(log, "WARN phase-plan marshal: encoder refused") {
		t.Fatalf("stderr must name the marshal failure:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(ws, "phase-plan.json")); !os.IsNotExist(err) {
		t.Fatalf("no plan artifact may exist after a marshal failure: %v", err)
	}
	if len(ledger.entries) != 1 || ledger.entries[0].Kind != "phase_plan" || ledger.entries[0].ArtifactPath != "" {
		t.Fatalf("ledger = %+v, want one artifact-less phase_plan row", ledger.entries)
	}
}

func TestRecordPhasePlanKind_ALostAdvisorCaptureRowWarnsPerCapture(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "advisor-prompt-replan.txt"), "prompt")
	writeFile(t, filepath.Join(ws, "advisor-response-replan.txt"), "response")
	ledger := &kindFailingLedgerE4{failKinds: map[string]bool{"advisor_prompt": true, "advisor_response": true}}
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil))

	log := captureStderr(t, func() {
		o.recordPhasePlanKind(context.Background(), 6, CycleState{WorkspacePath: ws}, fullSpinePlan(), nil, "replan")
	})

	for _, kind := range []string{"advisor_prompt", "advisor_response"} {
		if !strings.Contains(log, "WARN "+kind+" ledger append: forced "+kind+" append fail") {
			t.Errorf("stderr lacks the %s warning:\n%s", kind, log)
		}
	}
	if got := ledger.kinds(); len(got) != 1 || got[0] != "phase_replan" {
		t.Fatalf("ledger kinds = %v, want only the plan row", got)
	}
}

func TestUsageEvidenceLines_KeepsOnlyTheNewestBoundedTail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		lines int
		want  []string
	}{
		{"under the bound", 2, []string{"s1", "s2"}},
		{"exactly the bound", advisorUsageEvidenceLines, []string{"s1", "s2", "s3", "s4", "s5"}},
		{"one past the bound drops the oldest", advisorUsageEvidenceLines + 1, []string{"s2", "s3", "s4", "s5", "s6"}},
		{"far past the bound", 40, []string{"s36", "s37", "s38", "s39", "s40"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			var b strings.Builder
			for i := 1; i <= tc.lines; i++ {
				b.WriteString(`{"Driver":"claude-tmux","Trigger":"exit-85","Summary":"s` + strconv.Itoa(i) + `"}` + "\n")
				b.WriteString("not json\n")
			}
			writeFile(t, filepath.Join(ws, UsageEvidenceFile), b.String())

			got := usageEvidenceLines(ws)

			if len(got) != len(tc.want) {
				t.Fatalf("lines = %v, want summaries %v", got, tc.want)
			}
			for i, s := range tc.want {
				if got[i] != "- claude-tmux (exit-85): "+s {
					t.Fatalf("line %d = %q, want summary %s", i, got[i], s)
				}
			}
		})
	}
}

func TestAssembleFailureDigest_EachPersistenceFailureReturnsTheBuiltDigest(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		orig := failureDigestMarshal
		failureDigestMarshal = func(any) ([]byte, error) { return nil, errors.New("encoder refused") }
		t.Cleanup(func() { failureDigestMarshal = orig })
		ws := t.TempDir()

		d, err := AssembleFailureDigest(12, ws, nil)

		if err == nil || err.Error() != "marshal failure digest: encoder refused" || d.Cycle != 12 || d.Fingerprint == "" {
			t.Fatalf("digest=%+v err=%v", d, err)
		}
		if _, statErr := os.Stat(filepath.Join(ws, "failure-digest.json")); !os.IsNotExist(statErr) {
			t.Fatalf("an unencoded digest was written: %v", statErr)
		}
	})
	t.Run("write", func(t *testing.T) {
		ws := filepath.Join(t.TempDir(), "workspace-is-a-file")
		writeFile(t, ws, "x")

		d, err := AssembleFailureDigest(13, ws, nil)

		if err == nil || !strings.HasPrefix(err.Error(), "write failure-digest.json: ") || d.Cycle != 13 {
			t.Fatalf("digest=%+v err=%v", d, err)
		}
	})
}

func TestPorcelainDirty_AnUnlaunchableGitIsAnErrorAndTheSetReadsClean(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir")

	set, err := porcelainDirty(context.Background(), missing)
	if err == nil || !strings.HasPrefix(err.Error(), "git status in "+missing+": ") || strings.Contains(err.Error(), "exit ") || set != nil {
		t.Fatalf("porcelainDirty = (%v, %v), want a launch error naming the dir", set, err)
	}

	var got map[string]bool
	log := captureStderr(t, func() { got = porcelainDirtySet(context.Background(), missing) })
	if got == nil || len(got) != 0 {
		t.Fatalf("porcelainDirtySet = %v, want an empty non-nil set", got)
	}
	if !strings.Contains(log, "WARN dirty-set: git status in "+missing) || !strings.Contains(log, "read as a clean tree") {
		t.Fatalf("stderr must warn that the tree is read as clean:\n%s", log)
	}
}

func identityOrchestrator(h *carryHarness, gateCalls *int, rows ...LedgerEntry) *Orchestrator {
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: rows}, buildRunners(nil),
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			return CompositionAuditSnapshot{}, nil
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			*gateCalls++
			return h.gates
		}),
		WithCompositionVerdictWriter(func(_ string, in CompositionVerdictInput) error {
			h.written = append(h.written, in)
			return nil
		}),
	)
}

func TestIdentityCarryForward_DeclinesWhenTheWorktreeCannotBeRead(t *testing.T) {
	cases := []struct {
		name      string
		arrange   func(t *testing.T, h *carryHarness, cs *CycleState)
		wantLog   string
		wantGates int
	}{
		{"HEAD is unreadable", func(t *testing.T, _ *carryHarness, cs *CycleState) {
			cs.ActiveWorktree = t.TempDir()
		}, "declined: HEAD: ", 0},
		{"the index cannot be written as a tree", func(t *testing.T, h *carryHarness, _ *CycleState) {
			blob := h.fx.git("hash-object", "-w", "base.txt")
			cmd := exec.Command("git", "-C", h.fx.worktree, "update-index", "--index-info")
			cmd.Stdin = strings.NewReader("100644 " + blob + " 1\tunmerged.txt\n")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("stage an unmerged entry: %v\n%s", err, out)
			}
		}, "declined: write-tree: ", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := pendedIdenticalLane(t)
			cs := h.fx.cycleState()
			tc.arrange(t, h, &cs)
			gateCalls := 0
			o := identityOrchestrator(h, &gateCalls, h.auditRow())

			var carried bool
			log := captureStderr(t, func() { carried = o.identityCarryForward(context.Background(), 31, cs, h.fx.base, h.fx.root) })

			if carried || gateCalls != tc.wantGates || len(h.written) != 0 {
				t.Fatalf("carried=%v gates=%d written=%d, want a decline before any gate", carried, gateCalls, len(h.written))
			}
			if !strings.Contains(log, "cycle 31 identity carry "+tc.wantLog) {
				t.Fatalf("stderr lacks %q:\n%s", tc.wantLog, log)
			}
		})
	}
}

func TestIdentityCarryForward_AnUnidentifiablePatchIsReauditedAfterTheGates(t *testing.T) {
	var hashed []byte
	orig := identityPatchID
	identityPatchID = func(diff []byte) (string, error) {
		hashed = diff
		return "", errors.New("patch-id produced nothing")
	}
	t.Cleanup(func() { identityPatchID = orig })
	h := pendedIdenticalLane(t)
	cs := h.fx.cycleState()
	gateCalls := 0
	o := identityOrchestrator(h, &gateCalls, h.auditRow())

	var carried bool
	log := captureStderr(t, func() { carried = o.identityCarryForward(context.Background(), 32, cs, h.fx.base, h.fx.root) })

	if carried || gateCalls != 1 || len(h.written) != 0 {
		t.Fatalf("carried=%v gates=%d written=%d, want gates run then a decline with no record", carried, gateCalls, len(h.written))
	}
	if len(hashed) == 0 || !strings.Contains(string(hashed), "lane.txt") {
		t.Fatalf("the audited change must be what is hashed: %q", hashed)
	}
	if !strings.Contains(log, "declined: patch-id: patch-id produced nothing") {
		t.Fatalf("stderr must name the patch-id failure:\n%s", log)
	}
}

func TestGatesOnIntactTree_AnUntakeableFenceReportsNoGates(t *testing.T) {
	t.Parallel()
	calls := 0
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			calls++
			return allComposedGatesPass()
		}))

	gates, outcomes, ok := o.gatesOnIntactTree(context.Background(), t.TempDir(), "any-tree")

	if ok || gates != nil || outcomes != nil {
		t.Fatalf("gatesOnIntactTree = (%v, %v, %v), want nothing reported without a fence", gates, outcomes, ok)
	}
	if calls != 1 {
		t.Fatalf("gate runs = %d, want the gates to have run once", calls)
	}
}

func TestRecordProvisioningFailure_AnUncreatableWorkspaceWarnsAndWritesNoDigest(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	writeFile(t, parent, "x")
	ws := filepath.Join(parent, "cycle-3")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	log := captureStderr(t, func() { o.recordProvisioningFailure(3, t.TempDir(), ws, errors.New("disk full")) })

	if !strings.Contains(log, "WARN worktree provisioning failed (source phases will be blocked): disk full") ||
		!strings.Contains(log, "WARN create workspace for provisioning failure: ") {
		t.Fatalf("stderr must carry both warnings:\n%s", log)
	}
	if strings.Contains(log, "assemble failure digest") {
		t.Fatalf("no digest may be attempted without a workspace:\n%s", log)
	}
}

func TestNormalizeScoutGoalHash_AnUnreadableReportWarnsAndLeavesItAlone(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, LaneScopeFile), `{"todo_ids":["a"],"goal_hash":"`+strings.Repeat("ab", 32)+`"}`)
	if err := os.Mkdir(filepath.Join(ws, scoutArtifactName), 0o755); err != nil {
		t.Fatal(err)
	}

	log := captureStderr(t, func() { normalizeScoutGoalHash(ws) })

	if !strings.Contains(log, "WARN "+scoutArtifactName+" unreadable during goal_hash normalize") {
		t.Fatalf("stderr must warn about the unreadable report:\n%s", log)
	}
	if info, err := os.Stat(filepath.Join(ws, scoutArtifactName)); err != nil || !info.IsDir() {
		t.Fatalf("the report path must be left untouched: %v", err)
	}
}

func TestNormalizeScoutGoalHash_AnAbsentReportIsSilent(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, LaneScopeFile), `{"todo_ids":["a"],"goal_hash":"`+strings.Repeat("ab", 32)+`"}`)

	if log := captureStderr(t, func() { normalizeScoutGoalHash(ws) }); log != "" {
		t.Fatalf("an absent report is not a warning: %q", log)
	}
}

func TestMaterializedEvalSlugs_NoWorkspaceOrAnUnreadableHomeOwnsNothing(t *testing.T) {
	if got := materializedEvalSlugs(""); got != nil {
		t.Fatalf("no workspace = %v, want nil", got)
	}
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, filepath.FromSlash(strings.TrimSuffix(evalsDir, "/"))), "a file, not the eval home")

	var got []string
	log := captureStderr(t, func() { got = materializedEvalSlugs(ws) })

	if got != nil {
		t.Fatalf("an unreadable eval home = %v, want nil", got)
	}
	if !strings.Contains(log, "WARN lane ownership: workspace eval home unreadable") {
		t.Fatalf("stderr must warn about the unreadable home:\n%s", log)
	}
	if log := captureStderr(t, func() { materializedEvalSlugs(t.TempDir()) }); log != "" {
		t.Fatalf("an absent eval home is not a warning: %q", log)
	}
}

func TestActiveVerifiedMints_ACorruptRegistryThatCannotBeQuarantinedWarnsTwice(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	root := t.TempDir()
	registry := mintregistry.Path(root)
	writeFile(t, registry, "{corrupt")
	dir := filepath.Dir(registry)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	var got map[string]bool
	log := captureStderr(t, func() { got = activeVerifiedMints(root) })

	if got != nil {
		t.Fatalf("a corrupt registry exempts nothing, got %v", got)
	}
	if !strings.Contains(log, "ABNORMAL mint registry unreadable") || !strings.Contains(log, "WARN mint registry quarantine failed: ") {
		t.Fatalf("stderr must carry the corruption and the failed quarantine:\n%s", log)
	}
	if body, err := os.ReadFile(registry); err != nil || string(body) != "{corrupt" {
		t.Fatalf("the unquarantined registry must stay in place: %q %v", body, err)
	}
}

func TestGatesOnIntactTree_AFenceThatCannotSnapshotAnUnreadableFileReportsNoGates(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	r := committedRepoE4(t, map[string]string{"a.txt": "a\n"})
	tree := r.Git("write-tree")
	calls := 0
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			calls++
			return allComposedGatesPass()
		}))
	gates, _, ok := o.gatesOnIntactTree(context.Background(), r.Dir, tree)
	if !ok || gates["test"] != "pass" {
		t.Fatalf("control: a readable intact tree = (%v, %v), want the gates", gates, ok)
	}
	secret := filepath.Join(r.Dir, "unreadable.txt")
	writeFile(t, secret, "x")
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o600) })

	gates, outcomes, ok := o.gatesOnIntactTree(context.Background(), r.Dir, tree)

	if ok || gates != nil || outcomes != nil {
		t.Fatalf("gatesOnIntactTree = (%v, %v, %v), want nothing reported when the fence could not be taken", gates, outcomes, ok)
	}
	if calls != 2 {
		t.Fatalf("gate runs = %d, want 2", calls)
	}
}
