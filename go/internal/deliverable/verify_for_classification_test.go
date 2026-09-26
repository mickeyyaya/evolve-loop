package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestVerifyForClassification_SalvagesPersistsAndReportsOnce(t *testing.T) {
	t.Parallel()
	fenced := "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n## Ledger Entry\n\n```json\n{\"verdict\": \"PASS\", \"red\": 0}\n```\n"
	ws, root := writeAuditReport(t, fenced)
	c := signalcenter.New()
	var events []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	r.phaseIO = config.StageEnforce // production: the prose fallback is gated off, the sentinel is the sole verdict source
	WithSignals(c)(r)
	in := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root, Cycle: 1685, RunID: "r1685"}
	check := gatesignal.Check{Cycle: 1685, RunID: "r1685", Phase: "audit"}

	res, err := r.VerifyForClassification(check, "audit", rootsFor(in))
	if err != nil || !res.OK {
		t.Fatalf("the salvaged deliverable verifies OK for classification: ok=%v err=%v violations=%+v", res.OK, err, res.Violations)
	}
	if v, ok := phasecontract.ParseVerdictSentinel(res.Content); !ok || v != "PASS" {
		t.Fatalf("the classified bytes carry the repaired sentinel, got ok=%v v=%q", ok, v)
	}
	disk, _ := os.ReadFile(filepath.Join(ws, "audit-report.md"))
	if string(disk) != res.Content {
		t.Fatalf("the repaired bytes are PERSISTED before classification — the file is what the classifier judged")
	}
	count := func(code signalcenter.Code) int {
		n := 0
		for _, e := range events {
			if e.Code == code {
				n++
			}
		}
		return n
	}
	if count(gatesignal.CodeSalvaged) != 1 {
		t.Fatalf("one GATE_CONTRACT_SALVAGED for the one salvage, got %d: %+v", count(gatesignal.CodeSalvaged), events)
	}
	if rr := r.Review(context.Background(), in); !rr.Approve {
		t.Fatalf("the gate approves the file it repaired: %+v", rr)
	}
	if count(gatesignal.CodeSalvaged) != 1 || count(gatesignal.CodeVerified) != 1 {
		t.Errorf("review after the salvage reports Verified once and no second salvage: salvaged=%d verified=%d", count(gatesignal.CodeSalvaged), count(gatesignal.CodeVerified))
	}
}

func TestVerifyForClassification_OutsideEnforceReturnsTheVerifiedBytesUntouched(t *testing.T) {
	t.Parallel()
	fenced := "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n```json\n{\"verdict\": \"PASS\"}\n```\n"
	ws, root := writeAuditReport(t, fenced)
	r := newTestReviewer(config.StageShadow, filepath.Join(t.TempDir(), "breaker.json"), 3)
	r.phaseIO = config.StageEnforce
	in := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root}
	res, err := r.VerifyForClassification(gatesignal.Check{Phase: "audit"}, "audit", rootsFor(in))
	if err != nil || res.OK || res.Content != fenced {
		t.Fatalf("below enforce the gate does not persist a salvage, so the classifier sees the verified bytes as they are: ok=%v err=%v", res.OK, err)
	}
	if disk, _ := os.ReadFile(filepath.Join(ws, "audit-report.md")); string(disk) != fenced {
		t.Fatal("below enforce the file is untouched")
	}
}

func TestVerifyForClassification_BaselineIsRecordedOncePerBlock(t *testing.T) {
	t.Parallel()
	rows := func(root string) int {
		b, err := os.ReadFile(filepath.Join(root, ".evolve", BadVerdictBaselineFile))
		if err != nil {
			return 0
		}
		return len(strings.Split(strings.TrimSpace(string(b)), "\n"))
	}
	// salvageable: recorded once by the engine path; Review then sees a clean file
	fenced := "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n```json\n{\"verdict\": \"PASS\"}\n```\n"
	ws, root := writeAuditReport(t, fenced)
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	r.phaseIO = config.StageEnforce
	in := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root}
	if _, err := r.VerifyForClassification(gatesignal.Check{Phase: "audit"}, "audit", rootsFor(in)); err != nil {
		t.Fatal(err)
	}
	r.Review(context.Background(), in)
	if got := rows(root); got != 1 {
		t.Fatalf("a salvaged block leaves exactly one baseline row, got %d", got)
	}
	// unrecoverable (two candidates): the engine path records nothing however often settle re-probes; Review records once
	two := fenced + "\n```json\n{\"verdict\": \"FAIL\"}\n```\n"
	ws2, root2 := writeAuditReport(t, two)
	r2 := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	r2.phaseIO = config.StageEnforce
	in2 := core.ReviewInput{Phase: "audit", Workspace: ws2, ProjectRoot: root2}
	for i := 0; i < 3; i++ {
		if res, _ := r2.VerifyForClassification(gatesignal.Check{Phase: "audit"}, "audit", rootsFor(in2)); res.OK {
			t.Fatal("two candidates are never salvaged")
		}
	}
	if got := rows(root2); got != 0 {
		t.Fatalf("the engine's re-probes record nothing for an unrecoverable block, got %d", got)
	}
	r2.Review(context.Background(), in2)
	if got := rows(root2); got != 1 {
		t.Fatalf("Review records the unrecoverable block once, got %d", got)
	}
}

func TestPlainVerifier_IsTheCatalogAwareVerifyWithoutSalvage(t *testing.T) {
	t.Parallel()
	fenced := "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n```json\n{\"verdict\": \"PASS\"}\n```\n"
	ws, root := writeAuditReport(t, fenced)
	in := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root}
	res, err := PlainVerifier{PhaseIO: config.StageEnforce}.VerifyForClassification(gatesignal.Check{Phase: "audit"}, "audit", rootsFor(in))
	if err != nil || res.OK || res.Content != fenced {
		t.Fatalf("the Null-Object verifier verifies and never repairs: ok=%v err=%v", res.OK, err)
	}
}

func TestNewReviewerStage_ThreadsBothDialsIntoTheEnginePath(t *testing.T) {
	t.Parallel()
	fenced := "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n```json\n{\"verdict\": \"PASS\"}\n```\n"
	for _, tc := range []struct {
		name         string
		stage        config.Stage
		phaseIO      config.Stage
		wantOK       bool
		wantRepaired bool
	}{
		{"enforce/enforce: the missing sentinel is salvaged and persisted", config.StageEnforce, config.StageEnforce, true, true},
		{"shadow/enforce: below enforce nothing is persisted", config.StageShadow, config.StageEnforce, false, false},
		{"enforce/off: the prose verdict still counts, nothing to salvage", config.StageEnforce, config.StageOff, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws, root := writeAuditReport(t, fenced)
			r := NewReviewerStage(tc.stage, tc.phaseIO)
			r.logf = func(string, ...any) {}
			in := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root}
			res, err := r.VerifyForClassification(gatesignal.Check{Phase: "audit"}, "audit", rootsFor(in))
			if err != nil || res.OK != tc.wantOK {
				t.Fatalf("ok=%v err=%v violations=%+v", res.OK, err, res.Violations)
			}
			disk, _ := os.ReadFile(filepath.Join(ws, "audit-report.md"))
			if repaired := string(disk) != fenced; repaired != tc.wantRepaired {
				t.Fatalf("file repaired=%v, want %v", repaired, tc.wantRepaired)
			}
		})
	}
}
