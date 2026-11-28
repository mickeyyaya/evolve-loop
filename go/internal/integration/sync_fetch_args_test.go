package integration

import (
	"context"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestSyncToOrigin_FetchesWithMainFetchArgs(t *testing.T) {
	t.Parallel()
	var fetchedArgs []string
	repo := Repo{
		Dir: t.TempDir(),
		Git: func(_ context.Context, _, _ string, args ...string) (string, error) {
			switch args[0] {
			case "fetch":
				fetchedArgs = append([]string(nil), args...)
			case "rev-parse":
				return testSHA, nil
			}
			return "", nil
		},
	}
	got, err := SyncToOrigin(context.Background(), repo)
	if err != nil || got != SyncCurrent {
		t.Fatalf("SyncToOrigin = %q, %v, want %q", got, err, SyncCurrent)
	}
	if !slices.Equal(fetchedArgs, gitexec.FetchMainArgs()) {
		t.Fatalf("fetch args = %v, want %v", fetchedArgs, gitexec.FetchMainArgs())
	}
}
