package bridge

import (
	"bytes"
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func TestDispatchSweep_ATaggedProgramOutsideTheAllowListIsSpared(t *testing.T) {
	t.Parallel()
	agent := hostProc(810, 1, thisDispatch)
	agent.Comm, agent.Args = "/usr/bin/ssh-agent", []string{"ssh-agent"}
	electron := hostProc(811, 1, thisDispatch)
	electron.Comm = "/Applications/X.app/Contents/Frameworks/X Helper (Renderer)"
	family := hostProc(812, 1, thisDispatch)
	family.Comm, family.Args = "/Users/x/.local/bin/claude", []string{"claude", "-p", "x"}
	var events []string
	h := &hostTable{tables: [][]proctree.Process{{agent, electron, family, hostProc(813, 1, thisDispatch)}, nil}, events: &events}

	(&dispatchSweep{id: thisDispatch}).finish(context.Background(), reapDeps(h, &fakeTmux{}, &bytes.Buffer{}, nil))

	if want := []int{812, 813}; !reflect.DeepEqual(sorted(h.sent), want) {
		t.Errorf("signaled %v, want %v: only a launcher and a family binary die by the tag", h.sent, want)
	}
}

func sorted(in []int) []int {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

func TestDispatchSweep_ACwdOrphanOutsideTheAllowListIsStillStopped(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	tests := hostProc(820, 1, "")
	tests.Comm, tests.Args = "/tmp/go-build/pkg.test", []string{"pkg.test"}
	var events []string
	h := &hostTable{tables: [][]proctree.Process{{tests}, nil}, events: &events}
	deps := reapDeps(h, &fakeTmux{}, &bytes.Buffer{}, nil)
	deps.ListCwds = func(context.Context) ([]proctree.Cwd, error) {
		return []proctree.Cwd{{Pid: 820, Ppid: 1, Cwd: worktree}}, nil
	}

	(&dispatchSweep{id: thisDispatch, cwdProofDir: worktree, since: reapT0}).finish(context.Background(), deps)

	if want := []int{820}; !reflect.DeepEqual(h.sent, want) {
		t.Errorf("signaled %v, want %v: the cwd proof stays on the deny table, so an unlisted orphan in the worktree is stopped", h.sent, want)
	}
}

func TestFamilyBinaries_HoldsEveryManifestBinaryIncludingNonTmuxDrivers(t *testing.T) {
	t.Parallel()
	got := FamilyBinaries()
	for _, name := range DriverNames() {
		m, err := loadManifestRaw(name)
		if err != nil || m.Binary == "" {
			continue
		}
		if !slices.Contains(got, m.Binary) {
			t.Errorf("driver %s: binary %q is missing from FamilyBinaries %v", name, m.Binary, got)
		}
	}
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Errorf("FamilyBinaries = %v, want sorted and unique", got)
	}
}

func TestTagKillable_AdmitsEachFamilyBinaryAndEachLauncherButNoHelper(t *testing.T) {
	t.Parallel()
	killable := TagKillable()
	for _, bin := range append(FamilyBinaries(), proctree.Launchers()...) {
		if !killable(proctree.Process{Comm: "/usr/local/bin/" + bin, Args: []string{bin}}) {
			t.Errorf("%s: not killable by tag", bin)
		}
	}
	for _, bin := range []string{"ssh-agent", "gpg-agent", "firefox", "gopls"} {
		if killable(proctree.Process{Comm: bin, Args: []string{bin}}) {
			t.Errorf("%s: killable by tag, want spared", bin)
		}
	}
}
