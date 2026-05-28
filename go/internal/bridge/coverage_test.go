package bridge

import (
	"context"
	"strings"
	"testing"
)

// coverage_test.go — exhaustive coverage of pure helpers, the driver
// registry, and the env/flag glue. Pairs with exec_integration_test.go
// (real exec/tmux) to drive internal/bridge to 100%.

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	fn()
}

// stubDriver is a registry test double.
type stubDriver struct{ name string }

func (s stubDriver) Name() string                                     { return s.name }
func (stubDriver) Launch(context.Context, *Config, Deps) (int, error) { return 0, nil }

// registerBuiltins re-registers the seven production drivers after a
// ResetDriversForTesting, so the global registry is restored for other tests.
// WS-F added ollama-tmux as the seventh peer.
func registerBuiltins() {
	Register(claudePDriver{})
	Register(codexDriver{})
	Register(agyDriver{})
	Register(claudeTmuxDriver{})
	Register(codexTmuxDriver{})
	Register(agyTmuxDriver{})
	Register(ollamaTmuxDriver{})
}

func TestDriverRegistry(t *testing.T) {
	// DriverNames over the real (init-registered) set. Strict count: an
	// accidental init() registration (e.g. a test stub forgetting deferred
	// cleanup) gets caught immediately; a deliberate addition explicitly
	// bumps this constant.
	names := DriverNames()
	if len(names) != 7 {
		t.Fatalf("DriverNames = %v (len %d), want exactly 7 builtins", names, len(names))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("DriverNames not sorted: %v", names)
		}
	}

	// Reset → empty → register a stub → lookup hit, then restore builtins.
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

	mustPanic(t, func() { Register(nil) })                  // nil driver
	mustPanic(t, func() { Register(stubDriver{""}) })       // empty name
	mustPanic(t, func() { Register(stubDriver{"stub-x"}) }) // duplicate
}

func TestAllDigits(t *testing.T) {
	for in, want := range map[string]bool{"": false, "0": true, "60": true, "1a": false, "x": false} {
		if got := allDigits(in); got != want {
			t.Fatalf("allDigits(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLastLines(t *testing.T) {
	if got := lastLines("a\nb\nc", 10); got != "a\nb\nc" {
		t.Fatalf("lastLines short = %q", got)
	}
	if got := lastLines("a\nb\nc\nd\ne", 2); got != "d\ne" {
		t.Fatalf("lastLines long = %q, want d\\ne", got)
	}
}

func TestTruncate64(t *testing.T) {
	if got := truncate64("short"); got != "short" {
		t.Fatalf("truncate64 short = %q", got)
	}
	long := strings.Repeat("x", 70)
	if got := truncate64(long); len(got) != 64 {
		t.Fatalf("truncate64 long len = %d, want 64", len(got))
	}
}

func TestParseExtendSecs(t *testing.T) {
	for in, want := range map[string]int{"extend:60": 60, "extend:0": 0, "extend:x": 0, "nope": 0, "extend:": 0} {
		if got := parseExtendSecs(in); got != want {
			t.Fatalf("parseExtendSecs(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestMapCodexModelAndName(t *testing.T) {
	for in, want := range map[string]string{"haiku": "gpt-5.4-mini", "sonnet": "gpt-5.4", "opus": "gpt-5.5", "gpt-x": "gpt-x", "weird": "weird"} {
		if got := mapCodexModel(in); got != want {
			t.Fatalf("mapCodexModel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]bool{"gpt-5": true, "o1-x": true, "o3": true, "codex-z": true, "claude": false, "": false} {
		if got := isCodexModelName(in); got != want {
			t.Fatalf("isCodexModelName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDecideAutoRespond_ExtraBranches(t *testing.T) {
	// extend_timeout with non-numeric keys → escalate
	prompts := []ManifestPrompt{
		{Name: "skipEmpty", Regex: "", Policy: "escalate"}, // empty regex → skipped
		{Name: "badExtend", Regex: "slow", ResponseKeys: "x", Policy: "extend_timeout"},
	}
	if a, rc := decideAutoRespond("slow op", prompts, map[string]int{}); a != "escalate:badExtend" || rc != 85 {
		t.Fatalf("extend_timeout bad keys = (%q,%d), want escalate", a, rc)
	}
	// extend_timeout with numeric keys → extend
	prompts = []ManifestPrompt{{Name: "ext", Regex: "slow", ResponseKeys: "90", Policy: "extend_timeout"}}
	if a, rc := decideAutoRespond("slow", prompts, map[string]int{}); a != "extend:90" || rc != 2 {
		t.Fatalf("extend_timeout good = (%q,%d), want extend:90,2", a, rc)
	}
}

func TestDefaultChallengeToken(t *testing.T) {
	tok, err := defaultChallengeToken()
	if err != nil {
		t.Fatalf("defaultChallengeToken err: %v", err)
	}
	if len(tok) != 16 { // 8 bytes hex
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
	names := ManifestNames()
	want := map[string]bool{"claude-p": true, "claude-tmux": true, "codex": true, "codex-tmux": true, "agy": true, "agy-tmux": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("ManifestNames missing %v (got %v)", want, names)
	}
}

func TestDriverEnv_MergesReqEnv(t *testing.T) {
	env := driverEnv(Deps{Env: map[string]string{"FOO_COV": "bar"}})
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
