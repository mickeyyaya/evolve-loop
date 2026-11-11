package integration

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestConsoleTwoCommitPRLandsAsTwoIntegrateCommits(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.f.consoleSteps(t, 7, map[string]string{"a.txt": "a\n"}, map[string]string{"b.txt": "b\n"})
	r.f.g.Git("push", "-q", "origin", "pr-7:refs/pull/7/head")
	if _, err := WriteRecord(WaveDir(r.root, ConsoleWave), rec); err != nil {
		t.Fatal(err)
	}
	before := r.originMain()
	plan, err := r.train.Plan(context.Background(), []int{7})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{7}) {
		t.Fatalf("Landed = %v, want [7]", out.Landed)
	}
	after := r.originMain()
	if n := strings.TrimSpace(r.origin.Git("rev-list", "--count", before+".."+after)); n != "2" {
		t.Errorf("a PR of two commits must land as two commits, got %s", n)
	}
	if one := strings.TrimSpace(r.origin.Git("rev-list", "--count", "--grep=Source-PR: #7", before+".."+after)); one != "2" {
		t.Errorf("Source-PR commits = %s, want 2", one)
	}
}
