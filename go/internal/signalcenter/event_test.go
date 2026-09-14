package signalcenter

// event_test.go — the ONE schema and its closed vocabularies (ADR-0101 decisions 1, 2, 4;
// design §4–§5). Every rule here is one the Center enforces on Emit: an event that
// breaks one is never dropped — it is stamped with a SIGNALCENTER_* code, keeps the
// raw values in Fields, and is raised to at least WARN, so the drift is visible in the
// same file the operator reads.

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSeverity_LevelsMatchTheSchemaContract(t *testing.T) {
	t.Parallel()
	// observer-severity.md: INFO=10, WARN=20, INCIDENT=30; no other tier exists.
	cases := []struct {
		sev   Severity
		level int
	}{{SeverityInfo, 10}, {SeverityWarn, 20}, {SeverityIncident, 30}}
	for _, tc := range cases {
		if !tc.sev.Valid() || tc.sev.Level() != tc.level {
			t.Errorf("%q: Valid=%v Level=%d, want valid at %d", tc.sev, tc.sev.Valid(), tc.sev.Level(), tc.level)
		}
	}
	if Severity("ERROR").Valid() || Severity("").Valid() || Severity("error").Level() != 0 {
		t.Error("ERROR/empty/lower-case are not tiers of the contract")
	}
	if !SeverityIncident.AtLeast(SeverityWarn) || !SeverityWarn.AtLeast(SeverityWarn) || SeverityInfo.AtLeast(SeverityWarn) {
		t.Error("AtLeast must order INFO < WARN < INCIDENT inclusively")
	}
}

func TestModule_ClosedSet(t *testing.T) {
	t.Parallel()
	for _, m := range []Module{ModuleOrchestrator, ModuleAdvisor, ModuleRunner, ModuleBridge, ModuleLiveness, ModuleShip,
		ModuleAudit, ModuleTriage, ModuleScout, ModuleBuild, ModuleTDD, ModuleGateContract, ModuleGateEval, ModuleGateRepo,
		ModuleInbox, ModuleConfig, ModuleLoop, ModuleWatchdog, ModuleObserver, ModuleDashboard, ModuleLedger, ModuleOutcome, ModuleFailureDiag, ModuleCarryover, ModuleFailureLearning, ModuleSignalCenter} {
		if !m.Known() {
			t.Errorf("%q must be in the closed module set", m)
		}
	}
	if Module("").Known() || Module("Orchestrator").Known() || Module("core").Known() {
		t.Error("empty, differently-cased and undeclared modules are unknown")
	}
	if got := Modules(); len(got) != 26 || !sortedModules(got) {
		t.Errorf("Modules() must list the 26 declared modules sorted, got %v", got)
	}
}

func sortedModules(ms []Module) bool {
	for i := 1; i < len(ms); i++ {
		if ms[i-1] >= ms[i] {
			return false
		}
	}
	return true
}

func TestKind_ClosedSetAndTerminal(t *testing.T) {
	t.Parallel()
	terminal := map[Kind]bool{
		KindPhaseAborted: true, KindGateRejected: true, KindShipError: true, KindSystemFailure: true,
		KindQuotaPaused: true, KindLoopHalt: true,
	}
	all := []Kind{KindPhaseDispatched, KindPhaseOutcome, KindPhaseAborted, KindGateRejected, KindGateCorrected,
		KindShipLanded, KindShipError, KindShipWarning, KindRunnerWarning, KindInboxWarning, KindAuditWarning, KindSystemFailure, KindQuotaPaused, KindBridgeWarning, KindBridgeTripwire,
		KindPaneLiveness, KindLedgerAppended, KindOutcomeWarning, KindFailureDiagWarning, KindCarryoverWarning, KindFailureLearningWarning, KindConfigWarning, KindObserverWarning, KindGatePassed, KindCycleSealed, KindLoopWave, KindLoopHalt, KindLoopEscalation,
		KindListenerPanicked, KindRegistryDrift, KindSinkDropped}
	for _, k := range all {
		if !k.Known() {
			t.Errorf("%q must be known", k)
		}
		if k.Terminal() != terminal[k] {
			t.Errorf("%q: Terminal=%v, want %v (design §5.2)", k, k.Terminal(), terminal[k])
		}
	}
	if Kind("phase.Outcome").Known() || Kind("").Known() {
		t.Error("kinds are exact, lower-case, dotted")
	}
	if got := Kinds(); len(got) != len(all) {
		t.Errorf("Kinds() = %d kinds, want %d", len(got), len(all))
	}
}

func TestCode_FormatAndModulePrefix(t *testing.T) {
	t.Parallel()
	if !Code("SHIP_GIT_PUSH_REJECTED").Valid() || !Code("GATE_CONTRACT_MISSING_EFFECT").Valid() {
		t.Error("MODULE_SNAKE_CASE codes are valid")
	}
	for _, bad := range []Code{"", "ship_git", "SHIP", "SHIP-GIT", "Ship_Git", "_SHIP_X", "SHIP__X"} {
		if bad.Valid() {
			t.Errorf("%q must be invalid", bad)
		}
	}
	if !Code("GATE_CONTRACT_MISSING_EFFECT").BelongsTo(ModuleGateContract) || Code("SHIP_X").BelongsTo(ModuleOrchestrator) {
		t.Error("a code belongs to the module whose upper-cased, dot-to-underscore name prefixes it")
	}
}

func TestEvent_JSONGoldenLine(t *testing.T) {
	t.Parallel()
	e := Event{
		SchemaVersion: SchemaVersion, Seq: 41, PID: 9055, TS: "2026-09-13T17:46:02.114Z",
		Cycle: 1636, RunID: "01J", Phase: "triage", Attempt: 1,
		Module: ModuleOrchestrator, Origin: "Orchestrator.recordPhaseOutcome", Kind: KindPhaseOutcome,
		Code: "ORCHESTRATOR_PHASE_VERDICT_FAIL", Severity: SeverityWarn,
		Reason: "triage verdict=FAIL: top_n card names protected surface",
		Fields: map[string]string{"verdict": "FAIL", "archetype": "plan"},
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":"signal/1.0","seq":41,"pid":9055,"ts":"2026-09-13T17:46:02.114Z","cycle":1636,"run_id":"01J","phase":"triage","attempt":1,"module":"orchestrator","origin":"Orchestrator.recordPhaseOutcome","kind":"phase.outcome","code":"ORCHESTRATOR_PHASE_VERDICT_FAIL","severity":"WARN","reason":"triage verdict=FAIL: top_n card names protected surface","fields":{"archetype":"plan","verdict":"FAIL"}}`
	if string(raw) != want {
		t.Errorf("golden line drifted:\n got %s\nwant %s", raw, want)
	}
	minimal := Event{SchemaVersion: SchemaVersion, Seq: 1, PID: 7, TS: "t", Module: ModuleLoop, Origin: "run", Kind: KindLoopWave, Severity: SeverityInfo, Reason: "wave 1 open"}
	raw, _ = json.Marshal(minimal)
	for _, absent := range []string{`"cycle"`, `"run_id"`, `"phase"`, `"attempt"`, `"code"`, `"fields"`} {
		if strings.Contains(string(raw), absent) {
			t.Errorf("empty %s must be omitted: %s", absent, raw)
		}
	}
	var back Event
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, minimalNoFields(minimal)) {
		t.Errorf("round trip lost data: %+v (%v)", back, err)
	}
}

func minimalNoFields(e Event) Event { e.Fields = nil; return e }

func validEvent() Event {
	return Event{
		Module: ModuleShip, Origin: "Landing.Land", Kind: KindShipError, Code: "SHIP_GIT_PUSH_REJECTED",
		Severity: SeverityWarn, Reason: "push rejected", Fields: map[string]string{"class": "transient"},
	}
}

func init() {
	RegisterCode(ModuleShip, "SHIP_GIT_PUSH_REJECTED", "test fixture: the remote refused the push")
}

// Normalize is the validation the Center applies before fan-out: it never rejects,
// it rewrites — the first violation (in the fixed order below) becomes the event's
// code, every violation is listed in fields.drift, the raw values are kept, and the
// severity is raised to at least WARN.
func TestNormalize_ValidEventIsUntouched(t *testing.T) {
	t.Parallel()
	in := validEvent()
	out, drift := Normalize(in)
	if len(drift) != 0 || !reflect.DeepEqual(out, in) {
		t.Errorf("a valid event must pass through untouched: drift=%v out=%+v", drift, out)
	}
}

func TestNormalize_StampsAndRaisesOnEveryViolation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mut  func(*Event)
		code Code
		raw  string // the fields key that keeps the raw value
	}{
		{"unknown module", func(e *Event) { e.Module = "core" }, CodeUnknownModule, "raw_module"},
		{"unknown kind", func(e *Event) { e.Kind = "phase.exploded" }, CodeUnknownKind, "raw_kind"},
		{"unknown severity", func(e *Event) { e.Severity = "ERROR" }, CodeUnknownSeverity, "raw_severity"},
		{"missing code on WARN", func(e *Event) { e.Code = "" }, CodeMissingCode, ""},
		{"unregistered code", func(e *Event) { e.Code = "SHIP_NEW_THING" }, CodeUnregisteredCode, "raw_code"},
		{"code of another module", func(e *Event) { e.Code = "SIGNALCENTER_UNKNOWN_KIND" }, CodeUnregisteredCode, "raw_code"},
		{"missing reason", func(e *Event) { e.Reason = "  " }, CodeMissingReason, ""},
		{"bad origin", func(e *Event) { e.Origin = "ship/gitops.go:12" }, CodeBadOrigin, "raw_origin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := validEvent()
			tc.mut(&in)
			out, drift := Normalize(in)
			if len(drift) == 0 || drift[0] != tc.code {
				t.Fatalf("drift = %v, want first %q", drift, tc.code)
			}
			if out.Code != tc.code || !out.Severity.AtLeast(SeverityWarn) || !out.Module.Known() || !out.Kind.Known() || !out.Severity.Valid() {
				t.Errorf("stamped event must carry the drift code, a known module/kind, a valid severity ≥ WARN: %+v", out)
			}
			if out.Fields["drift"] != string(tc.code) {
				t.Errorf("fields.drift must list the violations, got %q", out.Fields["drift"])
			}
			if tc.raw != "" && out.Fields[tc.raw] == "" {
				t.Errorf("the raw value must survive in fields.%s: %+v", tc.raw, out.Fields)
			}
			if out.Reason == "" {
				t.Error("a stamped event still needs a reason")
			}
		})
	}
}

func TestNormalize_InfoNeedsNoCode_IncidentKeepsIncident(t *testing.T) {
	t.Parallel()
	info := validEvent()
	info.Severity, info.Code, info.Kind = SeverityInfo, "", KindShipLanded
	if out, drift := Normalize(info); len(drift) != 0 || out.Code != "" {
		t.Errorf("INFO without a code is valid, got drift=%v code=%q", drift, out.Code)
	}
	inc := validEvent()
	inc.Severity, inc.Kind, inc.Code = SeverityIncident, KindSystemFailure, ""
	if out, _ := Normalize(inc); out.Severity != SeverityIncident || out.Code != CodeMissingCode {
		t.Errorf("a stamped INCIDENT stays INCIDENT (never lowered) and gets the drift code: %+v", out)
	}
	multi := validEvent()
	multi.Module, multi.Reason = "core", ""
	if _, drift := Normalize(multi); len(drift) != 2 || drift[0] != CodeUnknownModule || drift[1] != CodeMissingReason {
		t.Errorf("every violation is listed in the fixed order, got %v", drift)
	}
}

func TestNormalize_FieldsAreBounded(t *testing.T) {
	t.Parallel()
	in := validEvent()
	in.Fields = map[string]string{"Bad Key": "x", "ok_key": "line\nbreak\x01ctl", "verdict": strings.Repeat("v", 5000)}
	for i := 0; i < 20; i++ {
		in.Fields["k"+string(rune('a'+i))] = "v"
	}
	out, drift := Normalize(in)
	if len(drift) != 0 {
		t.Errorf("field bounds are enforced silently (not drift): %v", drift)
	}
	if _, kept := out.Fields["Bad Key"]; kept {
		t.Error("keys must match ^[a-z][a-z0-9_]{0,31}$; a bad key is dropped")
	}
	if strings.ContainsAny(out.Fields["ok_key"], "\n\x01") {
		t.Errorf("values are sanitized (no raw control characters): %q", out.Fields["ok_key"])
	}
	if n := len(out.Fields); n > MaxFields {
		t.Errorf("at most %d fields survive, got %d", MaxFields, n)
	}
	if out.Fields["truncated"] == "" {
		t.Error("over-cap keys are dropped and fields.truncated records how many")
	}
	if raw, _ := json.Marshal(out); len(raw) > MaxLineBytes {
		t.Errorf("the rendered line is capped at %d bytes, got %d", MaxLineBytes, len(raw))
	}
	if len(out.Fields["verdict"]) > 600 {
		t.Errorf("a value is rune-capped by the shared sanitizer, got %d bytes", len(out.Fields["verdict"]))
	}
}

func TestOriginConvention(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"finalizeOutcome", "Orchestrator.recordPhaseOutcome", "Center.Emit", "_x"} {
		if !ValidOrigin(ok) {
			t.Errorf("%q is a valid Func or Type.Method origin", ok)
		}
	}
	for _, bad := range []string{"", "a.b.c", "core/x.go:1", "Type.", ".Method", "with space"} {
		if ValidOrigin(bad) {
			t.Errorf("%q is not a valid origin", bad)
		}
	}
}

func TestNormalize_LineCapDropsLargestFieldsFirst(t *testing.T) {
	t.Parallel()
	in := validEvent()
	in.Fields = map[string]string{}
	for i := 0; i < MaxFields; i++ {
		in.Fields["f"+string(rune('a'+i))] = strings.Repeat("x", 400) // 12 × 400 ≈ 4.8 KiB > MaxLineBytes
	}
	out, drift := Normalize(in)
	if len(drift) != 0 {
		t.Errorf("the line cap is hygiene, not drift: %v", drift)
	}
	raw, _ := json.Marshal(out)
	if len(raw) > MaxLineBytes {
		t.Fatalf("line still %d bytes > %d", len(raw), MaxLineBytes)
	}
	n, _ := strconv.Atoi(out.Fields["truncated"])
	if n < 1 || len(out.Fields) >= MaxFields {
		t.Errorf("fields were dropped largest-first and counted: truncated=%q, %d fields left", out.Fields["truncated"], len(out.Fields))
	}
}

// The ≤ MaxLineBytes guarantee (one O_APPEND write stays atomic) must hold for
// ANY event: a hostile Reason (JSON escapes '<' to six bytes), an over-long but
// regex-valid Origin, and oversized Phase/RunID — not only for Fields (go
// review, S1). Fields go first; then Reason is cut to fit; identifiers are
// bounded to MaxIdentRunes.
func TestNormalize_LineCapHoldsForAnyEvent(t *testing.T) {
	t.Parallel()
	in := validEvent()
	in.Reason = strings.Repeat("<", 3000)
	in.Origin = strings.Repeat("A", 2000)
	in.Phase, in.RunID = strings.Repeat("<", 1000), strings.Repeat("<", 1000) // bounded to 128 runes, 6 bytes each once escaped
	in.Fields = map[string]string{}
	for i := 0; i < MaxFields; i++ {
		in.Fields["f"+string(rune('a'+i))] = strings.Repeat("&", 400)
	}
	out, _ := Normalize(in)
	raw, _ := json.Marshal(out)
	if len(raw) > MaxLineBytes {
		t.Fatalf("line is %d bytes > %d — the cap only looked at Fields", len(raw), MaxLineBytes)
	}
	for name, v := range map[string]string{"origin": out.Origin, "phase": out.Phase, "run_id": out.RunID} {
		if n := len([]rune(v)); n > MaxIdentRunes {
			t.Errorf("%s is %d runes, bound is %d", name, n, MaxIdentRunes)
		}
	}
	if !strings.HasSuffix(out.Reason, "…") || out.Reason == "…" {
		t.Errorf("a Reason cut to fit keeps a prefix and ends with an ellipsis: %q", out.Reason)
	}
	if out.Fields["truncated"] == "" {
		t.Errorf("dropped fields are counted before the reason is cut: %+v", out.Fields)
	}
}

func TestCutRunes_KeepsAPrefixOrNothing(t *testing.T) {
	t.Parallel()
	if got := cutRunes("abcdefghijkl", 6); got != "abcdefghi…" {
		t.Errorf("6 bytes over removes 6/6+1 = 2 runes plus room for the ellipsis: %q", got)
	}
	if got := cutRunes("ab", 600); got != "" {
		t.Errorf("nothing left to keep becomes empty, never a lone ellipsis (the loop's stop): %q", got)
	}
}
