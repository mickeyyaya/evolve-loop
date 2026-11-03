package request_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
)

func TestForPair_TriageTDDMatchesTriageTDDOverTheWorkspace(t *testing.T) {
	t.Parallel()

	ws := filepath.Join(cycle1946Fixture, "workspace")
	want, err := request.TriageTDD(ws, fixtureResolver(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := request.ForPair(request.PairTriageTDD, request.Site{Workspace: ws, Resolve: fixtureResolver(t)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest() != want.Digest() {
		t.Errorf("digest = %s, want %s", got.Digest(), want.Digest())
	}
}

func TestForPair_TDDBuildReadsTheCycleAcsPackageOfTheWorktree(t *testing.T) {
	t.Parallel()

	wt := t.TempDir()
	pkg := filepath.Join(wt, "go", "acs", "cycle7")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_Thing(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(pkg, "a_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := request.TDDBuild(pkg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := request.ForPair(request.PairTDDBuild, request.Site{Worktree: wt, Cycle: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest() != want.Digest() || len(got.Asks) != 1 {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestForPair_AnUnknownPairIsAnError(t *testing.T) {
	t.Parallel()

	for _, pair := range []string{"bogus", request.PairScoutTriage} {
		if _, err := request.ForPair(pair, request.Site{}); !errors.Is(err, request.ErrNoRequestForPair) {
			t.Errorf("pair %q: err = %v, want ErrNoRequestForPair", pair, err)
		}
	}
}

func TestForPair_TheBuilderErrorsPassThrough(t *testing.T) {
	t.Parallel()

	if _, err := request.ForPair(request.PairTriageTDD, request.Site{Workspace: t.TempDir(), Resolve: fixtureResolver(t)}); !errors.Is(err, request.ErrNoCommittedSet) {
		t.Errorf("triage-tdd err = %v", err)
	}
	if _, err := request.ForPair(request.PairTDDBuild, request.Site{Worktree: t.TempDir(), Cycle: 3}); err == nil {
		t.Error("tdd-build over a worktree without the package must fail")
	}
}
