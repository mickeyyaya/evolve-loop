package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		// Unknown CLI falls through to the baseCLI passthrough (default branch).
		"mystery":      "mystery",
		"mystery-tmux": "mystery",
	}
	for in, want := range cases {
		if got := cliFamily(in); got != want {
			t.Errorf("cliFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCapManifest pins the agy→antigravity special-case and the identity
// passthrough for every other CLI (the manifest-stem mapping).
func TestCapManifest(t *testing.T) {
	if got := capManifest("agy"); got != "antigravity" {
		t.Errorf("capManifest(agy) = %q, want antigravity", got)
	}
	for _, base := range []string{"claude", "codex", "gemini"} {
		if got := capManifest(base); got != base {
			t.Errorf("capManifest(%q) = %q, want identity", base, got)
		}
	}
}

// TestEffTier pins the tier > model_tier > model precedence ladder, including
// the two fallback rungs the existing suite never exercised.
func TestEffTier(t *testing.T) {
	cases := []struct {
		name string
		p    cfgPhase
		want string
	}{
		{"tier wins", cfgPhase{Tier: "deep", ModelTier: "balanced", Model: "opus"}, "deep"},
		{"model_tier fallback", cfgPhase{ModelTier: "balanced", Model: "sonnet"}, "balanced"},
		{"model last resort", cfgPhase{Model: "claude-opus-4-7"}, "claude-opus-4-7"},
		{"all empty", cfgPhase{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.p.effTier(); got != c.want {
				t.Errorf("effTier() = %q, want %q", got, c.want)
			}
		})
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

	// Phases: builder resolves from its PROFILE (Step 9 removed llm_config, so
	// the llm_config.json written above is ignored), carrying envelope +
	// cross-family. The profile is cli=agy-tmux, model_tier_default=sonnet.
	var builder PhaseStatus
	for _, p := range rep.Phases {
		if p.Role == "builder" {
			builder = p
		}
	}
	if builder.CurrentCLI != "agy-tmux" || builder.CurrentTier != "sonnet" || builder.Source != "profile" {
		t.Errorf("builder routing: %+v", builder)
	}
	if builder.Envelope.Max != "deep" || builder.CrossFamilyWith != "auditor" {
		t.Errorf("builder constraints: %+v", builder)
	}
	if rep.SetupCompletedAt != "" {
		t.Errorf("fresh repo should have no setup marker, got %q", rep.SetupCompletedAt)
	}
}

// TestCapTierFromManifest exercises the default CapTier seam directly:
// empty AdaptersDir → unknown; a manifest declaring both native capabilities
// → full; one missing → delegated; a missing manifest file → Inspect defaults
// both to true → full.
func TestCapTierFromManifest(t *testing.T) {
	if got := capTierFromManifest("", "claude"); got != "unknown" {
		t.Errorf("empty adaptersDir: got %q, want unknown", got)
	}

	dir := t.TempDir()
	// Full: both native capabilities present (under the .supports block, the
	// shape capability.Inspect reads).
	writeFile(t, filepath.Join(dir, "claude.capabilities.json"),
		`{"supports": {"budget_cap_native": true, "permission_scoping": true}}`)
	if got := capTierFromManifest(dir, "claude"); got != "full" {
		t.Errorf("both-native manifest: got %q, want full", got)
	}

	// Delegated: one capability false flips the verdict.
	writeFile(t, filepath.Join(dir, "codex.capabilities.json"),
		`{"supports": {"budget_cap_native": false, "permission_scoping": true}}`)
	if got := capTierFromManifest(dir, "codex"); got != "delegated" {
		t.Errorf("missing-budget manifest: got %q, want delegated", got)
	}

	// agy resolves via the antigravity manifest stem (capManifest mapping).
	writeFile(t, filepath.Join(dir, "antigravity.capabilities.json"),
		`{"supports": {"budget_cap_native": true, "permission_scoping": true}}`)
	if got := capTierFromManifest(dir, "agy"); got != "full" {
		t.Errorf("agy via antigravity manifest: got %q, want full", got)
	}

	// Absent manifest → Inspect defaults both to true → full (not unknown).
	if got := capTierFromManifest(dir, "gemini"); got != "full" {
		t.Errorf("absent manifest: got %q, want full (Inspect defaults true)", got)
	}

	// Inspect I/O error (non-ENOENT) → unknown: make the manifest path a
	// directory so os.ReadFile fails with EISDIR.
	if err := os.MkdirAll(filepath.Join(dir, "perl.capabilities.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := capTierFromManifest(dir, "perl"); got != "unknown" {
		t.Errorf("manifest-read error: got %q, want unknown", got)
	}
}

// TestReadProfileConstraints_MalformedJSON pins that a profile that exists but
// is not valid JSON reports ok=false (the unmarshal-error branch) rather than
// returning partial constraints.
func TestReadProfileConstraints_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "scout.json"), `{not valid json`)
	_, _, _, ok := readProfileConstraints(dir, "scout")
	if ok {
		t.Error("malformed profile JSON should report ok=false")
	}
	// Missing file is also ok=false (the ReadFile-error branch).
	if _, _, _, ok := readProfileConstraints(dir, "absent"); ok {
		t.Error("missing profile should report ok=false")
	}
}

// TestDetect_DefaultSeams drives Detect with NO seam overrides except Doctor +
// CapTier (so no live binaries are probed): exercises the nil Env/Now default
// branches, a profile-less role (resolvellm error / no-constraints path), and
// the SetupCompletedAt populated readback.
func TestDetect_DefaultSeams(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	// Stamp a setup marker so the SetupCompletedAt/SetupVersion readback fires.
	writeFile(t, filepath.Join(evolveDir, "state.json"),
		`{"setupCompletedAt":"2025-12-31T00:00:00Z","setupVersion":1}`)

	// Env + Now left nil → defaults (os.Getenv, time.Now) are used.
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		Doctor:      fakeDoctor,
		CapTier:     func(string) string { return "full" },
	})

	if rep.SetupCompletedAt != "2025-12-31T00:00:00Z" || rep.SetupVersion != 1 {
		t.Errorf("setup marker readback: at=%q ver=%d", rep.SetupCompletedAt, rep.SetupVersion)
	}
	if rep.ScannedAt == "" {
		t.Error("default Now seam should still stamp ScannedAt")
	}
	// "intent" has no profile file → no constraints; should still appear.
	var sawIntent bool
	for _, p := range rep.Phases {
		if p.Role == "intent" {
			sawIntent = true
			if len(p.AllowedCLIs) != 0 {
				t.Errorf("profile-less role should carry no allowed_clis: %+v", p)
			}
		}
	}
	if !sawIntent {
		t.Error("intent role missing from phases")
	}
}

// TestDetect_NilDoctorAndCapTierSeams drives Detect with Doctor AND CapTier
// left nil so the default closures (real bridge.Doctor + capTierFromManifest)
// execute. It is offline/deterministic: bridge.Doctor probes the local
// environment without network or repo state, and we assert only structural
// invariants (one CLIStatus per detected base family, every Role present) so
// the result is stable regardless of which CLIs the host has installed.
func TestDetect_NilDoctorAndCapTierSeams(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		AdaptersDir: t.TempDir(), // empty → capTierFromManifest returns full/unknown deterministically
		// Doctor + CapTier nil → default seams run.
	})
	if rep.ScannedAt == "" {
		t.Error("default Now seam should stamp ScannedAt")
	}
	if len(rep.Phases) != len(Roles) {
		t.Errorf("phases len = %d, want %d (one per Role)", len(rep.Phases), len(Roles))
	}
	// Base-family dedup invariant holds regardless of host CLIs.
	seen := map[string]bool{}
	for _, c := range rep.CLIs {
		if seen[c.CLI] {
			t.Errorf("duplicate CLI family in report: %q", c.CLI)
		}
		seen[c.CLI] = true
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

	t.Run("malformed config JSON errors", func(t *testing.T) {
		writeFile(t, cfgPath, `{"phases": {not json`)
		if _, err := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir}); err == nil {
			t.Error("unparseable config should error")
		}
	})

	t.Run("role without a profile is skipped, not an error", func(t *testing.T) {
		// "intent" has no profile in fixtureRepo → readProfileConstraints
		// reports !ok → the role is skipped (no violation, OK stays true).
		writeFile(t, cfgPath, `{"phases":{"intent":{"cli":"codex","tier":"fast"}}}`)
		rep, err := Validate(ValidateOptions{ConfigPath: cfgPath, EvolveDir: evolveDir})
		if err != nil {
			t.Fatal(err)
		}
		if !rep.OK || len(rep.Violations) != 0 {
			t.Errorf("profile-less role should pass with no violations: %+v", rep)
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

// TestCompleteMkdirFails pins the MkdirAll error branch: when EvolveDir's
// parent is a regular file, MkdirAll cannot create the directory, so Complete
// returns an error instead of silently proceeding.
func TestCompleteMkdirFails(t *testing.T) {
	base := t.TempDir()
	fileAsParent := filepath.Join(base, "iam-a-file")
	writeFile(t, fileAsParent, "x")
	// EvolveDir nests under a regular file → MkdirAll fails (ENOTDIR).
	_, err := Complete(CompleteOptions{EvolveDir: filepath.Join(fileAsParent, "evolve")})
	if err == nil {
		t.Error("Complete under a file-path parent should fail at MkdirAll")
	}
	if err != nil && !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("error should name the mkdir step, got %v", err)
	}
}
