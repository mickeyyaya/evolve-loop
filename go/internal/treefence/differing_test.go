//go:build integration

package treefence

import (
	"context"
	"slices"
	"testing"
)

func TestSnapshot_DifferingNamesThePathsBeyondItsBase(t *testing.T) {
	root := initRepo(t)
	ctx := context.Background()
	tracked, err := TakeStaged(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	full, err := Take(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := full.Differing(ctx, tracked)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"src/new_test.go"}; !slices.Equal(got, want) {
		t.Fatalf("Differing = %v, want %v: the untracked input alone, not the tracked edit both trees hold", got, want)
	}
	if same, err := tracked.Differing(ctx, tracked); err != nil || len(same) != 0 {
		t.Fatalf("a snapshot differs from itself in nothing: %v %v", same, err)
	}
	if back, err := tracked.Differing(ctx, full); err != nil || !slices.Equal(back, []string{"src/new_test.go"}) {
		t.Fatalf("the differing set is the same from either side (%v): %v", err, back)
	}
	write(t, root, "src/mat.go", "package src // a later change\n")
	write(t, root, "z.go", "package z\n")
	later, err := Take(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := later.Differing(ctx, full); err != nil || !slices.Equal(got, []string{"src/mat.go", "z.go"}) {
		t.Fatalf("modified and added paths, sorted (%v): %v", err, got)
	}
}
