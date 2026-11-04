package integration

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func (f composeFixture) holds(tip, file string) bool {
	return slices.Contains(strings.Fields(f.g.Git("ls-tree", "--name-only", tip)), file)
}

func (f composeFixture) members(t *testing.T, tip string) string {
	t.Helper()
	var held []string
	for _, file := range []string{"a.txt", "b.txt", "c.txt"} {
		if f.holds(tip, file) {
			held = append(held, strings.ToUpper(strings.TrimSuffix(file, ".txt")))
		}
	}
	return strings.Join(held, "+")
}

func threeDeliverables(t *testing.T, f composeFixture) []DeliverableRecord {
	t.Helper()
	return []DeliverableRecord{
		f.lane(t, 601, map[string]string{"a.txt": "a\n"}),
		f.lane(t, 602, map[string]string{"b.txt": "b\n"}),
		f.lane(t, 603, map[string]string{"c.txt": "breaks a test\n"}),
	}
}

func TestPrefixCheck_PassesAAndABAndFailsABCThenEjectsCWithTheDiffOfAB(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := threeDeliverables(t, f)
	log := &gateLog{}
	log.fail = func(tip string) error {
		if f.holds(tip, "c.txt") {
			return &GateRed{Failures: []string{"TestC"}}
		}
		return nil
	}
	b := f.batch(t)
	var got PrefixResult
	got, err := enforcing(f, log).PrefixCheck(context.Background(), b, recs)
	if err != nil {
		t.Fatalf("PrefixCheck: %v", err)
	}
	var trials []string
	for _, tip := range log.tips {
		trials = append(trials, f.members(t, tip))
	}
	if !slices.Equal(trials, []string{"A", "A+B", "A+B+C"}) {
		t.Errorf("the gate ran on %v, want [A A+B A+B+C]", trials)
	}
	if !slices.Equal(got.Landed, []int{601, 602}) || len(got.Ejected) != 1 {
		t.Fatalf("PrefixCheck = landed %v ejected %v, want [601 602] and one ejection", got.Landed, got.Ejected)
	}
	ej := got.Ejected[0]
	if ej.Cycle != 603 || ej.Cause != CauseGate {
		t.Errorf("ejection = %+v, want cycle 603 on the gate", ej)
	}
	if want := f.g.Git("diff", b.Base, got.Batch.Tip); strings.TrimSpace(ej.Diff) != want || !strings.Contains(want, "+b") {
		t.Errorf("the ejection of C carries %q, want the diff of A+B %q", ej.Diff, want)
	}
	if f.members(t, got.Batch.Tip) != "A+B" || f.g.Git("rev-parse", "refs/heads/"+b.Branch) != got.Batch.Tip {
		t.Errorf("the batch branch does not hold A+B at %s", got.Batch.Tip)
	}
	for _, rec := range recs {
		onBranch := f.repo.LandedOnRef(context.Background(), rec, b.Branch)
		if onBranch != (rec.Cycle != 603) {
			t.Errorf("cycle %d on the batch branch = %v", rec.Cycle, onBranch)
		}
	}
}

func TestPrefixCheck_AConflictEjectsWithNoGateRunAndTheRestLand(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	edit := func(first string) string {
		return first + "\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	}
	recs := []DeliverableRecord{
		f.lane(t, 611, map[string]string{sharedFile: edit("ONE")}),
		f.lane(t, 612, map[string]string{sharedFile: edit("uno")}),
		f.lane(t, 613, map[string]string{"c.txt": "c\n"}),
	}
	log := &gateLog{}
	got, err := enforcing(f, log).PrefixCheck(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("PrefixCheck: %v", err)
	}
	if !slices.Equal(got.Landed, []int{611, 613}) || len(got.Ejected) != 1 || got.Ejected[0].Cause != CauseConflict || got.Ejected[0].Cycle != 612 {
		t.Fatalf("PrefixCheck = landed %v ejected %+v, want [611 613] and a conflict ejection of 612", got.Landed, got.Ejected)
	}
	if log.calls() != 2 {
		t.Errorf("the gate ran %d times, want 2: the conflicted trial runs none", log.calls())
	}
}

func TestPrefixCheck_AtShadowComposesTextuallyAndCallsNoGate(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	c := Composer{Repo: f.repo, Stage: policy.FleetIntegrationShadow}
	got, err := c.PrefixCheck(context.Background(), f.batch(t), threeDeliverables(t, f))
	if err != nil {
		t.Fatalf("PrefixCheck: %v", err)
	}
	if !slices.Equal(got.Landed, []int{601, 602, 603}) || len(got.Ejected) != 0 || f.members(t, got.Batch.Tip) != "A+B+C" {
		t.Errorf("shadow PrefixCheck = %+v, want all three composed", got)
	}
}

func TestPrefixCheck_NoDeliverableLeavesTheBatch(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	b := f.batch(t)
	got, err := enforcing(f, &gateLog{}).PrefixCheck(context.Background(), b, nil)
	if err != nil || got.Batch != b || got.Landed != nil || got.Ejected != nil {
		t.Errorf("PrefixCheck(nil) = %+v, %v, want the batch unchanged", got, err)
	}
}

func TestPrefixCheck_RefusesABadInputBeforeGit(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := threeDeliverables(t, f)
	b := f.batch(t)
	c := Composer{Repo: Repo{Dir: f.g.Dir, Git: refuseGit(t)}, Stage: policy.FleetIntegrationShadow}
	bad := slices.Clone(recs)
	bad[1].Audit.ArtifactSHA = ""
	inputs := map[string][]DeliverableRecord{
		"a duplicate cycle":   {recs[0], recs[0]},
		"an unauditable item": bad,
	}
	for name, in := range inputs {
		if _, err := c.PrefixCheck(context.Background(), b, in); err == nil {
			t.Errorf("%s: PrefixCheck accepted it", name)
		}
	}
	if _, err := (Composer{Repo: c.Repo}).PrefixCheck(context.Background(), b, recs); err == nil {
		t.Error("PrefixCheck accepted a composer with no stage")
	}
}

func TestPrefixCheck_AGitFailureStopsTheCheck(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := threeDeliverables(t, f)
	b := f.batch(t)
	for _, verb := range []string{"merge-tree", "update-ref"} {
		faulted, after := false, 0
		inner := failOn(verb)
		git := func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
			if faulted {
				after++
			}
			out, err := inner(ctx, dir, stdin, args...)
			faulted = faulted || gitVerb(args) == verb
			return out, err
		}
		c := Composer{Repo: Repo{Dir: f.g.Dir, Git: git}, Stage: policy.FleetIntegrationShadow}
		got, err := c.PrefixCheck(context.Background(), b, recs)
		var ej *Ejection
		if err == nil || errors.As(err, &ej) {
			t.Errorf("%s fault: PrefixCheck = %+v, %v, want a plain error", verb, got, err)
		}
		if after != 0 {
			t.Errorf("%s fault: %d git calls ran after the fault, want 0", verb, after)
		}
	}
	if tip := f.g.Git("rev-parse", "refs/heads/"+b.Branch); tip != b.Tip {
		t.Errorf("a failed check moved the batch branch to %s", tip)
	}
}

func TestPrefixCheck_EveryDeliverableRedLeavesTheBranch(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	b := f.batch(t)
	red := &gateLog{fail: func(string) error { return &GateRed{} }}
	got, err := enforcing(f, red).PrefixCheck(context.Background(), b, threeDeliverables(t, f))
	if err != nil || got.Landed != nil || len(got.Ejected) != 3 || got.Batch != b {
		t.Fatalf("PrefixCheck = %+v, %v, want three ejections and the batch unchanged", got, err)
	}
	if tip := f.g.Git("rev-parse", "refs/heads/"+b.Branch); tip != b.Tip {
		t.Errorf("an all-red check moved the batch branch to %s", tip)
	}
}
