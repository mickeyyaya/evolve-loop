package specrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func realPremiseChallengeFAIL(t *testing.T) string {
	t.Helper()
	return readFixture(t, "cycle-1528-premise-challenge-report.md")
}

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

func premiseRules(stage string) *phasespec.ClassifyRules {
	return &phasespec.ClassifyRules{
		RequireSections:     []string{"Stated Premise", "Falsification Attempts", "Verdict"},
		VerdictFromSentinel: stage,
	}
}

func adversarialRules(stage string) *phasespec.ClassifyRules {
	return &phasespec.ClassifyRules{
		RequireSections:     []string{"Threat Model", "Findings", "Verdict"},
		VerdictFromSentinel: stage,
	}
}

func TestEvaluateClassify_Enforce_HonorsRealCycle1528FAIL(t *testing.T) {
	got, diags := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageEnforce))
	if got != core.VerdictFAIL {
		t.Fatalf("cycle-1528 premise-challenge stated FAIL and must classify FAIL at enforce; got %q (diags %+v)", got, diags)
	}
}

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

func TestEvaluateClassify_Enforce_RealPASSFixtureStaysPASS(t *testing.T) {
	got, diags := EvaluateClassify(realAdversarialReviewPASS(t), adversarialRules(SentinelStageEnforce))
	if got != core.VerdictPASS {
		t.Fatalf("a stated PASS must stay PASS; got %q (diags %+v)", got, diags)
	}
}

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

func TestEvaluateClassify_Enforce_HonorsWARN(t *testing.T) {
	art := "## Threat Model\nx\n## Findings\ny\n## Verdict\nz\n<!-- evolve-verdict: {\"phase\":\"adversarial-review\",\"verdict\":\"WARN\",\"schema_version\":1} -->\n"
	got, _ := EvaluateClassify(art, adversarialRules(SentinelStageEnforce))
	if got != core.VerdictWARN {
		t.Fatalf("a stated WARN must classify WARN; got %q", got)
	}
}

func TestEvaluateClassify_UnknownStage_FailsLoudly(t *testing.T) {
	got, diags := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules("shadwo"))
	if got != core.VerdictFAIL {
		t.Fatalf("an unknown verdict_from_sentinel stage must FAIL loudly; got %q", got)
	}
	if len(diags) == 0 || !strings.Contains(diags[0].Message, "verdict_from_sentinel") {
		t.Fatalf("the diagnostic must name the offending key; got %+v", diags)
	}
}

func TestEvaluateClassify_OmittedKeyEqualsExplicitOff(t *testing.T) {
	legacy := &phasespec.ClassifyRules{RequireSections: []string{"Stated Premise", "Falsification Attempts", "Verdict"}}
	wantV, wantD := EvaluateClassify(realPremiseChallengeFAIL(t), legacy)
	gotV, gotD := EvaluateClassify(realPremiseChallengeFAIL(t), premiseRules(SentinelStageOff))
	if gotV != wantV || len(gotD) != len(wantD) {
		t.Fatalf("omitted key must equal explicit off: got (%q,%+v) want (%q,%+v)", gotV, gotD, wantV, wantD)
	}
	if gotV != core.VerdictPASS {
		t.Fatalf("legacy behavior is PASS; got %q", gotV)
	}
}

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

func TestEvaluateClassify_EmptyArtifactStillFails(t *testing.T) {
	got, _ := EvaluateClassify("", premiseRules(SentinelStageEnforce))
	if got != core.VerdictFAIL {
		t.Fatalf("an empty artifact must FAIL; got %q", got)
	}
}

func TestClassifyShadow_RecordsWouldFlipOnRealArtifact(t *testing.T) {
	rec, ok := classifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageShadow))
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

func TestClassifyShadow_RecordsAgreement(t *testing.T) {
	rec, ok := classifyShadow(1453, "adversarial-review", realAdversarialReviewPASS(t), adversarialRules(SentinelStageShadow))
	if !ok {
		t.Fatalf("agreement must still produce a record")
	}
	if rec.WouldFlip {
		t.Fatalf("a stated PASS routed as PASS is not a flip; got %+v", rec)
	}
}

func TestClassifyShadow_TakenUnderEnforceToo(t *testing.T) {
	rec, ok := classifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageEnforce))
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

func TestClassifyShadow_OffYieldsNoRecord(t *testing.T) {
	if _, ok := classifyShadow(1528, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules(SentinelStageOff)); ok {
		t.Fatalf("stage off must yield no record")
	}
	if _, ok := classifyShadow(1528, "scout", "anything", nil); ok {
		t.Fatalf("nil rules must yield no record")
	}
}

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

func TestClassifyShadow_UnreadableSentinelIsNotAgreement(t *testing.T) {
	art := "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nno sentinel here\n"
	rec, ok := classifyShadow(1600, "premise-challenge", art, premiseRules(SentinelStageShadow))
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

func TestHooksClassify_WritesTheShadowRecordForAnOptedInPhase(t *testing.T) {
	ws := t.TempDir()
	h := hooks{spec: phasespec.PhaseSpec{Name: "premise-challenge", Classify: premiseRules(SentinelStageShadow)}}

	verdict, _, _ := h.Classify(realPremiseChallengeFAIL(t), core.PhaseRequest{Cycle: 1528, Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("shadow must route unchanged through the hook too; got %q", verdict)
	}
	b, err := os.ReadFile(filepath.Join(ws, VerdictShadowRecordFile("premise-challenge")))
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

func TestHooksClassify_OptedOutPhaseIsUnchanged(t *testing.T) {
	ws := t.TempDir()
	h := hooks{spec: phasespec.PhaseSpec{Name: "scout", Classify: &phasespec.ClassifyRules{RequireSections: []string{"Verdict"}}}}

	verdict, diags, _ := h.Classify("## Verdict\nfine\n", core.PhaseRequest{Cycle: 1528, Workspace: ws}, core.BridgeResponse{})

	if verdict != core.VerdictPASS || len(diags) != 0 {
		t.Fatalf("opted-out phases must be unchanged; got (%q,%+v)", verdict, diags)
	}
	if got := shadowRecordsIn(t, ws); len(got) != 0 {
		t.Fatalf("an opted-out phase must leave NO record; found %v", got)
	}
}

func shadowRecordsIn(t *testing.T, ws string) []string {
	t.Helper()
	hits, err := filepath.Glob(filepath.Join(ws, verdictShadowRecordPrefix+"*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for i, h := range hits {
		hits[i] = filepath.Base(h)
	}
	return hits
}

func TestHooksClassify_NoWorkspaceIsSafe(t *testing.T) {
	h := hooks{spec: phasespec.PhaseSpec{Name: "premise-challenge", Classify: premiseRules(SentinelStageShadow)}}
	verdict, _, _ := h.Classify(realPremiseChallengeFAIL(t), core.PhaseRequest{Cycle: 1528}, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Fatalf("a missing workspace must not change the verdict; got %q", verdict)
	}
	if _, err := os.Stat(VerdictShadowRecordFile("premise-challenge")); !os.IsNotExist(err) {
		t.Fatalf("no workspace must mean no file written to CWD (stat err %v)", err)
	}
}

func TestHooksClassify_TwoJudgmentPhasesInOneCycleBothKeepTheirRecord(t *testing.T) {
	ws := t.TempDir()
	pc := hooks{spec: phasespec.PhaseSpec{Name: "premise-challenge", Classify: premiseRules(SentinelStageShadow)}}
	ar := hooks{spec: phasespec.PhaseSpec{Name: "adversarial-review", Classify: adversarialRules(SentinelStageShadow)}}

	pc.Classify(realPremiseChallengeFAIL(t), core.PhaseRequest{Cycle: 1528, Workspace: ws}, core.BridgeResponse{})
	ar.Classify(realAdversarialReviewPASS(t), core.PhaseRequest{Cycle: 1528, Workspace: ws}, core.BridgeResponse{})

	for _, want := range []struct {
		phase    string
		sentinel string
		flip     bool
	}{
		{"premise-challenge", core.VerdictFAIL, true},
		{"adversarial-review", core.VerdictPASS, false},
	} {
		b, err := os.ReadFile(filepath.Join(ws, VerdictShadowRecordFile(want.phase)))
		if err != nil {
			t.Fatalf("%s lost its shadow record to a sibling phase: %v", want.phase, err)
		}
		var rec VerdictShadowRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatalf("%s record unreadable: %v", want.phase, err)
		}
		if rec.Phase != want.phase || rec.SentinelVerdict != want.sentinel || rec.WouldFlip != want.flip {
			t.Fatalf("%s record was overwritten or mis-scoped: %+v", want.phase, rec)
		}
	}
}

func TestVerdictShadowRecordFile_IsScopedAndSafe(t *testing.T) {
	if a, b := VerdictShadowRecordFile("premise-challenge"), VerdictShadowRecordFile("adversarial-review"); a == b {
		t.Fatalf("two phases must not share a filename: %q", a)
	}
	for _, hostile := range []string{"../escape", "a/b", ""} {
		got := VerdictShadowRecordFile(hostile)
		if strings.ContainsAny(got, `/\`) {
			t.Fatalf("phase %q produced a traversing filename %q", hostile, got)
		}
	}
}

func TestClassifyShadow_StructuralFailureSaysTheSentinelWasNeverConsulted(t *testing.T) {
	art := "## Stated Premise\nonly this one\n<!-- evolve-verdict: {\"phase\":\"premise-challenge\",\"verdict\":\"FAIL\",\"schema_version\":1} -->\n"
	rec, ok := classifyShadow(9999, "premise-challenge", art, premiseRules(SentinelStageShadow))
	if !ok {
		t.Fatalf("an opted-in phase must always produce a record")
	}
	if rec.SentinelConsulted {
		t.Fatalf("structure failed, so the sentinel was never consulted; got %+v", rec)
	}
	if rec.StructuralVerdict != core.VerdictFAIL {
		t.Fatalf("structural verdict must be FAIL; got %+v", rec)
	}
	if strings.Contains(rec.Rationale, "no readable verdict sentinel") {
		t.Fatalf("the report HAS a readable sentinel — the record must not claim otherwise: %q", rec.Rationale)
	}
	if !strings.Contains(rec.Rationale, "never read") {
		t.Fatalf("the record must say the sentinel was not consulted; got %q", rec.Rationale)
	}
}

func TestClassifyShadow_InvalidStageIsNamedInTheRecord(t *testing.T) {
	rec, ok := classifyShadow(9999, "premise-challenge", realPremiseChallengeFAIL(t), premiseRules("shadwo"))
	if !ok {
		t.Fatalf("a declared (if invalid) stage must still produce a record")
	}
	if !strings.Contains(rec.Rationale, "invalid verdict_from_sentinel") {
		t.Fatalf("the record must name the config defect; got %q", rec.Rationale)
	}
}

func TestEvaluateClassify_Enforce_TailSentinelWinsOverEarlierDecoys(t *testing.T) {
	art := "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nz\n" +
		"Here is the contract example:\n```\n<!-- evolve-verdict: {\"phase\":\"premise-challenge\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n```\n" +
		"<!-- evolve-verdict: {\"phase\":\"premise-challenge\",\"verdict\":\"FAIL\",\"schema_version\":1} -->\n"
	got, _ := EvaluateClassify(art, premiseRules(SentinelStageEnforce))
	if got != core.VerdictFAIL {
		t.Fatalf("the LAST sentinel is the real verdict; a fenced decoy must not win. got %q", got)
	}
}

func TestEvaluateClassify_Enforce_LowercaseVerdictFailsOpenLoudly(t *testing.T) {
	art := "## Stated Premise\nx\n## Falsification Attempts\ny\n## Verdict\nz\n<!-- evolve-verdict: {\"phase\":\"premise-challenge\",\"verdict\":\"fail\",\"schema_version\":1} -->\n"
	got, diags := EvaluateClassify(art, premiseRules(SentinelStageEnforce))
	if got != core.VerdictPASS {
		t.Fatalf("a lowercased verdict is not canonical and must fail open; got %q", got)
	}
	var told bool
	for _, d := range diags {
		if strings.Contains(d.Message, `"fail"`) {
			told = true
		}
	}
	if !told {
		t.Fatalf("the discarded verdict must be named in a diagnostic; got %+v", diags)
	}
}

func TestEvaluateClassify_Enforce_HonorsSKIPPED(t *testing.T) {
	art := "## Threat Model\nx\n## Findings\ny\n## Verdict\nz\n<!-- evolve-verdict: {\"phase\":\"adversarial-review\",\"verdict\":\"SKIPPED\",\"schema_version\":1} -->\n"
	got, _ := EvaluateClassify(art, adversarialRules(SentinelStageEnforce))
	if got != core.VerdictSKIPPED {
		t.Fatalf("a stated SKIPPED must classify SKIPPED; got %q", got)
	}
}

func TestHooksClassify_UnwritableWorkspaceIsReportedNotSwallowed(t *testing.T) {
	fileAsWorkspace := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileAsWorkspace, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h := hooks{spec: phasespec.PhaseSpec{Name: "premise-challenge", Classify: premiseRules(SentinelStageShadow)}}

	verdict, diags, _ := h.Classify(realPremiseChallengeFAIL(t), core.PhaseRequest{Cycle: 1528, Workspace: fileAsWorkspace}, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Fatalf("a failed measurement must never change the verdict it measures; got %q", verdict)
	}
	var told bool
	for _, d := range diags {
		if strings.Contains(d.Message, "shadow record not written") {
			told = true
		}
	}
	if !told {
		t.Fatalf("a failed shadow write must surface a diagnostic; got %+v", diags)
	}
}
