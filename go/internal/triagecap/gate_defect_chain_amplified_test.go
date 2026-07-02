package triagecap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// gate_defect_chain_amplified_test.go — cycle-459 test-amplification lane for
// the F1–F5 gate-defect-chain contract (inbox triagecap-prose-counter-defect).
// Authored black-box from the inbox spec, the TDD contract
// (gate_defect_chain_test.go), and the package's exported doc surface — not
// from the implementation. The TDD tests replay the cycle-448/449 incident;
// these probe the boundaries the incident did not exercise: alternate target
// markers, clause cut-off on either side of a target, per-item dedup, the
// ## dropped section, artifact scale, the ShouldDemote window/template
// bounds, the declaration-primary reject reason, the undeclared-companion
// WARN, and relief idempotency under re-review.

// TestCountCommittedFloors_TargetScopeAmplified probes the F1 target-scoping
// boundaries: a package counts only when named in the clause leading up to a
// target-marked percent; evidence percents on either side of the target must
// not pull their packages into the count.
func TestCountCommittedFloors_TargetScopeAmplified(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		want     int
	}{
		{
			name:     "ASCII >= marker counts like the unicode marker",
			artifact: "## top_n\n- cov-a: raise bridge coverage >= 90%\n",
			want:     1,
		},
		{
			name:     "bare to-N% target marker counts",
			artifact: "## top_n\n- cov-b: push evalgate coverage to 95%\n",
			want:     1,
		},
		{
			name:     "evidence percent BEFORE the target clause does not count its package",
			artifact: "## top_n\n- cov-c: core sits at 83.1% today; raise bridge coverage to ≥90%\n",
			want:     1,
		},
		{
			name:     "evidence percent AFTER the target does not count its package",
			artifact: "## top_n\n- cov-g: raise core coverage to ≥85% (bridge currently at 93.5%)\n",
			want:     1,
		},
		{
			name:     "two target markers with one package each count two floors",
			artifact: "## top_n\n- cov-d: raise bridge coverage to ≥90% and push evalgate to ≥85%\n",
			want:     2,
		},
		{
			name:     "same package targeted twice in one item counts once",
			artifact: "## top_n\n- cov-e: raise bridge coverage to ≥90%, stretch bridge to ≥95%\n",
			want:     1,
		},
		{
			name:     "dropped section floors are not committed",
			artifact: "## top_n\n- fix-bug: Fix the dispatch worktree bug — priority=H\n\n## dropped\n- coverage-x: push core coverage to ≥98%\n",
			want:     0,
		},
		{
			name:     "large artifact: 200 non-floor items do not perturb the count",
			artifact: largeTriageArtifact(),
			want:     3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountCommittedFloors(tt.artifact, knownPkgsFixture); got != tt.want {
				t.Errorf("CountCommittedFloors = %d, want %d", got, tt.want)
			}
		})
	}
}

func largeTriageArtifact() string {
	var b strings.Builder
	b.WriteString("## top_n\n")
	b.WriteString("- coverage-trio: raise swarmrunner, bridge, evalgate coverage ≥98%\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "- task-%d: refactor dispatch step %d — priority=M\n", i, i)
	}
	return b.String()
}

// TestShouldDemote_WindowAndTemplateBoundsAmplified probes the F4 predicate
// directly at its documented bounds: the two most recently RECORDED
// rejections must be an adjacent same-template pair with the newer within
// the demotion window of the current cycle. gapSummary448/449 collapse to
// one template (digit runs normalize); the other template differs in words.
func TestShouldDemote_WindowAndTemplateBoundsAmplified(t *testing.T) {
	const otherTemplate = "cycle 446 failed during build: tmux pane vanished before prompt delivery"
	pair := []FailEntry{{Cycle: 448, Summary: gapSummary448}, {Cycle: 449, Summary: gapSummary449}}
	tests := []struct {
		name    string
		entries []FailEntry
		current int
		want    bool
	}{
		{"empty history never demotes", nil, 450, false},
		{"single rejection never demotes", []FailEntry{{Cycle: 449, Summary: gapSummary449}}, 450, false},
		{"adjacent same-template pair demotes the next cycle", pair, 450, true},
		{"reset-sealed hole is transparent at the predicate level", pair, 451, true},
		{"pair far outside the window never demotes", pair, 456, false},
		{"non-adjacent recorded pair never demotes",
			[]FailEntry{{Cycle: 446, Summary: gapSummary448}, {Cycle: 449, Summary: gapSummary449}}, 450, false},
		{"different reason templates never demote",
			[]FailEntry{{Cycle: 448, Summary: gapSummary448}, {Cycle: 449, Summary: otherTemplate}}, 450, false},
		{"older unrelated record does not shield the recent pair",
			[]FailEntry{{Cycle: 446, Summary: otherTemplate}, {Cycle: 448, Summary: gapSummary448}, {Cycle: 449, Summary: gapSummary449}}, 450, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ShouldDemote(tt.entries, tt.current)
			if got != tt.want {
				t.Errorf("ShouldDemote(current=%d) = %v, want %v", tt.current, got, tt.want)
			}
		})
	}
}

// TestCapReviewer_DeclarationPathRejectListsDeclaredPackages (F5 on the
// declaration-primary path): the counted-package list in the reject reason
// must reflect what the counter actually counted. With a committed_floors
// declaration present, counting is declaration-primary, so the reason must
// list the DECLARED packages — the prose here resolves to none.
func TestCapReviewer_DeclarationPathRejectListsDeclaredPackages(t *testing.T) {
	ws := writeTriageWorkspace(t, "## top_n\n- coverage-agg: push total coverage toward 93%\n")
	decl := `{"committed_floors":["swarmrunner","bridge","evalgate"]}`
	if err := os.WriteFile(filepath.Join(ws, TriageDecisionName()), []byte(decl), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, nil, nil)
	rr := r.Review(context.Background(), reviewIn(ws))
	if rr.Approve {
		t.Fatal("declared 3 floors > cap 2 must reject at enforce")
	}
	for _, pkg := range []string{"swarmrunner", "bridge", "evalgate"} {
		if !strings.Contains(rr.Reason, pkg) {
			t.Errorf("declaration-primary reject reason must list counted package %q:\n%s", pkg, rr.Reason)
		}
	}
}

// TestCapReviewer_CompanionWithoutCommittedFloorsStillWarns (F3): the
// producer check is about the DECLARATION, not the file — a companion that
// exists but carries no committed_floors field leaves a floor-bearing report
// undeclared and must WARN exactly like a missing companion.
func TestCapReviewer_CompanionWithoutCommittedFloorsStillWarns(t *testing.T) {
	ws := writeTriageWorkspace(t, "## top_n\n- coverage-one: Push bridge coverage to ≥98%\n")
	if err := os.WriteFile(filepath.Join(ws, TriageDecisionName()), []byte(`{"note":"no declaration here"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var logs []string
	r := newGateDefectReviewer(config.StageEnforce, nil, nil, &logs)
	if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
		t.Fatalf("1 floor under cap must approve: %s", rr.Reason)
	}
	warned := false
	for _, l := range logs {
		if strings.Contains(l, "triage-decision.json") && strings.Contains(l, "committed_floors") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("companion without committed_floors leaves the report undeclared — must warn; logs:\n%s", strings.Join(logs, "\n"))
	}
}

// TestCapReviewer_ReliefIsIdempotentForTheRelievedCycle (F4): a crash-resume
// re-review of the SAME relieved cycle must keep the relief (one cycle means
// that cycle, not one Review call) and must not file a second defect —
// otherwise a resume after the demotion would burn the cycle the relief was
// granted to.
func TestCapReviewer_ReliefIsIdempotentForTheRelievedCycle(t *testing.T) {
	root := newGapRoot(t)
	pair := []FailEntry{{Cycle: 448, Summary: gapSummary448}, {Cycle: 449, Summary: gapSummary449}}
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, pair, nil)

	if res := r.Review(context.Background(), writeGapWorkspace(t, root, 451)); !res.Approve {
		t.Fatalf("cycle 451 must consume the pair's relief: %s", res.Reason)
	}
	if res := r.Review(context.Background(), writeGapWorkspace(t, root, 451)); !res.Approve {
		t.Fatalf("re-reviewing the relieved cycle 451 must stay relieved (crash-resume), got reject: %s", res.Reason)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "auto-heuristic-demotion-*.json"))
	if len(matches) != 1 {
		t.Errorf("re-review of the relieved cycle must not file a second defect; found %d", len(matches))
	}
}
