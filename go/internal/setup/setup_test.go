package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// --- helpers ---

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureRepo lays down a temp project with .evolve/profiles + llm_config.
func fixtureRepo(t *testing.T) (project, evolveDir string) {
	t.Helper()
	project = t.TempDir()
	evolveDir = filepath.Join(project, ".evolve")
	profiles := filepath.Join(evolveDir, "profiles")
	// builder: envelope balanced..deep, cross-family with auditor, claude+agy
	writeFile(t, filepath.Join(profiles, "builder.json"), `{
	  "cli": "agy-tmux", "model_tier_default": "sonnet",
	  "model_tier_envelope": {"min":"balanced","default":"balanced","max":"deep"},
	  "cross_family_with": "auditor", "allowed_clis": ["claude","agy"]
	}`)
	// auditor: envelope deep..deep, cross-family with builder, all
	writeFile(t, filepath.Join(profiles, "auditor.json"), `{
	  "cli": "codex-tmux", "model_tier_default": "sonnet",
	  "model_tier_envelope": {"min":"deep","default":"deep","max":"deep"},
	  "cross_family_with": "builder", "allowed_clis": ["all"]
	}`)
	// scout: envelope balanced..deep
	writeFile(t, filepath.Join(profiles, "scout.json"), `{
	  "cli": "claude-tmux", "model_tier_default": "sonnet",
	  "model_tier_envelope": {"min":"balanced","default":"balanced","max":"deep"}
	}`)
	return project, evolveDir
}

func fakeDoctor(ctx context.Context) bridge.DoctorReport {
	return bridge.DoctorReport{
		ScannedAt: "2026-01-01T00:00:00Z",
		Results: []bridge.DoctorResult{
			{CLI: "claude-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/claude"}, Auth: bridge.AuthInfo{Configured: true, Source: "file:credentials.json"}, Verdict: "ready"},
			{CLI: "claude-p", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/claude"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"}, // dup family → grouped out
			{CLI: "codex-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/codex"}, Auth: bridge.AuthInfo{Configured: true, SubscriptionType: "chatgpt-account"}, Verdict: "ready"},
			{CLI: "gemini", Binary: bridge.BinaryInfo{Present: false}, Auth: bridge.AuthInfo{}, Verdict: "blocked"},
		},
	}
}

// --- tier + family normalization ---

func TestTierRank(t *testing.T) {
	cases := map[string]int{
		"fast": 1, "haiku": 1, "balanced": 2, "sonnet": 2, "deep": 3, "opus": 3,
		"claude-opus-4-7": 3, "claude-3-5-sonnet": 2, "claude-haiku-4-5": 1, "gpt-5": 0, "": 0,
	}
	for in, want := range cases {
		if got := tierRank(in); got != want {
			t.Errorf("tierRank(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestCLIFamily(t *testing.T) {
	cases := map[string]string{
		"claude": "anthropic", "claude-tmux": "anthropic", "claude-p": "anthropic",
		"codex": "openai", "codex-tmux": "openai",
		"gemini": "google", "agy": "google", "agy-tmux": "google",
	}
	for in, want := range cases {
		if got := cliFamily(in); got != want {
			t.Errorf("cliFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAuthMode(t *testing.T) {
	envWith := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	configured := bridge.AuthInfo{Configured: true}
	if got := authMode("claude", configured, envWith(map[string]string{"ANTHROPIC_BASE_URL": "http://x"})); got != "CUSTOM_PROXY" {
		t.Errorf("proxy precedence: got %q", got)
	}
	if got := authMode("claude", configured, envWith(map[string]string{"ANTHROPIC_API_KEY": "sk-x"})); got != "API_KEY" {
		t.Errorf("api-key precedence: got %q", got)
	}
	if got := authMode("claude", configured, envWith(nil)); got != "SUBSCRIPTION_OAUTH" {
		t.Errorf("oauth: got %q", got)
	}
	if got := authMode("claude", bridge.AuthInfo{}, envWith(nil)); got != "MISCONFIGURED" {
		t.Errorf("misconfigured: got %q", got)
	}
	if got := authMode("codex", configured, envWith(nil)); got != "SUBSCRIPTION" {
		t.Errorf("codex subscription: got %q", got)
	}
}

func TestTierModelsFor(t *testing.T) {
	// agy has no model selector → all tiers map to gemini-3.5-flash.
	agy := tierModelsFor("agy")
	for _, tier := range []string{"fast", "balanced", "deep"} {
		if agy[tier] != "gemini-3.5-flash" {
			t.Errorf("agy[%s] = %q, want gemini-3.5-flash", tier, agy[tier])
		}
	}
	// codex maps to its native GPT tiers.
	codex := tierModelsFor("codex")
	want := map[string]string{"fast": "gpt-5.4-mini", "balanced": "gpt-5.4", "deep": "gpt-5.5"}
	for tier, m := range want {
		if codex[tier] != m {
			t.Errorf("codex[%s] = %q, want %q", tier, codex[tier], m)
		}
	}
	// claude has empty tier_aliases → identity (the keys ARE Claude's selectors).
	claude := tierModelsFor("claude")
	if claude["fast"] != "haiku" || claude["balanced"] != "sonnet" || claude["deep"] != "opus" {
		t.Errorf("claude identity broken: %+v", claude)
	}
}

// --- Detect ---

func TestDetect(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	writeFile(t, filepath.Join(evolveDir, "llm_config.json"), `{
	  "schema_version": 2,
	  "phases": {
	    "builder": {"cli":"claude","tier":"balanced","model":"sonnet"},
	    "auditor": {"cli":"claude","tier":"deep","model":"opus"}
	  },
	  "_fallback": {"cli":"claude","tier":"balanced"}
	}`)

	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		Env:         func(string) string { return "" },
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:      fakeDoctor,
		CapTier:     func(base string) string { return map[string]string{"claude": "full", "codex": "delegated"}[base] },
	})

	// CLIs: claude+codex+gemini (claude-p deduped into claude).
	if len(rep.CLIs) != 3 {
		t.Fatalf("want 3 CLI families, got %d: %+v", len(rep.CLIs), rep.CLIs)
	}
	byCLI := map[string]CLIStatus{}
	for _, c := range rep.CLIs {
		byCLI[c.CLI] = c
	}
	if c := byCLI["claude"]; c.AuthMode != "SUBSCRIPTION_OAUTH" || c.CapabilityTier != "full" || !c.BinaryPresent {
		t.Errorf("claude: %+v", c)
	}
	if c := byCLI["codex"]; c.SubscriptionType != "chatgpt-account" || c.CapabilityTier != "delegated" {
		t.Errorf("codex: %+v", c)
	}
	if c := byCLI["gemini"]; c.BinaryPresent || c.CapabilityTier != "n/a" {
		t.Errorf("gemini should be absent/n_a: %+v", c)
	}

	// Phases: builder resolves from llm_config, carries envelope + cross-family.
	var builder PhaseStatus
	for _, p := range rep.Phases {
		if p.Role == "builder" {
			builder = p
		}
	}
	if builder.CurrentCLI != "claude" || builder.CurrentModel != "sonnet" || builder.Source != "llm_config" {
		t.Errorf("builder routing: %+v", builder)
	}
	if builder.Envelope.Max != "deep" || builder.CrossFamilyWith != "auditor" {
		t.Errorf("builder constraints: %+v", builder)
	}
	if rep.SetupCompletedAt != "" {
		t.Errorf("fresh repo should have no setup marker, got %q", rep.SetupCompletedAt)
	}
}

// --- Validate ---

func TestValidate(t *testing.T) {
	_, evolveDir := fixtureRepo(t)
	cfgPath := filepath.Join(evolveDir, "cfg.json")

	t.Run("clean all-claude (cross-family WARN only, exit OK)", func(t *testing.T) {
		writeFile(t, cfgPath, `{"phases":{
		  "builder":{"cli":"claude","tier":"balanced"},
		  "auditor":{"cli":"claude","tier":"deep"}
		}}`)
		rep, err := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir})
		if err != nil {
			t.Fatal(err)
		}
		if !rep.OK {
			t.Fatalf("all-claude within envelope should be OK (cross-family is a warn): %+v", rep.Violations)
		}
		var sawWarn bool
		for _, v := range rep.Violations {
			if v.Kind == "cross_family" && v.Severity == "warn" {
				sawWarn = true
			}
		}
		if !sawWarn {
			t.Errorf("expected a cross_family warn, got %+v", rep.Violations)
		}
	})

	t.Run("cross-family error under --strict", func(t *testing.T) {
		writeFile(t, cfgPath, `{"phases":{
		  "builder":{"cli":"claude","tier":"balanced"},
		  "auditor":{"cli":"claude","tier":"deep"}
		}}`)
		rep, _ := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir, Strict: true})
		if rep.OK {
			t.Errorf("strict mode should fail same-family: %+v", rep.Violations)
		}
	})

	t.Run("envelope violation is an error", func(t *testing.T) {
		// auditor envelope is deep..deep; balanced is below min → error.
		writeFile(t, cfgPath, `{"phases":{"auditor":{"cli":"codex","tier":"balanced"}}}`)
		rep, _ := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir})
		if rep.OK {
			t.Fatalf("below-envelope tier should fail: %+v", rep.Violations)
		}
		if rep.Violations[0].Kind != "envelope" {
			t.Errorf("want envelope violation, got %+v", rep.Violations)
		}
	})

	t.Run("allowed_clis violation is an error", func(t *testing.T) {
		// builder allows [claude,agy]; codex is not allowed → error.
		writeFile(t, cfgPath, `{"phases":{"builder":{"cli":"codex","tier":"balanced"}}}`)
		rep, _ := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir})
		if rep.OK {
			t.Fatalf("disallowed cli should fail: %+v", rep.Violations)
		}
		if rep.Violations[0].Kind != "allowed_cli" {
			t.Errorf("want allowed_cli violation, got %+v", rep.Violations)
		}
	})

	t.Run("cross-family OK when families differ", func(t *testing.T) {
		writeFile(t, cfgPath, `{"phases":{
		  "builder":{"cli":"agy","tier":"balanced"},
		  "auditor":{"cli":"codex","tier":"deep"}
		}}`)
		rep, _ := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir})
		if !rep.OK {
			t.Fatalf("agy/codex differ: should be OK: %+v", rep.Violations)
		}
		for _, v := range rep.Violations {
			if v.Kind == "cross_family" {
				t.Errorf("no cross_family violation expected: %+v", v)
			}
		}
	})

	t.Run("missing config errors", func(t *testing.T) {
		if _, err := Validate(ValidateOptions{ConfigPath: filepath.Join(evolveDir, "nope.json"), EvolveDir: evolveDir}); err == nil {
			t.Error("missing config should error")
		}
	})
}

// --- Complete (lossless merge) ---

func TestCompletePreservesUnmodeledKeys(t *testing.T) {
	evolveDir := t.TempDir()
	// Pre-existing state.json with a key NOT in core.State (the real-world
	// expected_ship_sha) — Complete must preserve it.
	writeFile(t, filepath.Join(evolveDir, "state.json"), `{
	  "lastCycleNumber": 7,
	  "expected_ship_sha": "abc123",
	  "version": 1
	}`)

	stamp, err := Complete(CompleteOptions{EvolveDir: evolveDir, Now: func() time.Time { return time.Unix(1700000000, 0).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	if stamp == "" {
		t.Fatal("empty stamp")
	}
	raw, _ := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["expected_ship_sha"]; !ok {
		t.Error("Complete dropped expected_ship_sha (lossy write!)")
	}
	if _, ok := got["lastCycleNumber"]; !ok {
		t.Error("Complete dropped lastCycleNumber")
	}
	if _, ok := got["setupCompletedAt"]; !ok {
		t.Error("Complete did not stamp setupCompletedAt")
	}

	// Idempotent: re-run succeeds and marker is read back.
	if _, err := Complete(CompleteOptions{EvolveDir: evolveDir}); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	at, ver := readStateMarker(evolveDir)
	if at == "" || ver != Version {
		t.Errorf("marker readback: at=%q ver=%d", at, ver)
	}
}

func TestCompleteFreshStateFile(t *testing.T) {
	evolveDir := t.TempDir()
	if _, err := Complete(CompleteOptions{EvolveDir: evolveDir}); err != nil {
		t.Fatalf("fresh complete: %v", err)
	}
	if at, _ := readStateMarker(evolveDir); at == "" {
		t.Error("fresh complete should create state.json with marker")
	}
}

func TestCompleteRefusesMalformedState(t *testing.T) {
	evolveDir := t.TempDir()
	writeFile(t, filepath.Join(evolveDir, "state.json"), `{not json`)
	if _, err := Complete(CompleteOptions{EvolveDir: evolveDir}); err == nil {
		t.Error("malformed state.json should error rather than clobber")
	}
}
