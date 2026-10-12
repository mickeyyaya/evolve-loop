package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

type compositionProbeE4 struct {
	snapshotCalls int
	gateCalls     int
	writerCalls   int
	reviewCalls   int
	written       CompositionVerdictInput
}

func greenComposedGatesE4() map[string]ciparity.GateOutcome {
	out := map[string]ciparity.GateOutcome{}
	for _, g := range ciparity.RequiredComposedGates {
		out[g] = ciparity.GateOutcome{Status: "pass"}
	}
	return out
}

func (p *compositionProbeE4) orchestrator(snap CompositionAuditSnapshot, snapErr, writerErr error, review func([]MergeHunk, string, string) ScopedMergeReviewOutcome) *Orchestrator {
	opts := []Option{
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			p.snapshotCalls++
			return snap, snapErr
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			p.gateCalls++
			return greenComposedGatesE4()
		}),
		WithCompositionVerdictWriter(func(_ string, in CompositionVerdictInput) error {
			p.writerCalls++
			p.written = in
			return writerErr
		}),
	}
	if review != nil {
		opts = append(opts, WithScopedMergeReviewer(func(h []MergeHunk, a, c string) ScopedMergeReviewOutcome {
			p.reviewCalls++
			return review(h, a, c)
		}))
	}
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
}

func TestCompositionCarryForward_DeclinesBeforeAnyGateOnEachUnverifiableComposition(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	empty := committedRepoE4(t, map[string]string{"file.txt": "base\n"}).Dir
	cases := []struct {
		name          string
		worktree      string
		snap          CompositionAuditSnapshot
		wantSnapshots int
		wantLog       string
	}{
		{"no active worktree", "", CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, 0, ""},
		{"composed diff fails outside a repository", t.TempDir(), CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, 1, "composed diff unavailable"},
		{"empty composed diff has no patch-id", empty, CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, 1, "patch-id computation failed"},
		{"semantic drift between audited and composed patch-id", worktree, CompositionAuditSnapshot{Diff: diff, PatchID: "0000000000000000000000000000000000000000"}, 1, "semantic drift"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &compositionProbeE4{}
			o := p.orchestrator(tc.snap, nil, nil, nil)
			var carried bool
			log := captureStderr(t, func() {
				carried = o.compositionCarryForward(context.Background(), 3, CycleState{ActiveWorktree: tc.worktree, RunID: "r"}, t.TempDir())
			})
			if carried {
				t.Fatal("an unverifiable composition carried forward; want a full re-audit")
			}
			if p.snapshotCalls != tc.wantSnapshots || p.gateCalls != 0 || p.writerCalls != 0 {
				t.Fatalf("calls snapshot=%d gate=%d writer=%d, want snapshot=%d and no gate or writer", p.snapshotCalls, p.gateCalls, p.writerCalls, tc.wantSnapshots)
			}
			if !strings.Contains(log, tc.wantLog) {
				t.Errorf("stderr %q lacks %q", log, tc.wantLog)
			}
		})
	}
}

func TestCompositionCarryForward_MatchingPatchIDWritesTheVerdict(t *testing.T) {
	t.Parallel()

	worktree, diff, patchID := divergedCompositionFixture(t)
	p := &compositionProbeE4{}
	o := p.orchestrator(CompositionAuditSnapshot{LaneAuditRef: "ref-9", AuditedBase: "main", Diff: diff, PatchID: patchID}, nil, nil, nil)
	if !o.compositionCarryForward(context.Background(), 9, CycleState{ActiveWorktree: worktree, RunID: "r"}, t.TempDir()) {
		t.Fatal("a patch-id-identical composition with green gates must carry forward")
	}
	if p.written.Cycle != 9 || p.written.PatchID != patchID || p.written.LaneAuditRef != "ref-9" || p.written.GateResults["acs"] != "pass" {
		t.Fatalf("written verdict = %+v", p.written)
	}
}

func TestScopedMergeCarryForward_DeclinesOnEachFailClosedRung(t *testing.T) {
	worktree, diff, patchID := divergedCompositionFixture(t)
	compatible := func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
		return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
	}
	cases := []struct {
		name        string
		worktree    string
		snap        CompositionAuditSnapshot
		snapErr     error
		writerErr   error
		review      func([]MergeHunk, string, string) ScopedMergeReviewOutcome
		wantReviews int
		wantGates   int
		wantWrites  int
		wantLog     string
	}{
		{"no active worktree", "", CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, nil, nil, compatible, 0, 0, 0, ""},
		{"snapshot unavailable", worktree, CompositionAuditSnapshot{}, errors.New("no bound audit"), nil, compatible, 0, 0, 0, "snapshot unavailable: no bound audit"},
		{"composed diff fails outside a repository", t.TempDir(), CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, nil, nil, compatible, 0, 0, 0, "composed diff unavailable"},
		{"corrupt audited hunk header", worktree, CompositionAuditSnapshot{Diff: []byte("+++ b/file.txt\n@@ corrupt @@\n"), PatchID: patchID}, nil, nil, compatible, 0, 0, 0, "failed closed"},
		{"reviewer says entangled", worktree, CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, nil, nil, func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeEntangled}
		}, 1, 0, 0, "entangled — escalating"},
		{"resolution without a patch-id", worktree, CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, nil, nil, func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible, ResolutionDiff: []byte("not a diff\n")}
		}, 1, 0, 0, "rung-0 re-entry failed"},
		{"resolution patch-id differs from the audited one", worktree, CompositionAuditSnapshot{Diff: diff, PatchID: "1111111111111111111111111111111111111111"}, nil, nil, compatible, 1, 0, 0, "does not match audited change"},
		{"verdict writer fails closed", worktree, CompositionAuditSnapshot{Diff: diff, PatchID: patchID}, nil, errors.New("ledger locked"), compatible, 1, 1, 1, "writer failed (fail-closed): ledger locked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &compositionProbeE4{}
			o := p.orchestrator(tc.snap, tc.snapErr, tc.writerErr, tc.review)
			var carried bool
			log := captureStderr(t, func() {
				carried = o.scopedMergeCarryForward(context.Background(), 4, CycleState{ActiveWorktree: tc.worktree, RunID: "r"}, t.TempDir())
			})
			if carried {
				t.Fatal("a fail-closed rung carried the composition forward")
			}
			if p.reviewCalls != tc.wantReviews || p.gateCalls != tc.wantGates || p.writerCalls != tc.wantWrites {
				t.Fatalf("calls review=%d gate=%d writer=%d, want %d/%d/%d", p.reviewCalls, p.gateCalls, p.writerCalls, tc.wantReviews, tc.wantGates, tc.wantWrites)
			}
			if !strings.Contains(log, tc.wantLog) {
				t.Errorf("stderr %q lacks %q", log, tc.wantLog)
			}
		})
	}
}

func TestScopedMergeCarryForward_VerifiedResolutionWritesScopedVerdict(t *testing.T) {
	t.Parallel()

	worktree, diff, patchID := divergedCompositionFixture(t)
	p := &compositionProbeE4{}
	o := p.orchestrator(CompositionAuditSnapshot{LaneAuditRef: "ref-4", Diff: diff, PatchID: patchID}, nil, nil,
		func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible, ResolutionDiff: diff}
		})
	if !o.scopedMergeCarryForward(context.Background(), 4, CycleState{ActiveWorktree: worktree, RunID: "r"}, t.TempDir()) {
		t.Fatal("a verified compatible resolution with green gates must carry forward")
	}
	if p.written.Method != scopedReviewMethod || p.written.PatchID != patchID || string(p.written.ComposedDiff) != string(diff) {
		t.Fatalf("written verdict = %+v", p.written)
	}
}
