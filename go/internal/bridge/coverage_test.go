package bridge

import (
	"context"
	"strings"
	"testing"
)

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	fn()
}

type stubDriver struct{ name string }

func (s stubDriver) Name() string                                     { return s.name }
func (stubDriver) Launch(context.Context, *Config, Deps) (int, error) { return 0, nil }

func registerBuiltins() {
	Register(claudePDriver{})
	Register(codexDriver{})
	Register(agyDriver{})
	Register(claudeTmuxDriver{})
	Register(codexTmuxDriver{})
	Register(agyTmuxDriver{target: "agy-tmux"})
	Register(agyTmuxDriver{target: "agy-claude-tmux"})
	Register(ollamaTmuxDriver{})
}

func TestDriverRegistry(t *testing.T) {
	names := DriverNames()
	if len(names) != 8 {
		t.Fatalf("DriverNames = %v (len %d), want exactly 8 builtins", names, len(names))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("DriverNames not sorted: %v", names)
		}
	}

	ResetDriversForTesting()
	defer func() { ResetDriversForTesting(); registerBuiltins() }()
	if len(DriverNames()) != 0 {
		t.Fatal("ResetDriversForTesting should clear the registry")
	}
	Register(stubDriver{"stub-x"})
	if _, ok := LookupDriver("stub-x"); !ok {
		t.Fatal("LookupDriver should find the registered stub")
	}
	if _, ok := LookupDriver("absent"); ok {
		t.Fatal("LookupDriver should miss an unregistered name")
	}

	mustPanic(t, func() { Register(nil) })
	mustPanic(t, func() { Register(stubDriver{""}) })
	mustPanic(t, func() { Register(stubDriver{"stub-x"}) })
}

func TestAllDigits(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]bool{"": false, "0": true, "60": true, "1a": false, "x": false} {
		if got := allDigits(in); got != want {
			t.Fatalf("allDigits(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLastLines(t *testing.T) {
	t.Parallel()
	if got := lastLines("a\nb\nc", 10); got != "a\nb\nc" {
		t.Fatalf("lastLines short = %q", got)
	}
	if got := lastLines("a\nb\nc\nd\ne", 2); got != "d\ne" {
		t.Fatalf("lastLines long = %q, want d\\ne", got)
	}
}

func TestTruncate64(t *testing.T) {
	t.Parallel()
	if got := truncate64("short"); got != "short" {
		t.Fatalf("truncate64 short = %q", got)
	}
	long := strings.Repeat("x", 70)
	if got := truncate64(long); len(got) != 64 {
		t.Fatalf("truncate64 long len = %d, want 64", len(got))
	}
}

func TestParseExtendSecs(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"extend:60": 60, "extend:0": 0, "extend:x": 0, "nope": 0, "extend:": 0} {
		if got := parseExtendSecs(in); got != want {
			t.Fatalf("parseExtendSecs(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestMapCodexModelAndName(t *testing.T) {
	t.Parallel()
	m := codexFamilyManifest(t)
	for in, want := range map[string]string{"haiku": m.ModelTierMap["fast"], "sonnet": m.ModelTierMap["balanced"], "opus": m.ModelTierMap["deep"], "gpt-x": "gpt-x", "weird": "weird"} {
		if got := resolveTierModel(m, in); got != want {
			t.Fatalf("resolveTierModel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]bool{"gpt-5": true, "o1-x": true, "o3": true, "codex-z": true, "claude": false, "": false} {
		if got := isCodexModelName(in); got != want {
			t.Fatalf("isCodexModelName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDecideAutoRespond_ExtraBranches(t *testing.T) {
	t.Parallel()
	prompts := []ManifestPrompt{
		{Name: "skipEmpty", Regex: "", Policy: "escalate"},
		{Name: "badExtend", Regex: "slow", ResponseKeys: "x", Policy: "extend_timeout"},
	}
	if a, rc := decideAutoRespond("slow op", prompts, map[string]int{}, false); a != "escalate:badExtend" || rc != 85 {
		t.Fatalf("extend_timeout bad keys = (%q,%d), want escalate", a, rc)
	}
	prompts = []ManifestPrompt{{Name: "ext", Regex: "slow", ResponseKeys: "90", Policy: "extend_timeout"}}
	if a, rc := decideAutoRespond("slow", prompts, map[string]int{}, false); a != "extend:90" || rc != 2 {
		t.Fatalf("extend_timeout good = (%q,%d), want extend:90,2", a, rc)
	}
}

func TestDefaultChallengeToken(t *testing.T) {
	t.Parallel()
	tok, err := defaultChallengeToken()
	if err != nil {
		t.Fatalf("defaultChallengeToken err: %v", err)
	}
	if len(tok) != 16 {
		t.Fatalf("token len = %d, want 16", len(tok))
	}
}

func TestLookupEnv_NilSeamFallsBackToOS(t *testing.T) {
	t.Setenv("BRIDGE_COV_PROBE", "yes")
	if v, ok := lookupEnv(Deps{}, "BRIDGE_COV_PROBE"); !ok || v != "yes" {
		t.Fatalf("lookupEnv nil-seam fallback = (%q,%v), want (yes,true)", v, ok)
	}
}

func TestManifestNames_Embedded(t *testing.T) {
	t.Parallel()
	names := ManifestNames()
	want := map[string]bool{"claude-p": true, "claude-tmux": true, "codex": true, "codex-tmux": true, "agy": true, "agy-tmux": true, "agy-claude-tmux": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("ManifestNames missing %v (got %v)", want, names)
	}
}

func TestDriverEnv_MergesReqEnv(t *testing.T) {
	t.Parallel()
	env := driverEnv(Deps{Env: map[string]string{"FOO_COV": "bar"}}, nil)
	found := false
	for _, kv := range env {
		if kv == "FOO_COV=bar" {
			found = true
		}
	}
	if !found {
		t.Fatal("driverEnv should append Deps.Env entries")
	}
}
