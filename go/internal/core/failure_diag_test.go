package core

// failure_diag_test.go — unit 02 (ADR-0103): the orchestrator keeps the seam
// every abort site uses (writePhaseFailureDiag, now a method) and the exported
// DeliveryFailureCause facade; the unit-02 writer is built once, reads the
// clock and the Center live, and has ONE construction site. RED first.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The sidecar's error_message embeds the sentinel's text: a wire contract
// even though the variable itself never moves.
func TestErrArtifactTimeout_KeepsTheWireMessage(t *testing.T) {
	if ErrArtifactTimeout.Error() != "core: bridge artifact timeout" {
		t.Fatalf("the wire message is part of the on-disk contract: %q", ErrArtifactTimeout.Error())
	}
}

func TestFailureDiag_LiteralOrchestratorGetsTheWriterOnce(t *testing.T) {
	fixed := time.Date(2026, 9, 13, 3, 4, 5, 0, time.UTC)
	o := &Orchestrator{now: func() time.Time { return fixed }}
	first := o.failureDiag()
	if first == nil || first.SignalsWired() {
		t.Fatalf("a literal orchestrator gets an unwired writer: %v", first)
	}
	if o.failureDiag() != first {
		t.Fatal("the lazily built writer is kept, not rebuilt")
	}
	ws := t.TempDir()
	o.writePhaseFailureDiag(ws, "build", 281, ErrArtifactTimeout, 3)
	raw, err := os.ReadFile(failurediag.SidecarPath(ws, "build"))
	if err != nil || !strings.Contains(string(raw), `"timestamp":"2026-09-13T03:04:05Z"`) || !strings.Contains(string(raw), `"exit_code":81`) {
		t.Fatalf("the facade writes through the writer with the orchestrator's clock: %s (%v)", raw, err)
	}
	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signalcenter.New()))
	if wired.diag == nil {
		t.Fatal("NewOrchestrator builds the writer eagerly, after the options are applied")
	}
	if !wired.failureDiag().SignalsWired() {
		t.Fatal("NewOrchestrator wires the writer to the root's Center")
	}
}

func TestFailureDiag_SeesASignalCenterAppliedAfterConstruction(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if o.failureDiag().SignalsWired() {
		t.Fatal("no Center at construction: the writer is unwired")
	}
	signals, got := recordingCenter()
	WithSignalCenter(signals)(o)
	if !o.failureDiag().SignalsWired() {
		t.Fatal("the late Center is seen through the accessor, not a rebuilt writer")
	}
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(failurediag.SidecarPath(ws, "build"), "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	o.writePhaseFailureDiag(ws, "build", 281, ErrArtifactTimeout, 3)
	warned := eventsOfKind(*got, signalcenter.KindFailureDiagWarning)
	if len(warned) != 1 || warned[0].Code != failurediag.CodeSidecarWriteFailed || warned[0].Origin != "Writer.Write" || warned[0].Phase != "build" {
		t.Fatalf("the writer's warning reaches the late Center: %+v", *got)
	}
}

// The writer is exported now, so visibility no longer makes "one wired writer"
// structurally true: every non-test construction outside the unit must be the
// orchestrator's wiredFailureDiag.
func TestFailureDiagWriter_OneConstructionSite(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const onlySite = "internal/core/failure_diag.go"
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/core/failurediag/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), "failurediag.NewWriter(") && rel != onlySite {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("the unit-02 writer has ONE construction site (%s); these non-test files construct their own: %v", onlySite, offenders)
	}
}
