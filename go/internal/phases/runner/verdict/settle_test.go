package verdict

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestSettle_BoundedAt1PlusRetries_StopsOnOKOrError_CountsReprobes(t *testing.T) {
	var intervals []time.Duration
	never := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}}, WithSleep(func(d time.Duration) { intervals = append(intervals, d) }))
	s := never.e.settle(context.Background(), Identity{}, "audit", phasecontract.Roots{Workspace: never.ws})
	if s.err != nil || s.res.OK || s.attempts != SettleRetries || never.n.verify != 1+SettleRetries || len(intervals) != SettleRetries {
		t.Errorf("never-OK: attempts=%d probes=%d sleeps=%d, want %d/%d/%d", s.attempts, never.n.verify, len(intervals), SettleRetries, 1+SettleRetries, SettleRetries)
	}
	for _, d := range intervals {
		if d != SettleInterval {
			t.Fatalf("every rung sleeps SettleInterval, got %s", d)
		}
	}
	fourth := newHarness(t, probe{okFrom: 4, codes: []string{deliverable.CodeMissingArtifact}})
	s = fourth.e.settle(context.Background(), Identity{}, "audit", phasecontract.Roots{Workspace: fourth.ws})
	if !s.res.OK || s.err != nil || s.attempts != 3 || *fourth.n != (counts{verify: 4, sleep: 3}) {
		t.Errorf("OK on the 4th probe: attempts=%d %+v, want 3 re-probes, 4 probes, 3 sleeps", s.attempts, *fourth.n)
	}
	verr := newHarness(t, probe{err: errors.New("no deliverable contract")})
	s = verr.e.settle(context.Background(), Identity{}, "audit", phasecontract.Roots{Workspace: verr.ws})
	if s.err == nil || s.attempts != 0 || *verr.n != (counts{verify: 1}) {
		t.Errorf("an error ends the ladder at once (uncontracted phases pay zero retries): attempts=%d %+v", s.attempts, *verr.n)
	}
}

func TestSettle_HonoursCancellationBeforeAndAfterTheSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	during := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}}, WithSleep(func(time.Duration) { cancel() }))
	s := during.e.settle(ctx, Identity{}, "audit", phasecontract.Roots{Workspace: during.ws})
	if s.attempts != 0 || during.n.verify != 1 {
		t.Errorf("cancelled during the sleep: one probe, zero re-probes (attempts=%d probes=%d)", s.attempts, during.n.verify)
	}
	before := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}})
	s = before.e.settle(ctx, Identity{}, "audit", phasecontract.Roots{Workspace: before.ws})
	if s.attempts != 0 || *before.n != (counts{verify: 1, sleep: 0}) {
		t.Errorf("cancelled before entry: one probe, no sleep (attempts=%d %+v)", s.attempts, *before.n)
	}
}

func TestRootsFor_CarriesCycleAndEvolveDirOnlyWithAProjectRoot(t *testing.T) {
	d := Dispatch{Cycle: 42, Workspace: "ws", Worktree: "wt", ProjectRoot: "root", ExplanationDocumentationVersion: 3, ArtifactPath: "ws/audit-report.md"}
	got := rootsFor(d)
	want := phasecontract.Roots{Workspace: "ws", Worktree: "wt", EvolveDir: paths.EvolveDirOf("root"), DispatchedArtifact: "ws/audit-report.md", ExplanationDocumentationVersion: 3, Cycle: 42}
	if got != want {
		t.Errorf("rootsFor = %+v, want %+v", got, want)
	}
	d.ProjectRoot = ""
	if got := rootsFor(d); got.EvolveDir != "" {
		t.Errorf("no project root ⇒ no EvolveDir, got %q", got.EvolveDir)
	}
	// A value comparison cannot tell paths.EvolveDirOf from an inline join; only a source scan can.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), `".evolve"`) {
			t.Errorf("%s spells the .evolve layout inline — project it through paths.EvolveDirOf", name)
		}
	}
}
