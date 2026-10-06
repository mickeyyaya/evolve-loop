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

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRepo(t *testing.T) (project, evolveDir string) {
	t.Helper()
	project = t.TempDir()
	evolveDir = filepath.Join(project, ".evolve")
	profiles := filepath.Join(evolveDir, "profiles")
	writeFile(t, filepath.Join(profiles, "builder.json"), `{
	  "cli": "agy-tmux", "model_tier_default": "sonnet",
	  "model_tier_envelope": {"min":"balanced","default":"balanced","max":"deep"},
	  "cross_family_with": "auditor", "allowed_clis": ["claude","agy"]
	}`)
	writeFile(t, filepath.Join(profiles, "auditor.json"), `{
	  "cli": "codex-tmux", "model_tier_default": "sonnet",
	  "model_tier_envelope": {"min":"deep","default":"deep","max":"deep"},
	  "cross_family_with": "builder", "allowed_clis": ["all"]
	}`)
	writeFile(t, filepath.Join(profiles, "scout.json"), `{
	  "cli": "claude-tmux", "model_tier_default": "sonnet",
	  "model_tier_envelope": {"min":"balanced","default":"balanced","max":"deep"}
	}`)
	return project, evolveDir
}

func fakeDoctor(ctx context.Context) bridge.DoctorReport {
	sameFamilyAsClaudeTmux := bridge.DoctorResult{CLI: "claude-p", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/claude"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"}
	return bridge.DoctorReport{
		ScannedAt: "2026-01-01T00:00:00Z",
		Results: []bridge.DoctorResult{
			{CLI: "claude-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/claude"}, Auth: bridge.AuthInfo{Configured: true, Source: "file:credentials.json"}, Verdict: "ready"},
			sameFamilyAsClaudeTmux,
			{CLI: "codex-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/codex"}, Auth: bridge.AuthInfo{Configured: true, SubscriptionType: "chatgpt-account"}, Verdict: "ready"},
			{CLI: "gemini", Binary: bridge.BinaryInfo{Present: false}, Auth: bridge.AuthInfo{}, Verdict: "blocked"},
		},
	}
}

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
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
	agy := tierModelsFor("agy")
	wantAgy := map[string]string{
		"fast":     "Gemini 3.8 Flash (Low)",
		"balanced": "Gemini 3.8 Flash (High)",
		"deep":     "Gemini 3.1 Pro (High)",
	}
	for tier, m := range wantAgy {
		if agy[tier] != m {
			t.Errorf("agy[%s] = %q, want %q", tier, agy[tier], m)
		}
	}
	codex := tierModelsFor("codex")
	fam, err := bridge.LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(codex-tmux): %v", err)
	}
	want := map[string]string{"fast": fam.ModelTierMap["fast"], "balanced": fam.ModelTierMap["balanced"], "deep": fam.ModelTierMap["deep"]}
	for tier, m := range want {
		if codex[tier] != m {
			t.Errorf("codex[%s] = %q, want %q", tier, codex[tier], m)
		}
	}
	claude := tierModelsFor("claude")
	if claude["fast"] != "haiku" || claude["balanced"] != "sonnet" || claude["deep"] != "opus" {
		t.Errorf("claude identity broken: %+v", claude)
	}
}

func TestDetect(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	legacyLLMConfigDetectIgnores := filepath.Join(evolveDir, "llm_config.json")
	writeFile(t, legacyLLMConfigDetectIgnores, `{
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

func detectWithPolicy(t *testing.T, policyBody string) DetectReport {
	t.Helper()
	project, evolveDir := fixtureRepo(t)
	if policyBody != "" {
		writeFile(t, filepath.Join(evolveDir, "policy.json"), policyBody)
	}
	return Detect(context.Background(), DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		Env:         func(string) string { return "" },
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:      fakeDoctor,
		CapTier:     func(base string) string { return map[string]string{"claude": "full", "codex": "delegated"}[base] },
	})
}

func phaseByRole(rep DetectReport, role string) PhaseStatus {
	for _, p := range rep.Phases {
		if p.Role == role {
			return p
		}
	}
	return PhaseStatus{}
}

func TestDetect_PolicyPinOverlay(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins":{"builder":{"cli":"claude","model":"deep"}}}`)
	b := phaseByRole(rep, "builder")
	if b.Source != "policy-pin" || b.CurrentCLI != "claude" || b.CurrentTier != "deep" {
		t.Errorf("expected pinned claude/deep, got %+v", b)
	}
	if b.PinViolation != "" {
		t.Errorf("valid pin should have no violation, got %q", b.PinViolation)
	}
	if s := phaseByRole(rep, "scout"); s.Source != "profile" {
		t.Errorf("unpinned scout should stay profile-sourced, got %+v", s)
	}
}

func TestDetect_PhaseDefaultsFromProfile(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins":{"builder":{"cli":"claude","model":"deep"}}}`)
	b := phaseByRole(rep, "builder")
	if b.DefaultCLI != "agy-tmux" || b.DefaultTier != "sonnet" {
		t.Errorf("profile defaults: got cli=%q tier=%q, want agy-tmux/sonnet", b.DefaultCLI, b.DefaultTier)
	}
	if b.CurrentCLI != "claude" || b.CurrentTier != "deep" {
		t.Errorf("pin overlay broken: %+v", b)
	}
	s := phaseByRole(rep, "scout")
	if s.DefaultCLI != s.CurrentCLI || s.DefaultTier != s.CurrentTier {
		t.Errorf("unpinned scout Default*/Current* should match: %+v", s)
	}
	if s.DefaultCLI != "claude-tmux" || s.DefaultTier != "sonnet" {
		t.Errorf("scout profile defaults: got cli=%q tier=%q, want claude-tmux/sonnet", s.DefaultCLI, s.DefaultTier)
	}
}

func TestDetect_PolicyPinCLIViolation(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins":{"builder":{"cli":"codex","model":"deep"}}}`)
	b := phaseByRole(rep, "builder")
	if b.Source != "policy-pin" || b.CurrentCLI != "codex" {
		t.Errorf("pin should overlay even when invalid, got %+v", b)
	}
	if b.PinViolation == "" || !strings.Contains(b.PinViolation, "allowed_clis") {
		t.Errorf("expected allowed_clis violation, got %q", b.PinViolation)
	}
}

func TestDetect_PolicyPinTierViolation(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins":{"auditor":{"cli":"claude","model":"fast"}}}`)
	a := phaseByRole(rep, "auditor")
	if a.PinViolation == "" || !strings.Contains(a.PinViolation, "envelope") {
		t.Errorf("expected envelope violation, got %q", a.PinViolation)
	}
}

func TestDetect_PolicyPinNoProfile(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins":{"intent":{"cli":"claude","model":"opus"}}}`)
	i := phaseByRole(rep, "intent")
	if i.Source != "policy-pin" {
		t.Errorf("pin should overlay even without a profile, got %+v", i)
	}
	if i.PinViolation == "" || !strings.Contains(i.PinViolation, "not found") {
		t.Errorf("expected a profile-not-found violation, got %q", i.PinViolation)
	}
}

func TestDetect_MalformedPolicy(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins": {not json`)
	if rep.PolicyError == "" {
		t.Error("malformed policy.json should set PolicyError")
	}
	if b := phaseByRole(rep, "builder"); b.Source != "profile" {
		t.Errorf("malformed policy should leave builder profile-sourced, got %+v", b)
	}
}

func TestCapTierFromManifest(t *testing.T) {
	if got := capTierFromManifest("", "claude"); got != "unknown" {
		t.Errorf("empty adaptersDir: got %q, want unknown", got)
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "claude.capabilities.json"),
		`{"supports": {"budget_cap_native": true, "permission_scoping": true}}`)
	if got := capTierFromManifest(dir, "claude"); got != "full" {
		t.Errorf("both-native manifest: got %q, want full", got)
	}

	writeFile(t, filepath.Join(dir, "codex.capabilities.json"),
		`{"supports": {"budget_cap_native": false, "permission_scoping": true}}`)
	if got := capTierFromManifest(dir, "codex"); got != "delegated" {
		t.Errorf("missing-budget manifest: got %q, want delegated", got)
	}

	writeFile(t, filepath.Join(dir, "antigravity.capabilities.json"),
		`{"supports": {"budget_cap_native": true, "permission_scoping": true}}`)
	if got := capTierFromManifest(dir, "agy"); got != "full" {
		t.Errorf("agy via antigravity manifest: got %q, want full", got)
	}

	if got := capTierFromManifest(dir, "gemini"); got != "full" {
		t.Errorf("absent manifest: got %q, want full (Inspect defaults true)", got)
	}

	unreadableManifestIsADirectory := filepath.Join(dir, "perl.capabilities.json")
	if err := os.MkdirAll(unreadableManifestIsADirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := capTierFromManifest(dir, "perl"); got != "unknown" {
		t.Errorf("manifest-read error: got %q, want unknown", got)
	}
}

func TestReadProfileConstraints_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "scout.json"), `{not valid json`)
	_, ok := readProfileConstraints(dir, "scout")
	if ok {
		t.Error("malformed profile JSON should report ok=false")
	}
	if _, ok := readProfileConstraints(dir, "absent"); ok {
		t.Error("missing profile should report ok=false")
	}
}

func TestDetect_DefaultSeams(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	writeFile(t, filepath.Join(evolveDir, "state.json"),
		`{"setupCompletedAt":"2025-12-31T00:00:00Z","setupVersion":1}`)

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

func TestDetect_NilDoctorAndCapTierSeams(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	emptyAdaptersDir := t.TempDir()
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		AdaptersDir: emptyAdaptersDir,
	})
	if rep.ScannedAt == "" {
		t.Error("default Now seam should stamp ScannedAt")
	}
	if len(rep.Phases) != len(Roles) {
		t.Errorf("phases len = %d, want %d (one per Role)", len(rep.Phases), len(Roles))
	}
	seen := map[string]bool{}
	for _, c := range rep.CLIs {
		if seen[c.CLI] {
			t.Errorf("duplicate CLI family in report: %q", c.CLI)
		}
		seen[c.CLI] = true
	}
}

func TestCompletePreservesUnmodeledKeys(t *testing.T) {
	evolveDir := t.TempDir()
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

func TestCompleteMkdirFails(t *testing.T) {
	base := t.TempDir()
	fileAsParent := filepath.Join(base, "iam-a-file")
	writeFile(t, fileAsParent, "x")
	_, err := Complete(CompleteOptions{EvolveDir: filepath.Join(fileAsParent, "evolve")})
	if err == nil {
		t.Error("Complete under a file-path parent should fail at MkdirAll")
	}
	if err != nil && !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("error should name the mkdir step, got %v", err)
	}
}
