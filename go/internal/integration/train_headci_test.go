package integration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

type headCIStub struct {
	mu       sync.Mutex
	verdicts map[string]Verdict
	errs     map[string]error
	asked    []string
}

func (s *headCIStub) Watch(_ context.Context, pr PR) (Verdict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, pr.Tip)
	if err := s.errs[pr.Tip]; err != nil {
		return "", err
	}
	if v, ok := s.verdicts[pr.Tip]; ok {
		return v, nil
	}
	return Green, nil
}

func lastCommitOf(rec DeliverableRecord) string { return rec.Commits[len(rec.Commits)-1].SHA }

func keptRefs(plan Plan) []int {
	refs := make([]int, len(plan.Kept))
	for i, rec := range plan.Kept {
		refs[i] = rec.PR
	}
	return refs
}

func TestTrainPlan_EjectsPRWhoseOwnCIIsRed(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	red := r.pr(t, 1, "feat: red head", map[string]string{"a.md": "a\n"})
	green := r.pr(t, 2, "feat: green head", map[string]string{"b.md": "b\n"})
	stub := &headCIStub{verdicts: map[string]Verdict{lastCommitOf(red): Red, lastCommitOf(green): Green}}
	r.train.HeadCI = stub

	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Ejected) != 1 || plan.Ejected[0].PR != 1 || plan.Ejected[0].Cause != CauseHeadCI || plan.Ejected[0].Err == nil {
		t.Fatalf("Ejected = %+v, want PR 1 ejected on %q with a reason", plan.Ejected, CauseHeadCI)
	}
	if !slices.Equal(keptRefs(plan), []int{2}) {
		t.Errorf("Kept = %v, want only the PR whose head is green", keptRefs(plan))
	}
	if !slices.Contains(stub.asked, lastCommitOf(red)) || !slices.Contains(stub.asked, lastCommitOf(green)) {
		t.Errorf("the head CI was asked for %v, want both heads %s and %s", stub.asked, lastCommitOf(red), lastCommitOf(green))
	}
	if !strings.Contains(Brief(plan.Ejected[0]), CauseHeadCI) {
		t.Errorf("the brief %q must name the cause %q", Brief(plan.Ejected[0]), CauseHeadCI)
	}

	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Ejected, []int{1}) || !slices.Equal(out.Landed, []int{2}) {
		t.Errorf("Outcome = %+v, want PR 1 ejected and PR 2 landed", out)
	}
	if len(r.host.comments[1]) != 1 || !strings.Contains(r.host.comments[1][0], CauseHeadCI) {
		t.Errorf("PR 1 comments = %v, want one brief that names %q", r.host.comments[1], CauseHeadCI)
	}
	if tree := r.origin.Git("ls-tree", "-r", "--name-only", "refs/heads/main"); strings.Contains(tree, "a.md") || !strings.Contains(tree, "b.md") {
		t.Errorf("main holds %q, want b.md from PR 2 and no a.md from the red PR 1", tree)
	}
}

func TestTrainPlan_EjectsARedHeadBeforeAnyBatchCIStarts(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	red := r.pr(t, 1, "feat: red head", map[string]string{"a.md": "a\n"})
	r.train.HeadCI = &headCIStub{verdicts: map[string]Verdict{lastCommitOf(red): Red}}

	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil || len(plan.Kept) != 0 || len(plan.Ejected) != 1 || plan.Ejected[0].Cause != CauseHeadCI {
		t.Fatalf("Plan = %+v, %v; want one head-CI ejection and nothing kept", plan, err)
	}
	if _, err := r.train.Land(context.Background(), plan); err != nil {
		t.Fatalf("Land: %v", err)
	}
	if len(r.ci.opened) != 0 || len(r.host.closed) != 0 {
		t.Errorf("opened batches %v, closed PRs %v: a red head must start no batch CI", r.ci.opened, r.host.closed)
	}
}

func TestTrainPlan_EjectsAHeadWithNoRunOrACancelledRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
	}{
		{"no run", fmt.Errorf("%w of required.yml", ErrNoRun)},
		{"cancelled", &RunConclusionError{Run: "9", Conclusion: "cancelled"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newTrainRig(t)
			rec := r.pr(t, 1, "feat: a", map[string]string{"a.md": "a\n"})
			r.train.HeadCI = &headCIStub{errs: map[string]error{lastCommitOf(rec): c.err}}
			plan, err := r.train.Plan(context.Background(), []int{1})
			if err != nil || len(plan.Kept) != 0 || len(plan.Ejected) != 1 || plan.Ejected[0].Cause != CauseHeadCI {
				t.Errorf("Plan = %+v, %v; want a head-CI ejection", plan, err)
			}
		})
	}
}

func TestTrainPlan_KeepsAPendingHeadOutWithoutEjecting(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	pending := r.pr(t, 1, "feat: pending head", map[string]string{"a.md": "a\n"})
	green := r.pr(t, 2, "feat: green head", map[string]string{"b.md": "b\n"})
	r.train.HeadCI = &headCIStub{
		errs:     map[string]error{lastCommitOf(pending): fmt.Errorf("%w: run 9", ErrRunPending)},
		verdicts: map[string]Verdict{lastCommitOf(green): Green},
	}

	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Ejected) != 0 {
		t.Errorf("Ejected = %+v, want none for a head whose CI is still running", plan.Ejected)
	}
	if !slices.Equal(keptRefs(plan), []int{2}) {
		t.Errorf("Kept = %v, want only PR 2: the pending PR 1 stays out of this batch", keptRefs(plan))
	}

	if _, err := r.train.Land(context.Background(), plan); err != nil {
		t.Fatalf("Land: %v", err)
	}
	if len(r.host.comments[1]) != 0 {
		t.Errorf("PR 1 got comments %v, want none: it is not ejected", r.host.comments[1])
	}
	if _, closed := r.host.closed[1]; closed {
		t.Error("PR 1 was closed, want it left open for the next batch")
	}
}

func TestTrainPlan_FailsLoudlyWhenTheHeadCICannotBeRead(t *testing.T) {
	t.Parallel()
	ghDown := errors.New("gh: HTTP 502")
	r := newTrainRig(t)
	rec := r.pr(t, 1, "feat: a", map[string]string{"a.md": "a\n"})
	r.train.HeadCI = &headCIStub{errs: map[string]error{lastCommitOf(rec): ghDown}}
	plan, err := r.train.Plan(context.Background(), []int{1})
	if !errors.Is(err, ghDown) || len(plan.Kept) != 0 || len(plan.Ejected) != 0 {
		t.Errorf("Plan = %+v, %v; want the gh error and no ejection", plan, err)
	}
}

func TestTrainPlan_ABatchOfGreenHeadsPlansAsBefore(t *testing.T) {
	t.Parallel()
	files := map[int]map[string]string{
		1: {"a.txt": "a\n"},
		2: {sharedFile: lineOneX},
		3: {sharedFile: lineOneY},
	}
	plan := func(t *testing.T, withHeadCI bool) Plan {
		r := newTrainRig(t)
		for n := 1; n <= 3; n++ {
			r.pr(t, n, fmt.Sprintf("feat: %d", n), files[n])
		}
		if withHeadCI {
			r.train.HeadCI = &headCIStub{}
		}
		p, err := r.train.Plan(context.Background(), []int{3, 1, 2})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		return p
	}
	before, after := plan(t, false), plan(t, true)
	if !slices.Equal(before.Order, after.Order) || !slices.Equal(keptRefs(before), keptRefs(after)) || len(before.Ejected) != len(after.Ejected) {
		t.Fatalf("green heads changed the plan: order %v->%v kept %v->%v ejected %d->%d", before.Order, after.Order, keptRefs(before), keptRefs(after), len(before.Ejected), len(after.Ejected))
	}
	if len(after.Ejected) != 1 || after.Ejected[0].PR != 3 || after.Ejected[0].Cause != CauseConflict {
		t.Errorf("Ejected = %+v, want PR 3 on the conflict only", after.Ejected)
	}
}
