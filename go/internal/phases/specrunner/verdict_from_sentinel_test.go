package specrunner

// verdict_from_sentinel_test.go — a judgment phase's STATED verdict must be able
// to reach the orchestrator.
//
// The defect these tests pin (inbox judgment-phase-semantic-verdict-never-read,
// weight 0.93): EvaluateClassify decided a spec-driven phase's verdict from
// STRUCTURE ONLY. cycle-1528's premise-challenge concluded "FAIL (BLOCK). The
// cycle must not proceed as framed" with premise.severity_max == CRITICAL, AND
// emitted the canonical machine sentinel saying FAIL — and the cycle ran on
// through tdd, build, adversarial-review, audit, retro. Measured across this
// repo's whole run history: 225 of 225 judgment reports carry a well-formed
// sentinel, 100 of them say FAIL, and every one classified PASS.
//
// The fixtures are REAL artifacts, not synthetic ones, because the acceptance
// criterion is that the LIVE population parses — a hand-written fixture proves
// only that the parser handles what its author imagined.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// realPremiseChallengeFAIL is cycle-1528's verbatim report: the live objection
// that was correct (it falsified the plan's load-bearing premise and the
// resulting redesign shipped as ADR-0090) and changed nothing.
func realPremiseChallengeFAIL(t *testing.T) string {
	t.Helper()
	return readFixture(t, "cycle-1528-premise-challenge-report.md")
}

// realAdversarialReviewPASS is cycle-1453's verbatim report — a genuine PASS,
// so the no-false-positive direction is pinned against live bytes too.
func realAdversarialReviewPASS(t *testing.T) string {
	t.Helper()
	return readFixture(t, "cycle-1453-adversarial-review-report.md")
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// premiseRules mirrors .evolve/phases/premise-challenge/phase.json.
func premiseRules(stage string) *phasespec.ClassifyRules {
	return &phasespec.ClassifyRules{
		RequireSections:     []string{"Stated Premise", "Falsification Attempts", "Verdict"},
		VerdictFromSentinel: stage,
	}
}

// adversarialRules mirrors .evolve/phases/adversarial-review/phase.json.
func adversarialRules(stage string) *phasespec.ClassifyRules {
	return &phasespec.ClassifyRules{
		RequireSections:     []string{"Threat Model", "Findings", "Verdict"},
		VerdictFromSentinel: stage,
	}
}

// THE headline regression: the real ignored objection must now be FAIL.
func TestEvaluateClassify_Enforce_HonorsRealCycle1528FAIL(t *testing.T) {
	got, diags := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageEnforce))
	if got != core.VerdictFAIL {
		t.Fatalf("cycle-1528 premise-challenge stated FAIL and must classify FAIL at enforce; got %q (diags %+v)", got, diags)
	}
}

// The SAME artifact under the rollout stage must route EXACTLY as it does today.
// A shadow stage that changes routing is not a shadow stage.
func TestEvaluateClassify_Shadow_RoutingUnchangedOnRealCycle1528(t *testing.T) {
	shadowVerdict, _ := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageShadow))
	legacyVerdict, _ := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageOff))
	if shadowVerdict != legacyVerdict {
		t.Fatalf("shadow must not change routing: shadow=%q legacy=%q", shadowVerdict, legacyVerdict)
	}
	if shadowVerdict != core.VerdictPASS {
		t.Fatalf("today's behavior for a well-formed report is PASS; got %q", shadowVerdict)
	}
}

// Shadow must still SAY what it would have done — a silent shadow measures nothing.
func TestEvaluateClassify_Shadow_DisclosesTheWouldBeVerdict(t *testing.T) {
	_, diags := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageShadow))
	var found bool
	for _, d := range diags {
		if strings.Contains(d.Message, "verdict_from_sentinel") && strings.Contains(d.Message, core.VerdictFAIL) {
			found = true
		}
	}
	if !found {
		t.Fatalf("shadow must disclose the would-be FAIL in a diagnostic; got %+v", diags)
	}
}

// The no-false-positive direction, against live bytes.
func TestEvaluateClassify_Enforce_RealPASSFixtureStaysPASS(t *testing.T) {
	got, diags := EvaluateClassify(realAdversarialReviewPASS(t), adversarialRules(SentinelStageEnforce))
	if got != core.VerdictPASS {
		t.Fatalf("a stated PASS must stay PASS; got %q (diags %+v)", got, diags)
	}
}

// FAIL-OPEN: an absent or unparseable sentinel keeps today's verdict, so a
// malformed report can never hard-block a cycle.
func TestEvaluateClassify_Enforce_FailsOpenWhenSentinelAbsent(t *testing.T) {
	for _, tc := range []struct{ name, artifact string }{
		{"absent", "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nlooks fine to me\n"},
		{"malformed json", "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nz\n<!-- evolve-verdict: {not json} -->\n"},
		{"verdict-less payload", "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nz\n<!-- evolve-verdict: {\"phase\":\"premise-challenge\"} -->\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := EvaluateClassify(tc.artifact, premiseRules(SentinelStageEnforce))
			if got != core.VerdictPASS {
				t.Fatalf("fail-open: %s sentinel must keep today's PASS; got %q (diags %+v)", tc.name, got, diags)
			}
		})
	}
}

// WARN is a real stated verdict (99 of the 225 live reports say WARN) and must
// carry through — silently upgrading it to PASS would re-create this defect for
// the most common non-clean outcome.
func TestEvaluateClassify_Enforce_HonorsWARN(t *testing.T) {
	art := "## Threat Model\nx\n## Findings\ny\n## Verdict\nz\n<!-- evolve-verdict: {\"phase\":\"adversarial-review\",\"verdict\":\"WARN\",\"schema_version\":1} -->\n"
	got, _ := EvaluateClassify(art, adversarialRules(SentinelStageEnforce))
	if got != core.VerdictWARN {
		t.Fatalf("a stated WARN must classify WARN; got %q", got)
	}
}

// A typo'd stage must FAIL LOUDLY, never silently disable the gate — the same
// cycle-241 declared-semantics rule EvaluateClassify already applies to
// fail_if_signal and verdict_on_pass.
func TestEvaluateClassify_UnknownStage_FailsLoudly(t *testing.T) {
	got, diags := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules("shadwo"))
	if got != core.VerdictFAIL {
		t.Fatalf("an unknown verdict_from_sentinel stage must FAIL loudly; got %q", got)
	}
	if len(diags) == 0 || !strings.Contains(diags[0].Message, "verdict_from_sentinel") {
		t.Fatalf("the diagnostic must name the offending key; got %+v", diags)
	}
}

// NO-REGRESSION: every phase that does not declare the key is byte-identical.
func TestEvaluateClassify_StageOff_ByteIdenticalToLegacy(t *testing.T) {
	legacy := &phasespec.ClassifyRules{RequireSections: []string{"Stated Premise", "Falsification Attempts", "Verdict"}}
	wantV, wantD := EvaluateClassify(realPremiseChallengeFAIL(t), legacy)
	gotV, gotD := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageOff))
	if gotV != wantV || len(gotD) != len(wantD) {
		t.Fatalf("stage off must be byte-identical: got (%q,%+v) want (%q,%+v)", gotV, gotD, wantV, wantD)
	}
	if gotV != core.VerdictPASS {
		t.Fatalf("legacy behavior is PASS; got %q", gotV)
	}
}

// Structure is still evaluated FIRST: a truncated report that happens to carry a
// PASS sentinel must not launder itself past the section requirement.
func TestEvaluateClassify_StructuralFailurePrecedesSentinel(t *testing.T) {
	art := "## Stated Premise\nonly this one\n<!-- evolve-verdict: {\"phase\":\"premise-challenge\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
	got, diags := EvaluateClassify(art, premiseRules(SentinelStageEnforce))
	if got != core.VerdictFAIL {
		t.Fatalf("missing sections must FAIL regardless of the sentinel; got %q", got)
	}
	if len(diags) == 0 || !strings.Contains(diags[0].Message, "missing required section") {
		t.Fatalf("the structural diagnostic must survive; got %+v", diags)
	}
}

// An empty artifact stays FAIL — the sentinel path must not resurrect it.
func TestEvaluateClassify_EmptyArtifactStillFails(t *testing.T) {
	got, _ := EvaluateClassify("", premiseRules(SentinelStageEnforce))
	if got != core.VerdictFAIL {
		t.Fatalf("an empty artifact must FAIL; got %q", got)
	}
}

// The durable measurement record: what the soak actually reads.
func TestClassifyShadow_RecordsWouldFlipOnRealArtifact(t *testing.T) {
	rec, ok := ClassifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageShadow))
	if !ok {
		t.Fatalf("a shadow-staged phase must produce a record")
	}
	if rec.Cycle != 1528 || rec.Phase != "premise-challenge" {
		t.Fatalf("record must identify its cycle/phase; got %+v", rec)
	}
	if !rec.SentinelPresent || rec.SentinelVerdict != core.VerdictFAIL {
		t.Fatalf("record must carry the stated FAIL; got %+v", rec)
	}
	if rec.StructuralVerdict != core.VerdictPASS || rec.EffectiveVerdict != core.VerdictPASS {
		t.Fatalf("shadow's effective verdict is the structural one; got %+v", rec)
	}
	if !rec.WouldFlip {
		t.Fatalf("stated FAIL vs routed PASS is exactly the disagreement the soak exists to count; got %+v", rec)
	}
	if rec.Stage != SentinelStageShadow {
		t.Fatalf("record must name the stage it was taken under; got %q", rec.Stage)
	}
}

// Agreement must be recorded too — a record written only on disagreement
// measures a biased sample and cannot produce a flip RATE.
func TestClassifyShadow_RecordsAgreement(t *testing.T) {
	rec, ok := ClassifyShadow(1453, "adversarial-review", realAdversarialReviewPASS(t), adversarialRules(SentinelStageShadow))
	if !ok {
		t.Fatalf("agreement must still produce a record")
	}
	if rec.WouldFlip {
		t.Fatalf("a stated PASS routed as PASS is not a flip; got %+v", rec)
	}
}

// Under enforce the record is still taken — the operator needs the same column
// after promotion, otherwise the measurement dies exactly when it starts mattering.
func TestClassifyShadow_TakenUnderEnforceToo(t *testing.T) {
	rec, ok := ClassifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageEnforce))
	if !ok {
		t.Fatalf("enforce must still produce a record")
	}
	if rec.EffectiveVerdict != core.VerdictFAIL {
		t.Fatalf("under enforce the effective verdict is the stated one; got %+v", rec)
	}
	if rec.WouldFlip {
		t.Fatalf("under enforce the stated verdict IS the routed one, so nothing is being suppressed; got %+v", rec)
	}
}

// A phase that never opted in must not pay for a record it did not ask for.
func TestClassifyShadow_OffYieldsNoRecord(t *testing.T) {
	if _, ok := ClassifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageOff)); ok {
		t.Fatalf("stage off must yield no record")
	}
	if _, ok := ClassifyShadow(1528, "scout", "anything", nil); ok {
		t.Fatalf("nil rules must yield no record")
	}
}

// A sentinel the parser READS but this system has no verdict for ("MAYBE") is a
// distinct case from an unreadable one: ParseVerdictSentinel accepts any
// non-empty verdict string. Without the canonical-verdict guard that value would
// be routed as a verdict, and a phase could invent one.
func TestEvaluateClassify_Enforce_NonCanonicalStatedVerdictFailsOpen(t *testing.T) {
	art := "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nz\n<!-- evolve-verdict: {\"phase\":\"premise-challenge\",\"verdict\":\"MAYBE\",\"schema_version\":1} -->\n"
	got, diags := EvaluateClassify(art, premiseRules(SentinelStageEnforce))
	if got != core.VerdictPASS {
		t.Fatalf("a non-canonical stated verdict must fail open to the structural verdict; got %q", got)
	}
	var told bool
	for _, d := range diags {
		if strings.Contains(d.Message, "MAYBE") {
			told = true
		}
	}
	if !told {
		t.Fatalf("failing open must SAY so — a silently discarded conclusion is the defect being fixed; got %+v", diags)
	}
}

// The record must distinguish "stated nothing readable" from "stated PASS".
// Collapsing them would let malformed reports inflate the agreement rate an
// operator promotes on.
func TestClassifyShadow_UnreadableSentinelIsNotAgreement(t *testing.T) {
	art := "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nno sentinel here\n"
	rec, ok := ClassifyShadow(1600, "premise-challenge", art, premiseRules(SentinelStageShadow))
	if !ok {
		t.Fatalf("an opted-in phase must always produce a record")
	}
	if rec.SentinelPresent || rec.SentinelVerdict != "" {
		t.Fatalf("an unreadable sentinel must not be recorded as a stated verdict; got %+v", rec)
	}
	if rec.WouldFlip {
		t.Fatalf("nothing was suppressed, so this is not a flip; got %+v", rec)
	}
	if !strings.Contains(rec.Rationale, "fail-open") {
		t.Fatalf("the record must explain itself; got %q", rec.Rationale)
	}
}

// The written artifact is what a soak sweep actually reads — pin the filename
// and that it round-trips, not merely that the struct was built.
func TestWriteVerdictShadow_RoundTripsIntoTheWorkspace(t *testing.T) {
	ws := t.TempDir()
	rec, ok := ClassifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageShadow))
	writeVerdictShadow(ws, rec, ok)

	b, err := os.ReadFile(filepath.Join(ws, VerdictShadowRecordFile))
	if err != nil {
		t.Fatalf("shadow record must land at %s: %v", VerdictShadowRecordFile, err)
	}
	var got VerdictShadowRecord
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("record must be valid JSON: %v", err)
	}
	if got.SentinelVerdict != core.VerdictFAIL || !got.WouldFlip || got.Cycle != 1528 {
		t.Fatalf("round-tripped record lost its datum: %+v", got)
	}
}

// An opted-out phase must leave no file behind.
func TestWriteVerdictShadow_OffWritesNothing(t *testing.T) {
	ws := t.TempDir()
	rec, ok := ClassifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageOff))
	writeVerdictShadow(ws, rec, ok)
	if _, err := os.Stat(filepath.Join(ws, VerdictShadowRecordFile)); !os.IsNotExist(err) {
		t.Fatalf("an opted-out phase must write no shadow record (stat err %v)", err)
	}
}

// THE WIRING TEST. Everything above proves the components are correct; this
// proves they FIRE. The defect being fixed is a correct signal nobody read, and
// the fix's own first mutation-test survivor was "Classify stops writing the
// record" — the identical shape one layer up. A judgment phase's spec goes in,
// a shadow record must come out.
func TestHooksClassify_WritesTheShadowRecordForAnOptedInPhase(t *testing.T) {
	ws := t.TempDir()
	h := hooks{spec: phasespec.PhaseSpec{Name: "premise-challenge", Classify: premiseRules(SentinelStageShadow)}}

	verdict, _, _ := h.Classify(realPremiseChallengeFAIL(t), core.PhaseRequest{Cycle: 1528, Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("shadow must route unchanged through the hook too; got %q", verdict)
	}
	b, err := os.ReadFile(filepath.Join(ws, VerdictShadowRecordFile))
	if err != nil {
		t.Fatalf("Classify must write the shadow record: %v", err)
	}
	var rec VerdictShadowRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("record must be valid JSON: %v", err)
	}
	if rec.Phase != "premise-challenge" || rec.Cycle != 1528 {
		t.Fatalf("Classify must pass the phase and cycle through; got %+v", rec)
	}
	if rec.SentinelVerdict != core.VerdictFAIL || !rec.WouldFlip {
		t.Fatalf("the record must carry the suppressed FAIL; got %+v", rec)
	}
}

// The same hook, for a phase that never opted in, must be byte-identical to the
// legacy path: same verdict, and no file.
func TestHooksClassify_OptedOutPhaseIsUnchanged(t *testing.T) {
	ws := t.TempDir()
	h := hooks{spec: phasespec.PhaseSpec{Name: "scout", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Verdict"}}}}

	verdict, diags, _ := h.Classify("## Verdict\nfine\n", core.PhaseRequest{Cycle: 1528, Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS || len(diags) != 0 {
		t.Fatalf("opted-out phases must be unchanged; got (%q,%+v)", verdict, diags)
	}
	if _, err := os.Stat(filepath.Join(ws, VerdictShadowRecordFile)); !os.IsNotExist(err) {
		t.Fatalf("an opted-out phase must leave no record (stat err %v)", err)
	}
}

// A phase running without a workspace (unit paths, dry runs) must not crash or
// invent a file at the process CWD.
func TestHooksClassify_NoWorkspaceIsSafe(t *testing.T) {
	h := hooks{spec: phasespec.PhaseSpec{Name: "premise-challenge", Classify: premiseRules(SentinelStageShadow)}}
	verdict, _, _ := h.Classify(realPremiseChallengeFAIL(t), core.PhaseRequest{Cycle: 1528}, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Fatalf("a missing workspace must not change the verdict; got %q", verdict)
	}
	if _, err := os.Stat(VerdictShadowRecordFile); !os.IsNotExist(err) {
		t.Fatalf("no workspace must mean no file written to CWD (stat err %v)", err)
	}
}
