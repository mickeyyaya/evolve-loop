package filter

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestParse_KeysAreANDedAndValuesAreORed(t *testing.T) {
	f := mustParse(t, "kind=loop.exit,cycle.sealed  module=orchestrator")
	cases := []struct {
		name   string
		kind   signalcenter.Kind
		module signalcenter.Module
		want   bool
	}{
		{"second value of kind and the module", "cycle.sealed", "orchestrator", true},
		{"first value of kind and the module", "loop.exit", "orchestrator", true},
		{"kind matches but module does not", "cycle.sealed", "ship", false},
		{"module matches but kind does not", "ship.landed", "orchestrator", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := sealedEvent()
			e.Kind, e.Module = tc.kind, tc.module
			if got := f.Match(signalRecord(e)); got != tc.want {
				t.Fatalf("Match(kind=%s module=%s) = %v, want %v", tc.kind, tc.module, got, tc.want)
			}
		})
	}
}

func TestParse_AnEmptyExpressionSelectsEveryEvent(t *testing.T) {
	f := mustParse(t, "   ")
	if !f.Match(signalRecord(sealedEvent())) {
		t.Fatal("an empty route must select every event")
	}
}

func TestParse_AKindGlobThatMatchesNoKindIsRefused(t *testing.T) {
	for _, expr := range []string{"kind=nosuch.*", "kind!=nosuch.*", "kind=ship.*,nosuch.*"} {
		_, _, err := Parse(expr, testCatalog())
		if !errors.Is(err, ErrRefused) {
			t.Fatalf("Parse(%q) error = %v, want ErrRefused", expr, err)
		}
	}
	f := mustParse(t, "kind=ship.*")
	e := sealedEvent()
	e.Kind = "ship.landed"
	if !f.Match(signalRecord(e)) {
		t.Fatal("kind=ship.* must match ship.landed")
	}
}

func TestParse_AnUnknownKindOrModuleIsRefused(t *testing.T) {
	for _, expr := range []string{"kind=no.such", "module=nosuch", "module!=nosuch"} {
		_, _, err := Parse(expr, testCatalog())
		if !errors.Is(err, ErrRefused) {
			t.Fatalf("Parse(%q) error = %v, want ErrRefused", expr, err)
		}
	}
}

func TestParse_AnUnknownCodeOnlyWarns(t *testing.T) {
	cases := []struct {
		expr     string
		warnings int
	}{
		{"code=NEWER_BUILD_CODE", 1},
		{"code=NEWER_*,SHIP_GATE_RED", 1},
		{"code=SKILLS_DRIFT_*", 0},
		{"code=SHIP_GATE_RED", 0},
	}
	for _, tc := range cases {
		_, warnings, err := Parse(tc.expr, testCatalog())
		if err != nil {
			t.Fatalf("Parse(%q) error = %v, want nil", tc.expr, err)
		}
		if len(warnings) != tc.warnings {
			t.Fatalf("Parse(%q) warnings = %v, want %d", tc.expr, warnings, tc.warnings)
		}
	}
	f, _, _ := Parse("code=NEWER_BUILD_CODE", testCatalog())
	e := sealedEvent()
	e.Code = "NEWER_BUILD_CODE"
	if !f.Match(signalRecord(e)) {
		t.Fatal("a filter on an unknown code must still match that code")
	}
}

func TestParse_AGlobOnAnotherKeyIsAUsageError(t *testing.T) {
	for _, expr := range []string{"module=sh*", "phase=aud*", "fields.final_verdict=P*", "source=lo*"} {
		_, _, err := Parse(expr, testCatalog())
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("Parse(%q) error = %v, want ErrUsage", expr, err)
		}
	}
}

func TestParse_MalformedTermsAreUsageErrors(t *testing.T) {
	cases := map[string]string{
		"unknown key":              "colour=red",
		"empty fields name":        "fields.=x",
		"no operator":              "kind",
		"no key":                   "=loop.exit",
		"no value":                 "kind=",
		"an empty value":           "kind=loop.exit,",
		"order on a text key":      "phase>=audit",
		"order with two values":    "severity>=WARN,INCIDENT",
		"a number key not numeric": "cycle=abc",
		"an unknown severity":      "severity=DEBUG",
	}
	for name, expr := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := Parse(expr, testCatalog())
			if !errors.Is(err, ErrUsage) {
				t.Fatalf("Parse(%q) error = %v, want ErrUsage", expr, err)
			}
		})
	}
}

func TestRegisteredCatalog_IsTheSignalCenterVocabulary(t *testing.T) {
	for _, m := range []signalcenter.Module{signalcenter.ModuleAudit, signalcenter.ModuleBuild, signalcenter.ModuleLoop, signalcenter.ModuleShip} {
		signalcenter.RegisterCode(m, signalcenter.Code(strings.ToUpper(string(m))+"_FILTER_PROBE"), "a probe code of the filter catalog test")
	}
	cat := RegisteredCatalog()
	if len(cat.Kinds) != len(signalcenter.Kinds()) || len(cat.Modules) != len(signalcenter.Modules()) {
		t.Fatalf("catalog has %d kinds and %d modules, want %d and %d",
			len(cat.Kinds), len(cat.Modules), len(signalcenter.Kinds()), len(signalcenter.Modules()))
	}
	registered := 0
	for _, docs := range signalcenter.RegisteredCodes() {
		registered += len(docs)
	}
	if len(cat.Codes) != registered {
		t.Fatalf("catalog has %d codes, want %d", len(cat.Codes), registered)
	}
	for range 20 {
		codes := RegisteredCatalog().Codes
		for i := 1; i < len(codes); i++ {
			if codes[i-1] >= codes[i] {
				t.Fatalf("codes not sorted at %d: %s >= %s", i, codes[i-1], codes[i])
			}
		}
	}
	_, warnings, err := Parse("kind=signalcenter.* module=signalcenter code=SIGNALCENTER_SINK_DROPPED", cat)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("registered vocabulary refused: err=%v warnings=%v", err, warnings)
	}
}

func TestGlobMatch_AStarMatchesAnyRunOfCharacters(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"ship.landed", "ship.landed", true},
		{"ship", "ship.landed", false},
		{"ship.*", "ship.landed", true},
		{"*.exit", "loop.exit", true},
		{"*", "", true},
		{"x*", "ship", false},
		{"a*a", "a", false},
		{"a*a", "aa", true},
		{"l*p*t", "loop.exit", true},
		{"l*z*t", "loop.exit", false},
		{"*o*o*", "loop", true},
		{"*o*o*o*", "loop", false},
		{"l*p", "loop.exit", false},
	}
	for _, tc := range cases {
		if got := globMatch(tc.pattern, tc.s); got != tc.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

func TestParse_AnUnknownCodeWarningNamesTheKeyAndTheValue(t *testing.T) {
	_, warnings, err := Parse("code=SHIP_GATE_RED,NEWER_BUILD_CODE,NEWER_*", testCatalog())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Warning{{Key: "code", Value: "NEWER_BUILD_CODE"}, {Key: "code", Value: "NEWER_*"}}
	if len(warnings) != len(want) || warnings[0] != want[0] || warnings[1] != want[1] {
		t.Fatalf("warnings = %#v, want %#v", warnings, want)
	}
	if got := warnings[0].String(); got != `code "NEWER_BUILD_CODE" matches no registered code` {
		t.Fatalf("String() = %q", got)
	}
}
