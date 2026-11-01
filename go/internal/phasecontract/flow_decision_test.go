package phasecontract

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var validFlowDecisions = map[string]string{
	FlowSeatPlan:                 `[{"phase":"scout","run":true,"justification":"needed"}]`,
	FlowSeatRePlan:               `[{"phase":"build","run":false,"justification":"done"}]`,
	FlowSeatProposal:             `{"next_phase":"build","insert_phases":[],"justification":"next"}`,
	FlowSeatJudge:                `{"score":0.5,"rationale":"fine","missing_phases":[]}`,
	FlowSeatRetryAdjudicator:     `{"action":"rebuild","reentry_phase":"build","justification":"fixable"}`,
	FlowSeatFailureAdvisor:       `{"cause":"dead_shell","pane_substr":"zsh: command not found","justification":"plain shell"}`,
	FlowSeatTriage:               `{"cycle":3,"top_n":[{"id":"a"}],"committed_floors":["./x"]}`,
	FlowSeatSelfhealOrchestrator: validDiagnosis,
}

func TestFlowDecisions_EachSeatHasADeclaredFileAndASchema(t *testing.T) {
	t.Parallel()
	seats := FlowDecisions()
	if len(seats) != len(validFlowDecisions) {
		t.Fatalf("FlowDecisions() lists %d seats, the schema table has %d", len(seats), len(validFlowDecisions))
	}
	for _, d := range seats {
		if _, ok := validFlowDecisions[d.Seat]; !ok {
			t.Errorf("seat %q has no schema test row", d.Seat)
		}
		if !strings.HasSuffix(d.ArtifactName, ".json") || len(d.Required) == 0 || len(d.Allowed) < len(d.Required) {
			t.Errorf("seat %q: file %q, required %v, allowed %v", d.Seat, d.ArtifactName, d.Required, d.Allowed)
		}
	}
	if _, ok := FlowDecisionFor("scout"); ok {
		t.Fatal("scout is a phase, not a flow seat")
	}
}

func TestFlowDecision_ValidateAcceptsTheDeclaredShape(t *testing.T) {
	t.Parallel()
	for seat, body := range validFlowDecisions {
		d := mustFlowDecision(t, seat)
		if err := d.Validate([]byte(body + "\n")); err != nil {
			t.Errorf("%s: a valid %s was refused: %v", seat, d.ArtifactName, err)
		}
	}
}

func TestFlowDecision_ValidateRejectsAnUnknownFieldAndAMissingField(t *testing.T) {
	t.Parallel()
	for _, d := range FlowDecisions() {
		obj := sampleObject(t, d)
		unknown := withField(obj, `"verdict_from_prose":true`)
		if err := d.Validate(wrap(d, unknown)); err == nil || !strings.Contains(err.Error(), `unknown field "verdict_from_prose"`) {
			t.Errorf("%s: an unknown field must fail, got %v", d.ArtifactName, err)
		}
		for _, key := range d.Required {
			missing := withoutField(t, obj, key)
			if err := d.Validate(wrap(d, missing)); err == nil || !strings.Contains(err.Error(), `missing field "`+key+`"`) {
				t.Errorf("%s: a missing %q must fail, got %v", d.ArtifactName, key, err)
			}
		}
	}
}

func TestFlowDecision_ValidateRejectsProseAndTheWrongShape(t *testing.T) {
	t.Parallel()
	judge := mustFlowDecision(t, FlowSeatJudge)
	plan := mustFlowDecision(t, FlowSeatPlan)
	cases := []struct {
		name string
		d    FlowDecision
		body string
		want string
	}{
		{"prose around the json", judge, `My verdict: {"score":0.8,"rationale":"x"}`, "invalid character"},
		{"trailing prose", judge, `{"score":0.8,"rationale":"x"} done`, "trailing data"},
		{"array for an object", judge, `[{"score":0.8,"rationale":"x"}]`, "is not an object"},
		{"object for an array", plan, `{"phase":"scout","run":true}`, "is not an array"},
		{"an element that is not an object", plan, `["scout"]`, "is not an object"},
		{"a plan entry that names a CLI", plan, `[{"phase":"build","run":true,"cli":"codex-tmux","tier":"deep"}]`, `unknown field "cli"`},
		{"empty file", judge, ``, "EOF"},
	}
	for _, c := range cases {
		if err := c.d.Validate([]byte(c.body)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

func TestParseFlowStage_DefaultsToLegacyAndFlagsAnUnknownValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want FlowStage
		ok   bool
	}{
		{"", FlowStageLegacy, true},
		{"legacy", FlowStageLegacy, true},
		{"shadow", FlowStageShadow, true},
		{"enforce", FlowStageEnforce, true},
		{"strict", FlowStageLegacy, false},
	}
	for _, c := range cases {
		if got, ok := ParseFlowStage(c.in); got != c.want || ok != c.ok {
			t.Errorf("ParseFlowStage(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFlowStageAt_ReadsTheIsolationI8KeyOfThePolicy(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	if got := FlowStageAt("", &warn); got != FlowStageLegacy || warn.Len() != 0 {
		t.Fatalf("no project root: %q, warn %q", got, warn.String())
	}
	root := policyRoot(t, `{"isolation":{"i8":{"stage":"enforce"}}}`)
	if got := FlowStageAt(root, &warn); got != FlowStageEnforce {
		t.Fatalf("stage = %q, want enforce", got)
	}
	if got := FlowStageAt(t.TempDir(), &warn); got != FlowStageLegacy || warn.Len() != 0 {
		t.Fatalf("absent policy: %q, warn %q", got, warn.String())
	}
	if got := FlowStageAt(policyRoot(t, `{"isolation":{"i8":{"stage":"strict"}}}`), &warn); got != FlowStageLegacy || !strings.Contains(warn.String(), `unknown isolation.i8.stage "strict"`) {
		t.Fatalf("unknown stage: %q, warn %q", got, warn.String())
	}
	warn.Reset()
	if got := FlowStageAt(policyRoot(t, `{`), &warn); got != FlowStageLegacy || !strings.Contains(warn.String(), "flow decisions stay at legacy") {
		t.Fatalf("broken policy: %q, warn %q", got, warn.String())
	}
}

func TestFlowDecision_ReadAtEachStage(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatJudge)
	good := `{"score":0.5,"rationale":"file"}`
	other := `{"score":0.9,"rationale":"stdout"}`
	span := func(s string) func() ([]byte, bool) {
		return func() ([]byte, bool) { return []byte(s), s != "" }
	}
	cases := []struct {
		name     string
		stage    FlowStage
		file     string
		stdout   string
		raw      string
		source   string
		disagree bool
		fileErr  bool
	}{
		{"legacy reads a valid file", FlowStageLegacy, good, other, good, FlowSourceFile, false, false},
		{"legacy falls back to stdout", FlowStageLegacy, "", other, other, FlowSourceStdout, false, true},
		{"legacy with neither", FlowStageLegacy, "", "", "", "", false, true},
		{"shadow drives on stdout and records the disagreement", FlowStageShadow, good, other, other, FlowSourceStdout, true, false},
		{"shadow agrees", FlowStageShadow, good, " " + good, " " + good, FlowSourceStdout, false, false},
		{"shadow with only a file", FlowStageShadow, good, "", good, FlowSourceFile, true, false},
		{"shadow with only stdout", FlowStageShadow, "", other, other, FlowSourceStdout, true, true},
		{"enforce reads the file", FlowStageEnforce, good, other, good, FlowSourceFile, false, false},
		{"enforce never reads stdout", FlowStageEnforce, "", other, "", "", false, true},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), d.ArtifactName)
		if c.file != "" {
			if err := os.WriteFile(path, []byte(c.file), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		called := false
		legacy := func() ([]byte, bool) { called = true; return span(c.stdout)() }
		r := d.Read(c.stage, path, time.Time{}, legacy)
		if string(r.Raw) != c.raw || r.Source != c.source || r.Disagree != c.disagree || (r.FileErr != nil) != c.fileErr || r.Stage != c.stage {
			t.Errorf("%s: got raw %q source %q disagree %v fileErr %v stage %q", c.name, r.Raw, r.Source, r.Disagree, r.FileErr, r.Stage)
		}
		if c.stage == FlowStageEnforce && called {
			t.Errorf("%s: enforce called the stdout parser", c.name)
		}
	}
}

func TestFlowDecision_ReadRejectsAFileThatFailsTheSchema(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatJudge)
	path := filepath.Join(t.TempDir(), d.ArtifactName)
	if err := os.WriteFile(path, []byte(`{"score":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r := d.Read(FlowStageEnforce, path, time.Time{}, nil)
	if r.Raw != nil || r.FileErr == nil || !strings.Contains(r.FileErr.Error(), `missing field "rationale"`) {
		t.Fatalf("got raw %q err %v", r.Raw, r.FileErr)
	}
	r = d.Read(FlowStageEnforce, t.TempDir(), time.Time{}, nil)
	if r.FileErr == nil || errors.Is(r.FileErr, os.ErrNotExist) {
		t.Fatalf("a directory at the path must be a read error, got %v", r.FileErr)
	}
}

func TestFlowRead_ViolationEventForEachStage(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatJudge)
	cases := []struct {
		name   string
		read   FlowRead
		ok     bool
		action string
		rule   string
	}{
		{"a valid file", FlowRead{Stage: FlowStageLegacy, Source: FlowSourceFile, Raw: []byte("{}")}, false, "", ""},
		{"legacy fallback", FlowRead{Stage: FlowStageLegacy, Source: FlowSourceStdout, Raw: []byte("{}"), FileErr: os.ErrNotExist}, true, "logged", FlowRuleStdoutFallback},
		{"legacy without a decision", FlowRead{Stage: FlowStageLegacy, FileErr: os.ErrNotExist}, false, "", ""},
		{"shadow agreement", FlowRead{Stage: FlowStageShadow, Source: FlowSourceStdout, Raw: []byte("{}")}, false, "", ""},
		{"shadow disagreement", FlowRead{Stage: FlowStageShadow, Source: FlowSourceStdout, Raw: []byte("{}"), Disagree: true}, true, "logged", FlowRuleShadowDisagree},
		{"enforce refusal", FlowRead{Stage: FlowStageEnforce, FileErr: os.ErrNotExist}, true, "refused", FlowRuleFileInvalid},
	}
	for _, c := range cases {
		e, ok := d.ViolationEvent(c.read, FlowCell{Cycle: 7, Phase: "plan", Origin: "PlanJudge.GradePlan", Path: "/ws/routing-judge.json"})
		if ok != c.ok {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		f := e.Fields
		if e.Code != isolation.ViolationCode(8) || e.Module != signalcenter.ModuleIsolation || e.Kind != signalcenter.KindIsolationViolation ||
			e.Severity != signalcenter.SeverityWarn || e.Cycle != 7 || e.Phase != "plan" || e.Origin != "PlanJudge.GradePlan" {
			t.Errorf("%s: event %+v", c.name, e)
		}
		if f["invariant"] != "I8" || f["stage"] != string(c.read.Stage) || f["action"] != c.action || f["rule"] != c.rule ||
			f["cell"] != FlowSeatJudge || f["target"] != "/ws/routing-judge.json" || f["source"] != "parser" {
			t.Errorf("%s: fields %v", c.name, f)
		}
		if !strings.Contains(e.Reason, "routing-judge.json") {
			t.Errorf("%s: reason %q must name the file", c.name, e.Reason)
		}
	}
}

func TestFlowRead_ViolationReasonCarriesTheFileError(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatJudge)
	e, _ := d.ViolationEvent(FlowRead{Stage: FlowStageEnforce, FileErr: errors.New("missing field \"score\"")}, FlowCell{Path: "p"})
	if !strings.Contains(e.Reason, `missing field "score"`) {
		t.Fatalf("reason %q", e.Reason)
	}
}

func mustFlowDecision(t *testing.T, seat string) FlowDecision {
	t.Helper()
	d, ok := FlowDecisionFor(seat)
	if !ok {
		t.Fatalf("no flow decision for %q", seat)
	}
	return d
}

func sampleObject(t *testing.T, d FlowDecision) string {
	t.Helper()
	body := strings.TrimSpace(validFlowDecisions[d.Seat])
	if d.Shape == JSONShapeArray {
		body = strings.TrimSuffix(strings.TrimPrefix(body, "["), "]")
	}
	return body
}

func wrap(d FlowDecision, obj string) []byte {
	if d.Shape == JSONShapeArray {
		return []byte("[" + obj + "]")
	}
	return []byte(obj)
}

func withField(obj, field string) string {
	return strings.TrimSuffix(obj, "}") + "," + field + "}"
}

func withoutField(t *testing.T, obj, key string) string {
	t.Helper()
	for _, part := range splitTopLevel(strings.TrimSuffix(strings.TrimPrefix(obj, "{"), "}")) {
		if strings.HasPrefix(part, `"`+key+`"`) {
			rest := strings.Replace(obj, part, "", 1)
			rest = strings.ReplaceAll(rest, ",,", ",")
			rest = strings.Replace(rest, "{,", "{", 1)
			return strings.Replace(rest, ",}", "}", 1)
		}
	}
	t.Fatalf("sample %s has no required key %q", obj, key)
	return ""
}

func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

func policyRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMustFlowDecision_PanicsOnAnUnknownSeat(t *testing.T) {
	t.Parallel()
	if d := MustFlowDecision(FlowSeatJudge); d.ArtifactName != "routing-judge.json" {
		t.Fatalf("judge file %q", d.ArtifactName)
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "no flow decision for seat scout") {
			t.Fatalf("recover = %v", r)
		}
	}()
	MustFlowDecision("scout")
}

func TestFlowDecision_ResolveReadsTheStageOfThePolicyAndEmitsTheViolation(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatJudge)
	stdout := func() ([]byte, bool) { return []byte(`{"score":0.7,"rationale":"prose"}`), true }
	cases := []struct {
		policy string
		raw    string
		action string
	}{
		{`{}`, `{"score":0.7,"rationale":"prose"}`, "logged"},
		{`{"isolation":{"i8":{"stage":"enforce"}}}`, "", "refused"},
	}
	for _, c := range cases {
		root := policyRoot(t, c.policy)
		center := signalcenter.New()
		path := filepath.Join(root, d.ArtifactName)
		got := d.Resolve(root, FlowCell{Cycle: 4, Origin: "PlanJudge.GradePlan", Path: path}, stdout, center)
		if string(got) != c.raw {
			t.Errorf("%s: raw %q, want %q", c.policy, got, c.raw)
		}
		events := center.Recent()
		if len(events) != 1 || events[0].Code != isolation.ViolationCode(8) || events[0].Fields["action"] != c.action {
			t.Errorf("%s: events %+v", c.policy, events)
		}
	}
	root := policyRoot(t, `{}`)
	path := filepath.Join(root, d.ArtifactName)
	if err := os.WriteFile(path, []byte(`{"score":0.2,"rationale":"file"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	if got := d.Resolve(root, FlowCell{Path: path}, stdout, center); string(got) != `{"score":0.2,"rationale":"file"}` || len(center.Recent()) != 0 {
		t.Fatalf("a valid file: raw %q, events %v", got, center.Recent())
	}
}

func TestFlowViolationEvent_CarriesTheProseFallbackOfAReportReader(t *testing.T) {
	t.Parallel()
	e := FlowViolationEvent(FlowViolation{Seat: "scout", Stage: FlowStageEnforce, Action: "refused", Rule: FlowRuleProseFallback, Reason: "no handoff"},
		FlowCell{Cycle: 2, Phase: "triage", Origin: "cycleRun.reportFlowFallbacks", Path: "/ws/scout-report.md"})
	f := e.Fields
	if e.Code != isolation.ViolationCode(8) || e.Reason != "no handoff" || e.Cycle != 2 || e.Phase != "triage" ||
		f["cell"] != "scout" || f["stage"] != "enforce" || f["action"] != "refused" || f["rule"] != FlowRuleProseFallback || f["target"] != "/ws/scout-report.md" {
		t.Fatalf("event %+v", e)
	}
}

func TestFlowDecision_ReadRejectsAFileWrittenBeforeTheLaunchAtEveryStage(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatRetryAdjudicator)
	launched := time.Now()
	fresh := func() ([]byte, bool) { return []byte(`{"action":"retry@tdd","justification":"fresh"}`), true }
	for _, stage := range []FlowStage{FlowStageLegacy, FlowStageShadow, FlowStageEnforce} {
		path := filepath.Join(t.TempDir(), d.ArtifactName)
		if err := os.WriteFile(path, []byte(`{"action":"decline","justification":"stale"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		old := launched.Add(-time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		r := d.Read(stage, path, launched, fresh)
		if !errors.Is(r.FileErr, ErrFlowFileStale) || strings.Contains(string(r.Raw), "stale") {
			t.Fatalf("%s: a stale file must never decide: raw %q err %v", stage, r.Raw, r.FileErr)
		}
		e, ok := d.ViolationEvent(r, FlowCell{Path: path})
		if !ok || e.Fields["rule"] != FlowRuleFileStale || e.Fields["action"] != "refused" || e.Fields["stage"] != string(stage) {
			t.Fatalf("%s: event %+v %v", stage, e, ok)
		}
	}
	path := filepath.Join(t.TempDir(), d.ArtifactName)
	if err := os.WriteFile(path, []byte(`{"action":"decline","justification":"this attempt"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := d.Read(FlowStageEnforce, path, launched, nil); r.FileErr != nil || r.Source != FlowSourceFile {
		t.Fatalf("a file of this attempt decides: %+v", r)
	}
}

func TestFlowDecision_ClearRemovesTheFileAndFailsClosed(t *testing.T) {
	t.Parallel()
	d := mustFlowDecision(t, FlowSeatJudge)
	dir := t.TempDir()
	path := filepath.Join(dir, d.ArtifactName)
	if err := d.Clear(path); err != nil {
		t.Fatalf("an absent file is already clear: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Clear(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the file must be gone: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(path, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := d.Clear(path); err == nil || !strings.Contains(err.Error(), "clear the decision file before the launch") {
		t.Fatalf("a path that cannot be removed must fail closed: %v", err)
	}
}
