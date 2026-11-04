package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func (p pausedLane) bind(t *testing.T, id string, cycle int) continuation.Continuation {
	t.Helper()
	c := continuation.Continuation{Cycle: cycle, Branch: "cycle-x", SnapshotSHA: "deadbeef", BaseSHA: "cafebabe"}
	if err := continuation.WriteRegistryEntry(p.root, id, c); err != nil {
		t.Fatal(err)
	}
	return c
}

func (p pausedLane) ancestorLedger(t *testing.T, cycle int, body string) {
	t.Helper()
	ws := paths.RunWorkspace(p.root, cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, defectledger.LedgerFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (p pausedLane) gate() defectledger.Verdict {
	ws := paths.RunWorkspace(p.root, p.cycle)
	return defectledger.New(core.LaneScopeIDs, func(string, defectledger.Request) (bool, string) { return false, "stub" }).
		Reconcile(defectledger.Request{Cycle: p.cycle, Workspace: ws, ProjectRoot: p.root})
}

func (p pausedLane) resume(t *testing.T, stderr *bytes.Buffer, scope ...string) {
	t.Helper()
	var got startedCycle
	if _, err := p.lane(stderr, nil, scope...).run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed == nil {
		t.Fatalf("resume: %v, resumed %+v", err, got.resumed)
	}
}

const owedLedger = `{"origin_cycle":2093,"entries":[{"id":"d1","text":"first defect","status":"OPEN"}]}`

func TestLaneResume_ABindingWhoseAncestorLeftNoLedgerIsReleasedAndTheGateDoesNotBlock(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	var stderr bytes.Buffer

	p.resume(t, &stderr, "item-a")

	if _, ok, err := continuation.ReadRegistryEntry(p.root, "item-a"); ok || err != nil {
		t.Fatalf("registry still binds item-a (%v, %v); want the binding released", ok, err)
	}
	if v := p.gate(); v.Blocked {
		t.Fatalf("gate = %+v; want no block on a released binding", v)
	}
	loc, err := inboxmover.Locate(filepath.Join(p.evolveDir, "inbox"), "item-a")
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(loc.Path); !strings.Contains(string(body), "lane-resume-cycle-2109") {
		t.Errorf("item = %s; want the release recorded with its reason", body)
	}
	if !strings.Contains(stderr.String(), "RELEASED the binding of item-a to cycle-2092") {
		t.Errorf("stderr = %q; want the release line", stderr.String())
	}
}

func TestScopeBinding_AFailedReleaseFallsBackToAdoption(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	c := p.bind(t, "item-a", 2092)
	var stderr bytes.Buffer
	b := scopeBinding{projectRoot: p.root, stderr: &stderr, release: func(string, string) (bool, error) { return false, errors.New("boom") }}

	b.settle(2109, []string{"item-a"})

	if m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2109)); !ok || m.Cycle != c.Cycle {
		t.Fatalf("manifest = %+v, %v; want adoption after the failed release", m, ok)
	}
	if !strings.Contains(stderr.String(), "cannot release the binding of item-a to cycle-2092 (released false, err boom)") {
		t.Errorf("stderr = %q; want the release failure reported", stderr.String())
	}
}

func TestLaneResume_ABindingWhoseAncestorOwesDefectsIsAdoptedIntoTheWorkspace(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2108)
	p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)
	var stderr bytes.Buffer

	p.resume(t, &stderr, "item-a")

	m, ok, err := continuation.ReadManifest(paths.RunWorkspace(p.root, 2108))
	if err != nil || !ok || m.Cycle != 2093 {
		t.Fatalf("manifest = %+v, %v, %v; want the binding to cycle-2093 adopted", m, ok, err)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); !ok {
		t.Fatal("the adopted binding was released; want it kept")
	}
	for _, d := range p.gate().Diagnostics {
		if strings.Contains(d.Message, "holds no continuation manifest") {
			t.Fatalf("gate still reports a missing manifest: %s", d.Message)
		}
	}
	if !strings.Contains(stderr.String(), "ADOPTED the binding of item-a to cycle-2093") {
		t.Errorf("stderr = %q; want the adopt line", stderr.String())
	}
}

func TestLaneResume_ADeletedOrRewrittenManifestOfAnAdoptingCycleStillBlocks(t *testing.T) {
	t.Parallel()
	for _, tamper := range []string{"delete", "rewrite"} {
		t.Run(tamper, func(t *testing.T) {
			p := seedPausedLane(t, "item-a", 2108)
			p.bind(t, "item-a", 2093)
			p.ancestorLedger(t, 2093, owedLedger)
			p.resume(t, &bytes.Buffer{}, "item-a")
			ws := paths.RunWorkspace(p.root, 2108)
			if tamper == "delete" {
				if err := os.Remove(filepath.Join(ws, continuation.ManifestName)); err != nil {
					t.Fatal(err)
				}
			} else if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 1, SnapshotSHA: "deadbeef"}); err != nil {
				t.Fatal(err)
			}

			if v := p.gate(); !v.Blocked {
				t.Fatalf("gate = %+v; want the tampered manifest to block", v)
			}
		})
	}
}

func TestLaneResume_AManifestAlreadyInTheWorkspaceIsLeftForTheGate(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	ws := paths.RunWorkspace(p.root, 2109)
	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 2092, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}

	p.resume(t, &bytes.Buffer{}, "item-a")

	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); !ok {
		t.Fatal("a binding the workspace already adopted was released")
	}
}

func TestLaneResume_TheCycleOwnBindingAndAnUnboundScopeAreLeftAlone(t *testing.T) {
	t.Parallel()
	p := seedPausedScope(t, []string{"item-free", "item-self", "item-b"}, 2109, 0)
	p.bind(t, "item-self", 2109)
	p.bind(t, "item-b", 2092)

	p.resume(t, &bytes.Buffer{}, "item-free", "item-self", "item-b")

	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-b"); !ok {
		t.Fatal("a binding behind the cycle's own binding was released; want the gate's first binding to decide")
	}
	if _, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2109)); ok {
		t.Fatal("a manifest was written for the cycle's own binding")
	}
}

func TestLaneResume_ANewerBindingIsAdoptedNotReleased(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2090)
	p.bind(t, "item-a", 2107)

	p.resume(t, &bytes.Buffer{}, "item-a")

	if m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2090)); !ok || m.Cycle != 2107 {
		t.Fatalf("manifest = %+v, %v; want the newer binding adopted", m, ok)
	}
}

func TestLaneResume_AnUnreadableAncestorLedgerIsAdoptedSoTheGateGradesIt(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	p.ancestorLedger(t, 2092, `{not json`)

	p.resume(t, &bytes.Buffer{}, "item-a")

	if m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2109)); !ok || m.Cycle != 2092 {
		t.Fatalf("manifest = %+v, %v; want the binding adopted", m, ok)
	}
	if v := p.gate(); !v.Blocked {
		t.Fatalf("gate = %+v; want the unreadable ancestor ledger to block", v)
	}
}

func TestLaneResume_AManifestThatCannotBeWrittenIsReported(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2108)
	p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)
	ws := paths.RunWorkspace(p.root, 2108)
	if err := os.MkdirAll(filepath.Join(ws, continuation.ManifestName+".tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	p.resume(t, &stderr, "item-a")

	if !strings.Contains(stderr.String(), "cannot adopt the binding of item-a to cycle-2093") {
		t.Errorf("stderr = %q; want the adopt failure reported", stderr.String())
	}
}

func TestLaneResume_AnUnreadableManifestOrRegistryLeavesTheBindingToTheGate(t *testing.T) {
	t.Parallel()
	t.Run("manifest", func(t *testing.T) {
		p := seedPausedLane(t, "item-a", 2109)
		p.bind(t, "item-a", 2092)
		if err := os.MkdirAll(filepath.Join(paths.RunWorkspace(p.root, 2109), continuation.ManifestName), 0o755); err != nil {
			t.Fatal(err)
		}
		p.resume(t, &bytes.Buffer{}, "item-a")
		if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); !ok {
			t.Fatal("the binding was released past an unreadable manifest")
		}
	})
	t.Run("registry", func(t *testing.T) {
		p := seedPausedLane(t, "item-a", 2109)
		if err := os.WriteFile(continuation.RegistryPath(p.root), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		p.resume(t, &bytes.Buffer{}, "item-a")
		if _, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2109)); ok {
			t.Fatal("a manifest was written from an unreadable registry")
		}
	})
}
