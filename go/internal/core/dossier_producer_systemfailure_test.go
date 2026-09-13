package core

// dossier_producer_systemfailure_test.go — cycle 1663 RED contract for the
// inbox item lost-ship-dossier-evidence (re-filed 2026-08-23 after the
// cycle-1546 salvage excised the draft).
//
// finalizeCycle stamps the landing-lost SystemFailureSignal onto CycleResult
// (lost_landing_floor.go, PR #482) and correctly downgrades the verdict to
// WARN — but writeCycleDossier receives only the terminal outcome string, so
// the committed record (knowledge-base/cycles/cycle-N.{json,md}) carries a
// WARN indistinguishable from any other WARN. An operator reading the dossier
// cannot see WHY the cycle was downgraded; the only evidence is in gitignored
// runtime (ship-error.json vs ship-binding.json, diffed by hand per cycle).
//
// Contract (test-report.md ## AC-Materialization):
//
//	AC1 a landing-lost cycle's committed dossier carries the signal's
//	    structured category AND evidence text, driven through the REAL
//	    writeCycleDossier path — i.e. its two production callers,
//	    cycleRun.completeCycle (cycle_closeout.go) and
//	    cycleRun.abnormalEpilogue (cyclerun_epilogue.go). A seam that only a
//	    direct writeCycleDossier(…) call can reach is dead code.
//	AC2 the landed sibling of the SAME race (same transient ship error, but a
//	    ship-binding) carries NO landing-lost evidence; an ordinary PASS
//	    dossier stays byte-clean (golden captured from the pre-change
//	    producer, the cycle-1652 pin shape).
//	AC3 acs/cycle1544 predicates 004-005 restored — bound to the tests here.
//
// Wire shape pinned here (the one decision the tests must make so they can
// assert on something): the record lands under the top-level JSON key
// `system_failure`, an object carrying at least `category` and `evidence`.
// Generic — mirrors CycleResult.SystemFailure and the signal's own JSON tags
// — so the second producer of SystemFailureSignal (detectVerdictIncoherence)
// is not precluded later; a key named for one category would be. Extra keys
// (level, halt) are allowed, never required. The Go field/type names in
// internal/dossier and the BuildOpts spelling are the Builder's call — the
// FailureRecord/d.Failure pattern (failure.go, dossier.go:54-59) is the
// precedent to mirror; TestSchema_NoDrift enforces the schema half.
//
// Evidence is carried VERBATIM (asserted equal to the signal the floor
// produced, not merely "contains the code"): the floor formats one bounded
// line whose tail is the operator instruction ("rebase + re-verify …"), so a
// FailureRecord-style byte cap would cut exactly the actionable part.
//
// Fixtures are the REAL artifacts of the wave-20260822a-verify race
// (testdata/lostlanding): cycle-1535 lost its landing, cycle-1536 hit the
// same GIT_FLEET_REBASE_NEEDED and landed. The two are distinguishable ONLY
// by ship-binding.json, which is exactly what makes 1536 the right negative.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// systemFailureKey is the pinned top-level wire key (see the file comment).
const systemFailureKey = "system_failure"

// closeoutRun builds a cycleRun whose completeCycle drives the REAL closeout:
// finalizeCycle (the landing-lost floor) → emitCycleClose → writeCycleDossier
// into a git-initialised project root, the same shape
// TestCompleteCycle_ForwardsTheShipLatchToTheOutcomeLabel uses. The
// workspace is the vendored cycle's real ship artifacts.
func closeoutRun(t *testing.T, cycle int, fixture string) (*cycleRun, string) {
	t.Helper()
	root := t.TempDir()
	initDossierRepo(t, root)
	var ws string
	if fixture == "" {
		ws = t.TempDir()
	} else {
		ws = realCycleWorkspace(t, fixture)
	}
	o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
	cr := &cycleRun{
		ctx:    context.Background(),
		o:      o,
		cycle:  cycle,
		req:    CycleRequest{ProjectRoot: root, GoalHash: "lost-ship-dossier-evidence", Context: map[string]string{"goal": "durable closeout evidence for a raced landing"}},
		cs:     CycleState{WorkspacePath: ws, RunID: "run-" + fixture},
		result: CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS, PhasesRun: []Phase{PhaseBuild, PhaseAudit, PhaseShip}},
	}
	return cr, root
}

// systemFailureBlock decodes the pinned wire object, or fails the test naming
// the keys that ARE present so a mis-spelled key is diagnosable from the log.
func systemFailureBlock(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	sf, ok := m[systemFailureKey].(map[string]any)
	if !ok {
		t.Fatalf("RED: dossier carries no %q object; top-level keys=%v", systemFailureKey, dossierTopLevelKeys(m))
	}
	return sf
}

// TestDossierSystemFailure_LostLandingReachesTheCommittedDossier — AC1, the
// headline wiring test. cycle-1535's real artifacts through the real
// completeCycle: the floor fires (that half already ships — asserted as a
// precondition so a RED here can only mean the dossier sink), and the
// committed pair must carry the signal's category and its evidence verbatim,
// while the verdict routing dossierVerdict already does (WARN) is unchanged.
func TestDossierSystemFailure_LostLandingReachesTheCommittedDossier(t *testing.T) {
	cr, root := closeoutRun(t, 1535, "cycle-1535")
	if err := cr.completeCycle(); err != nil {
		t.Fatalf("completeCycle: %v", err)
	}
	// Precondition: the witness half. If this fails the floor regressed, not
	// the sink — keep the two failures distinguishable.
	sig := cr.result.SystemFailure
	if sig == nil || sig.Category != "landing-lost" {
		t.Fatalf("precondition: finalizeCycle must record the landing-lost signal on the result; got %+v", sig)
	}

	m, md := readDossierPair(t, root, 1535)
	if got := m["final_verdict"]; got != dossier.VerdictWarn {
		t.Errorf("final_verdict = %v, want %q (the verdict routing must not change — only evidence is added)", got, dossier.VerdictWarn)
	}
	sf := systemFailureBlock(t, m)
	if got := sf["category"]; got != "landing-lost" {
		t.Errorf("%s.category = %v, want %q", systemFailureKey, got, "landing-lost")
	}
	evidence, _ := sf["evidence"].(string)
	if evidence != sig.Evidence {
		t.Errorf("%s.evidence must be the floor's evidence VERBATIM\n got: %q\nwant: %q", systemFailureKey, evidence, sig.Evidence)
	}
	if !strings.Contains(evidence, "GIT_FLEET_REBASE_NEEDED") {
		t.Errorf("%s.evidence must name the ship-error code an operator has to act on; got %q", systemFailureKey, evidence)
	}
	// The human-readable half must agree with the JSON: an operator reading
	// cycle-N.md sees the category and the code without opening the JSON.
	for _, want := range []string{"landing-lost", "GIT_FLEET_REBASE_NEEDED"} {
		if !strings.Contains(md, want) {
			t.Errorf("dossier md must carry %q; got:\n%s", want, md)
		}
	}
	// The written record must still parse + validate through the package's
	// own trust boundary — a field the reader rejects is not durable.
	jb, err := os.ReadFile(filepath.Join(root, "knowledge-base", "cycles", "cycle-1535.json"))
	if err != nil {
		t.Fatalf("read dossier: %v", err)
	}
	d, err := dossier.ParseJSON(jb)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("a landing-lost dossier must stay valid: %v", err)
	}
}

// TestDossierSystemFailure_LandedSiblingCarriesNone — AC2, the negative that
// stops the feature from turning every contended wave into a wall of false
// evidence. cycle-1536 recorded the SAME transient ship error and LANDED
// (ship-binding commit adcbddb2): through the same real path its dossier
// carries no system-failure object, no landing-lost text, no leaked error
// code — and DOES carry the binding's commit, which proves the producer read
// this very workspace rather than an empty one.
func TestDossierSystemFailure_LandedSiblingCarriesNone(t *testing.T) {
	cr, root := closeoutRun(t, 1536, "cycle-1536")
	if err := cr.completeCycle(); err != nil {
		t.Fatalf("completeCycle: %v", err)
	}
	if cr.result.SystemFailure != nil {
		t.Fatalf("precondition: the landed sibling must not be flagged by the floor; got %+v", cr.result.SystemFailure)
	}
	m, md := readDossierPair(t, root, 1536)
	if got := m["final_verdict"]; got != dossier.VerdictPass {
		t.Errorf("final_verdict = %v, want %q", got, dossier.VerdictPass)
	}
	if got, _ := m["commit_sha"].(string); !strings.HasPrefix(got, "adcbddb2") {
		t.Errorf("commit_sha = %q, want the binding's adcbddb2… (proves the real 1536 workspace was projected)", got)
	}
	if v, present := m[systemFailureKey]; present {
		t.Errorf("a landed cycle must carry no %q object; got %v", systemFailureKey, v)
	}
	for _, leak := range []string{"landing-lost", "GIT_FLEET_REBASE_NEEDED"} {
		if strings.Contains(md, leak) {
			t.Errorf("landed sibling's md leaks %q — the transient error it survived is not evidence of a lost landing:\n%s", leak, md)
		}
	}
}

// goldenPassDossierParams is the FIXED ordinary-PASS input the byte-clean
// golden was captured from through the PRE-change producer (HEAD b9c0df76,
// before any system-failure field existed). Never shipped anything the
// workspace can prove (nonexistent workspace ⇒ no binding, no ship-error): the
// plain "ordinary PASS" every healthy cycle writes. Do not change a value here
// without regenerating the golden through a producer with NO signal support.
func goldenPassDossierParams(projectRoot string) cycleDossierParams {
	return cycleDossierParams{
		ProjectRoot:   projectRoot,
		WorkspacePath: "/nonexistent/workspace/for/golden-pass",
		Cycle:         4243,
		Goal:          "fixed goal text for the ordinary-PASS byte-clean pin",
		RunID:         "01JFIXEDRUNIDFORGOLDEN01",
		Outcome:       CycleOutcomeShippedViaBuild,
		PhaseTimings: []phaseTimingEntry{
			{Phase: "scout", DurationMS: 1200, Verdict: VerdictPASS, StartedAt: "2026-09-14T00:00:00Z", EndedAt: "2026-09-14T00:00:01.2Z", AttemptCount: 1, ResolvedModel: "gpt-5.6-terra", ModelSource: "profile"},
			{Phase: "build", DurationMS: 30000, Verdict: VerdictPASS, StartedAt: "2026-09-14T00:00:02Z", EndedAt: "2026-09-14T00:00:32Z", AttemptCount: 1},
			{Phase: "ship", DurationMS: 4000, Verdict: VerdictPASS, StartedAt: "2026-09-14T00:00:33Z", EndedAt: "2026-09-14T00:00:37Z", AttemptCount: 1},
		},
	}
}

// TestDossierSystemFailure_OrdinaryPassStaysByteClean — AC2's second half,
// in the strongest form the words allow: for an ordinary PASS input with no
// signal, the producer writes EXACTLY the bytes it wrote before the feature —
// no `system_failure: null`, no empty markdown section, no reordered key.
// Also the never-shipped edge: a workspace with no ship artifacts at all has
// no landing to lose.
func TestDossierSystemFailure_OrdinaryPassStaysByteClean(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	if err := writeCycleDossier(nil, goldenPassDossierParams(root)); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	dir := filepath.Join(root, "knowledge-base", "cycles")
	for _, tc := range []struct{ got, golden string }{
		{"cycle-4243.json", "cycle-4243.golden.json"},
		{"cycle-4243.md", "cycle-4243.golden.md"},
	} {
		got, err := os.ReadFile(filepath.Join(dir, tc.got))
		if err != nil {
			t.Fatalf("%s not written: %v", tc.got, err)
		}
		if want := readGolden(t, tc.golden); !bytes.Equal(got, want) {
			t.Errorf("RED: ordinary PASS %s is no longer byte-clean (a nil signal must add NOTHING to the record)\n--- got ---\n%s\n--- want ---\n%s", tc.got, got, want)
		}
		if bytes.Contains(got, []byte(systemFailureKey)) || bytes.Contains(got, []byte("landing-lost")) {
			t.Errorf("RED: ordinary PASS %s mentions a system failure it never had:\n%s", tc.got, got)
		}
	}
}

// TestDossierSystemFailure_AbnormalEpilogueThreadsTheSignal — AC1 at the
// SECOND production caller. A system-class signal can already be stamped on
// CycleResult mid-cycle (the audit and retro chokepoints,
// cyclerun_record.go) before the cycle dies; the abnormal-exit dossier must
// carry it exactly as the normal closeout would, or the two call sites
// diverge and the epilogue silently loses the classification. The signal is
// the real floor's value (detectLostLanding over the 1535 artifacts), not a
// hand-rolled literal. The no-signal row is the anti-fabrication twin.
func TestDossierSystemFailure_AbnormalEpilogueThreadsTheSignal(t *testing.T) {
	cases := []struct {
		name   string
		signal *SystemFailureSignal
	}{
		{"stamped-signal-reaches-the-abnormal-dossier", detectLostLanding(realCycleWorkspace(t, "cycle-1535"), VerdictPASS)},
		{"no-signal-fabricates-nothing", nil},
	}
	if cases[0].signal == nil {
		t.Fatalf("precondition: the floor must produce a signal for the 1535 artifacts")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr, root := epilogueRun(t, false)
			cr.result.SystemFailure = tc.signal
			cr.abnormalEpilogue(nil)
			m, md := readDossierPair(t, root, 77)
			if got := m["final_verdict"]; got != dossier.VerdictFail {
				t.Errorf("abnormal exit final_verdict = %v, want %q", got, dossier.VerdictFail)
			}
			if tc.signal == nil {
				if v, present := m[systemFailureKey]; present {
					t.Errorf("no signal on the result ⇒ no %q object; got %v", systemFailureKey, v)
				}
				if strings.Contains(md, "landing-lost") {
					t.Errorf("no signal on the result ⇒ no landing-lost text in md:\n%s", md)
				}
				return
			}
			sf := systemFailureBlock(t, m)
			if got := sf["category"]; got != tc.signal.Category {
				t.Errorf("%s.category = %v, want %q", systemFailureKey, got, tc.signal.Category)
			}
			if got, _ := sf["evidence"].(string); got != tc.signal.Evidence {
				t.Errorf("%s.evidence must be the stamped signal's evidence VERBATIM\n got: %q\nwant: %q", systemFailureKey, got, tc.signal.Evidence)
			}
			if !strings.Contains(md, tc.signal.Category) {
				t.Errorf("abnormal dossier md must carry the category %q:\n%s", tc.signal.Category, md)
			}
		})
	}
}
