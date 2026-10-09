package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var wholeRepoGates = []struct {
	name  string
	check func(core.PhaseRequest) ([]string, error)
}{
	{"go vet", goVetCheckDefault},
	{"acs-durable", acsDurableCheckDefault},
	{"integration-tier", integrationTierCheckDefault},
}

func TestWholeRepoGates_UnderivableChangeSet_WarnsNotSilentSkip(t *testing.T) {
	root := enforceFixtureNonGit(t)
	for _, g := range wholeRepoGates {
		off, err := g.check(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()})
		if err == nil {
			t.Errorf("%s gate on an underivable change-set = (%v, nil), want a WARN-carrying error (silent skip is the fail-open defect)", g.name, off)
			continue
		}
		if len(off) > 0 {
			t.Errorf("%s gate must WARN, not FAIL, on a transient underivable change-set; got offenders %v", g.name, off)
		}
		if !strings.Contains(err.Error(), "underivable") {
			t.Errorf("%s gate WARN must name the underivable change-set so the cause is greppable; got %q", g.name, err.Error())
		}
	}
}

func TestWholeRepoGates_CleanDerivableTree_StaySilent(t *testing.T) {
	root := enforceFixtureCleanGit(t)
	for _, g := range wholeRepoGates {
		off, err := g.check(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()})
		if off != nil || err != nil {
			t.Errorf("%s gate on a clean DERIVABLE tree = (%v, %v), want (nil, nil) — no spurious WARN", g.name, off, err)
		}
	}
}

func TestWholeRepoGates_NoGoModule_StaySilent(t *testing.T) {
	root := t.TempDir()
	for _, g := range wholeRepoGates {
		off, err := g.check(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1, Workspace: t.TempDir()})
		if off != nil || err != nil {
			t.Errorf("%s gate with no go module = (%v, %v), want (nil, nil)", g.name, off, err)
		}
	}
}

func TestWholeRepoGates_DerivableAndTouched_StillRun(t *testing.T) {
	req := tierFixture(t)
	withFakeRunner(t, fakeRunFunc(1, "", "bad.go:5:2: declared and not used: x", nil))
	off, err := goVetCheckDefault(req)
	if err != nil {
		t.Fatalf("go vet gate on a touched+derivable cycle: unexpected error %v", err)
	}
	if len(off) == 0 {
		t.Fatal("go vet gate must still report offenders on a real vet failure")
	}
}
