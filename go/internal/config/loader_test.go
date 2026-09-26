package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// observed builds a Loader reporting into a recording Center.
func observed(t *testing.T, opts ...Option) (*Loader, *[]signalcenter.Event) {
	t.Helper()
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return New(append(opts, WithSignals(func() *signalcenter.Center { return c }))...), got
}

func readerReturning(raw []byte, err error) Option {
	return WithReadFile(func(string) ([]byte, error) { return raw, err })
}

func TestNew_DefaultsToDiskAndNoCenter(t *testing.T) {
	l := New()
	if l.SignalsWired() {
		t.Fatal("New() wires no Center (the Null Object)")
	}
	reg := writeRegistry(t, `{"config":{"max_optional_insertions":9},"phases":[]}`)
	cfg, ws := l.Load(reg, nil)
	if cfg.MaxInsertions != 9 || len(ws) != 0 {
		t.Fatalf("the default reader reads the disk: MaxInsertions=%d ws=%+v", cfg.MaxInsertions, ws)
	}
}

func TestLoader_UseRegistryZeroNeverCallsTheReader(t *testing.T) {
	calls := 0
	l, events := observed(t, WithReadFile(func(string) ([]byte, error) { calls++; return nil, fs.ErrNotExist }))
	l.Load("/r/docs/architecture/phase-registry.json", map[string]string{"EVOLVE_USE_PHASE_REGISTRY": "0"})
	if calls != 0 || len(*events) != 0 {
		t.Fatalf("\"0\" skips the read: calls=%d events=%d", calls, len(*events))
	}
	l.Load("/r/docs/architecture/phase-registry.json", map[string]string{"EVOLVE_USE_PHASE_REGISTRY": "false"})
	if calls != 1 {
		t.Fatalf("\"false\" keeps the registry on (Q1): calls=%d", calls)
	}
}

func TestLoader_Load_RegistryAbsentIsSilent(t *testing.T) {
	l, events := observed(t, readerReturning(nil, &fs.PathError{Op: "open", Path: "/r/x.json", Err: fs.ErrNotExist}))
	cfg, ws := l.Load("/r/x.json", nil)
	if !reflect.DeepEqual(cfg, defaults()) || len(ws) != 0 || len(*events) != 0 {
		t.Fatalf("absent registry: ws=%+v events=%d", ws, len(*events))
	}
}

func TestLoader_Load_RegistryUnreadableWarnsAndRunsTheBaseline(t *testing.T) {
	const path = "/r/docs/architecture/phase-registry.json"
	readErr := &fs.PathError{Op: "open", Path: path, Err: syscall.EACCES}
	l, events := observed(t, readerReturning(nil, readErr))
	cfg, ws := l.Load(path, nil)
	if !reflect.DeepEqual(cfg, defaults()) {
		t.Errorf("the loop runs on the compiled baseline: %+v", cfg)
	}
	wantMsg := "phase registry unreadable (running on the built-in baseline — no triage, no registry order): read " + path + ": " + readErr.Error()
	wantFields := map[string]string{"step": "registry", "source": "registry", "path": path, "err": readErr.Error()}
	if len(ws) != 1 || ws[0].Code != "registry-unreadable" || ws[0].Message != wantMsg || !reflect.DeepEqual(ws[0].Fields, wantFields) {
		t.Fatalf("warning: %+v\nwant %s %q %v", ws, "registry-unreadable", wantMsg, wantFields)
	}
	if len(*events) != 1 {
		t.Fatalf("one event, got %d", len(*events))
	}
	e := (*events)[0]
	if e.Module != signalcenter.ModuleConfig || e.Kind != signalcenter.KindConfigWarning || e.Severity != signalcenter.SeverityWarn ||
		e.Origin != "Loader.Load" || e.Cycle != 0 || e.Phase != "" || e.Code != CodeRegistryUnreadable || e.Reason != wantMsg || !reflect.DeepEqual(e.Fields, wantFields) {
		t.Fatalf("event: %+v", e)
	}
}

func TestLoader_Load_RegistryMalformedWarnsAndRunsTheBaseline(t *testing.T) {
	const path = "/r/docs/architecture/phase-registry.json"
	l, events := observed(t, readerReturning([]byte("{"), nil))
	cfg, ws := l.Load(path, nil)
	if !reflect.DeepEqual(cfg.Mandatory, []string{"scout", "build", "audit", "ship"}) || cfg.Order != nil {
		t.Errorf("the baseline omits triage and the registry order: %+v", cfg)
	}
	wantMsg := "phase registry malformed (running on the built-in baseline — no triage, no registry order): parse " + path + ": unexpected end of JSON input"
	if len(ws) != 1 || ws[0].Code != "registry-malformed" || ws[0].Message != wantMsg || ws[0].Fields["err"] != "unexpected end of JSON input" || ws[0].Fields["path"] != path || ws[0].Fields["step"] != "registry" {
		t.Fatalf("warning: %+v\nwant %q", ws, wantMsg)
	}
	if len(*events) != 1 || (*events)[0].Code != CodeRegistryMalformed || (*events)[0].Reason != wantMsg {
		t.Fatalf("event: %+v", *events)
	}
}

func TestLoader_Load_StampsStepSourceKeyAndPathPerRange(t *testing.T) {
	env := kitchenSinkEnv()
	env["EVOLVE_CONDITIONAL_MANDATORY"] = "noColon"
	reg := kitchenSinkRegistry()
	_, ws := New().Load(reg, env)
	if len(ws) != 13 {
		t.Fatalf("thirteen warnings (the kitchen sink's twelve plus the missing-':' one), got %d: %+v", len(ws), ws)
	}
	registryKeys := []string{"dynamic_routing", "routing_mode", "model_routing", "deliverable_kinds[document]", "conditional_mandatory[plan-review]", "phases[ship].enabled"}
	for i, key := range registryKeys {
		f := ws[i].Fields
		if f["step"] != "registry" || f["source"] != "registry" || f["path"] != reg || f["key"] != key {
			t.Errorf("registry warning %d fields %v, want step/source=registry path=%s key=%s", i, f, reg, key)
		}
	}
	envKeys := []string{"EVOLVE_COMMIT_EVIDENCE", "EVOLVE_SANDBOX", "EVOLVE_CONDITIONAL_MANDATORY", "EVOLVE_MAX_OPTIONAL_INSERTIONS"}
	for i, key := range envKeys {
		f := ws[6+i].Fields
		if f["step"] != "env" || f["source"] != "env" || f["key"] != key || f["path"] != "" {
			t.Errorf("env warning %d fields %v, want step/source=env key=%s", 6+i, f, key)
		}
	}
	if w := ws[8]; w.Code != "unknown-value" || w.Message != `EVOLVE_CONDITIONAL_MANDATORY="noColon": missing ':' (want phase:expr)` || w.Fields["value"] != "noColon" || w.Fields["err"] == "" {
		t.Errorf("the missing-':' arm warns (D3): %+v", w)
	}
	if f := ws[10].Fields; ws[10].Code != "weak-spine" || f["step"] != "spine" || f["missing"] != "audit+ship" {
		t.Errorf("weak-spine fields: %+v", ws[10])
	}
	if f := ws[11].Fields; ws[11].Code != "spine-order" || f["step"] != "spine" || f["audit_pos"] != "1" || f["ship_pos"] != "0" {
		t.Errorf("spine-order fields: %+v", ws[11])
	}
	if f := ws[12].Fields; ws[12].Code != "inert-phase-enable" || f["step"] != "inert" || f["phase"] != "plan-review" || f["stage"] != "0" {
		t.Errorf("inert-phase-enable fields: %+v", ws[12])
	}
	_, ws = New().Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_DYNAMIC_ROUTING": "bogus", "EVOLVE_ROUTING_MODE": "bogus"})
	if len(ws) != 2 || ws[0].Fields["key"] != "EVOLVE_DYNAMIC_ROUTING" || !strings.HasPrefix(ws[0].Message, "dynamic_routing=") ||
		ws[1].Fields["key"] != "EVOLVE_ROUTING_MODE" || !strings.HasPrefix(ws[1].Message, "routing_mode=") || ws[1].Fields["default"] != "llm" {
		t.Errorf("env-sourced unknown-value: %+v", ws)
	}
}

func TestLoader_Load_EmitsEveryWarningOnceInOrderAndMatchesTheStreamGolden(t *testing.T) {
	l, events := observed(t)
	_, ws := l.Load(kitchenSinkRegistry(), kitchenSinkEnv())
	if len(*events) != len(ws) || len(ws) != 12 {
		t.Fatalf("one event per warning: %d events, %d warnings", len(*events), len(ws))
	}
	type row struct{ Module, Kind, Code, Origin, Step string }
	got := make([]row, 0, len(*events))
	for i, e := range *events {
		if e.Code == signalcenter.CodeMissingCode || e.Code == signalcenter.CodeUnregisteredCode || e.Fields["drift"] != "" {
			t.Errorf("event %d drifted at the Center: %+v", i, e)
		}
		if e.Reason != ws[i].Message || !reflect.DeepEqual(e.Fields, ws[i].Fields) {
			t.Errorf("event %d does not carry warning %d verbatim: %+v vs %+v", i, i, e, ws[i])
		}
		got = append(got, row{string(e.Module), string(e.Kind), string(e.Code), e.Origin, e.Fields["step"]})
	}
	if g, want := marshalIndented(t, got), readGolden(t, "kitchen_sink_signals.golden.json"); string(g) != string(want) {
		t.Fatalf("the stream sequence drifted from the golden:\n%s", diffLines(string(want), string(g)))
	}
}

func TestLoad_FacadeReturnsTheLoaderValuesAndEmitsNothing(t *testing.T) {
	cfg, ws := Load(kitchenSinkRegistry(), kitchenSinkEnv())
	cfg2, ws2 := New().Load(kitchenSinkRegistry(), kitchenSinkEnv())
	if !reflect.DeepEqual(cfg, cfg2) || !reflect.DeepEqual(ws, ws2) {
		t.Fatalf("the facade must return the Loader's values verbatim")
	}
}

func TestApplyRegistry_MapWarningsAreSortedByKey(t *testing.T) {
	var kinds, rules []string
	for _, k := range []string{"k3", "k7", "k1", "k8", "k5", "k2", "k6", "k4"} {
		kinds = append(kinds, `"`+k+`":{"root":"","min_options":0}`)
		rules = append(rules, `"`+k+`":"nonsense"`)
	}
	reg := writeRegistry(t, `{"config":{"deliverable_kinds":{`+strings.Join(kinds, ",")+`},"conditional_mandatory":{`+strings.Join(rules, ",")+`}},"phases":[]}`)
	_, ws := Load(reg, nil)
	if len(ws) != 16 {
		t.Fatalf("sixteen warnings, got %d", len(ws))
	}
	for i := 0; i < 8; i++ {
		want := "k" + string(rune('1'+i))
		if ws[i].Fields["key"] != "deliverable_kinds["+want+"]" || ws[8+i].Fields["key"] != "conditional_mandatory["+want+"]" {
			t.Errorf("position %d: %q / %q, want %s in key order", i, ws[i].Fields["key"], ws[8+i].Fields["key"], want)
		}
	}
}

func TestApplyRegistry_FourStepsAreIndependent(t *testing.T) {
	var ws []Warning
	cfg := defaults()
	applyRegistryDials(&cfg, registryConfig{}, &ws)
	if cfg.MaxInsertions != 4 {
		t.Errorf("a nil max_optional_insertions keeps the default: %d", cfg.MaxInsertions)
	}
	zero := 0
	applyRegistryDials(&cfg, registryConfig{MaxOptionalInsertions: &zero}, &ws)
	if cfg.MaxInsertions != 0 {
		t.Errorf("an explicit 0 is applied (the *int distinguishes nil from 0): %d", cfg.MaxInsertions)
	}
	applyDeliverableKinds(&cfg, map[string]DeliverableKindSpec{}, &ws)
	applyConditional(&cfg, map[string]string{}, &ws)
	if cfg.DeliverableKinds != nil || len(cfg.Conditional) != 1 || len(ws) != 0 {
		t.Errorf("empty maps are no-ops: kinds=%v conditional=%v ws=%v", cfg.DeliverableKinds, cfg.Conditional, ws)
	}
	applyPhases(&cfg, []registryPhase{{Name: ""}, {Name: "a"}, {Name: "b", Routing: &RoutingBlock{RubricHint: []string{"h"}}}}, &ws)
	if !reflect.DeepEqual(cfg.Order, []string{"a", "b"}) || len(cfg.Triggers) != 1 || len(cfg.Triggers["b"].RubricHint) != 1 {
		t.Errorf("phases: Order=%v Triggers=%v (Triggers only when routing is declared; the empty name skipped)", cfg.Order, cfg.Triggers)
	}
}

func TestGateStageAndRouterStage_LaddersTrimAndReject(t *testing.T) {
	for in, want := range map[string]Stage{"0": StageOff, "off": StageOff, " shadow ": StageShadow, "enforce": StageEnforce} {
		if s, ok := GateStage(in); !ok || s != want {
			t.Errorf("GateStage(%q) = %v,%v want %v,true", in, s, ok, want)
		}
		if s, ok := RouterStage(in); !ok || s != want {
			t.Errorf("RouterStage(%q) = %v,%v want %v,true", in, s, ok, want)
		}
	}
	if s, ok := GateStage("advisory"); ok || s != StageOff {
		t.Errorf("GateStage(advisory) = %v,%v want Off,false", s, ok)
	}
	if s, ok := RouterStage("advisory"); !ok || s != StageAdvisory {
		t.Errorf("RouterStage(advisory) = %v,%v want Advisory,true", s, ok)
	}
	for _, in := range []string{"", "enfroce", "on"} {
		if s, ok := GateStage(in); ok || s != StageOff {
			t.Errorf("GateStage(%q) = %v,%v want Off,false", in, s, ok)
		}
		if s, ok := RouterStage(in); ok || s != StageOff {
			t.Errorf("RouterStage(%q) = %v,%v want Off,false", in, s, ok)
		}
	}
}

func TestMustCondRule_PanicsWithTheDocumentedPrefix(t *testing.T) {
	defer func() {
		r := recover()
		if s, ok := r.(string); !ok || !strings.HasPrefix(s, "config: DefaultTddRuleExpr does not parse:") {
			t.Fatalf("panic %v, want the documented prefix", r)
		}
	}()
	if !reflect.DeepEqual(DefaultTddRule(), mustCondRule(DefaultTddRuleExpr)) {
		t.Fatal("DefaultTddRule is mustCondRule(DefaultTddRuleExpr)")
	}
	mustCondRule("garbage")
}

func TestRegistryPath_IsTheDocsArchitectureSpelling(t *testing.T) {
	if got, want := RegistryPath("/r"), filepath.Join("/r", "docs", "architecture", "phase-registry.json"); got != want {
		t.Fatalf("RegistryPath = %q, want %q", got, want)
	}
}

func TestParseSandboxMode_TrimsAndWarnsNamingTheCurrentDefault(t *testing.T) {
	var ws []Warning
	if got := parseSandboxMode(" on ", SandboxModeAuto, &ws); got != SandboxModeOn || len(ws) != 0 {
		t.Errorf("trimmed value: %q ws=%v", got, ws)
	}
	if got := parseSandboxMode("bogus", SandboxModeAuto, &ws); got != SandboxModeAuto {
		t.Errorf("unknown falls back to the current mode: %q", got)
	}
	if len(ws) != 1 || ws[0].Message != `EVOLVE_SANDBOX="bogus" unknown (want auto|on|off), defaulting to "auto"` || ws[0].Fields["default"] != "auto" || ws[0].Fields["key"] != "EVOLVE_SANDBOX" {
		t.Errorf("warning: %+v", ws)
	}
}

func TestLoadDomain_ReadsUnderTheEvolveDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "domain.json"), []byte(`{"domain":"writing"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d, ok, err := LoadDomain(root)
	if err != nil || !ok || d.DefaultDeliverableKind() != DeliverableKindDocument {
		t.Fatalf("LoadDomain = %+v %v %v, want the writing domain → document", d, ok, err)
	}
	if _, ok, err := LoadDomain(t.TempDir()); ok || err != nil {
		t.Fatalf("absence is the ordinary case: ok=%v err=%v", ok, err)
	}
}

func TestNoStderrOrWritesInTheLeaf(t *testing.T) {
	for name, src := range leafSources(t) {
		for _, needle := range []string{"os.Stderr", "fmt.Fprint", "os.WriteFile", "os.Create", "os.MkdirAll"} {
			if strings.Contains(src, needle) {
				t.Errorf("%s writes (%s): the leaf reports through the Center only", name, needle)
			}
		}
	}
}

func TestNoBareWarningLiteralOutsideTheAppender(t *testing.T) {
	for name, src := range leafSources(t) {
		n := strings.Count(src, "Warning{")
		if name == "signals.go" && n == 1 {
			continue
		}
		if n != 0 {
			t.Errorf("%s spells Warning{ %d times: only warn in signals.go mints a Warning", name, n)
		}
	}
}

func leafSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = string(b)
	}
	return out
}

func TestWarning_FieldsMarshalBesideCodeAndMessage(t *testing.T) {
	b, err := json.Marshal(Warning{Code: "weak-spine", Message: "m", Fields: map[string]string{"step": "spine"}})
	if err != nil || string(b) != `{"Code":"weak-spine","Message":"m","Fields":{"step":"spine"}}` {
		t.Fatalf("%s %v", b, err)
	}
}

var _ = errors.New
