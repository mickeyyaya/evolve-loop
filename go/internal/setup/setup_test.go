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

// fixtureRepo lays down a temp project with .evolve/profiles.
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

// --- CLI-name normalization ---

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

// detectWithPolicy runs Detect over fixtureRepo with the offline seams, after
// writing the given policy.json body (empty body → no file).
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

// TestDetect_PolicyPinOverlay: a valid pin overrides the profile routing and is
// reported with source="policy-pin" and no violation.
func TestDetect_PolicyPinOverlay(t *testing.T) {
	// builder profile: allowed_clis [claude,agy], envelope balanced..deep.
	// Pin claude+deep is within both guardrails.
	rep := detectWithPolicy(t, `{"pins":{"builder":{"cli":"claude","model":"deep"}}}`)
	b := phaseByRole(rep, "builder")
	if b.Source != "policy-pin" || b.CurrentCLI != "claude" || b.CurrentTier != "deep" {
		t.Errorf("expected pinned claude/deep, got %+v", b)
	}
	if b.PinViolation != "" {
		t.Errorf("valid pin should have no violation, got %q", b.PinViolation)
	}
	// An unpinned phase keeps its profile routing.
	if s := phaseByRole(rep, "scout"); s.Source != "profile" {
		t.Errorf("unpinned scout should stay profile-sourced, got %+v", s)
	}
}

// TestDetect_PolicyPinCLIViolation: a pin whose CLI is outside allowed_clis is
// surfaced as a violation (still overlaid so the user sees what they wrote).
func TestDetect_PolicyPinCLIViolation(t *testing.T) {
	// builder allows only [claude,agy]; codex breaches allowed_clis.
	rep := detectWithPolicy(t, `{"pins":{"builder":{"cli":"codex","model":"deep"}}}`)
	b := phaseByRole(rep, "builder")
	if b.Source != "policy-pin" || b.CurrentCLI != "codex" {
		t.Errorf("pin should overlay even when invalid, got %+v", b)
	}
	if b.PinViolation == "" || !strings.Contains(b.PinViolation, "allowed_clis") {
		t.Errorf("expected allowed_clis violation, got %q", b.PinViolation)
	}
}

// TestDetect_PolicyPinTierViolation: a pin whose tier is outside the envelope is
// surfaced as a violation.
func TestDetect_PolicyPinTierViolation(t *testing.T) {
	// auditor envelope is deep..deep; fast (rank 1) is below min.
	rep := detectWithPolicy(t, `{"pins":{"auditor":{"cli":"claude","model":"fast"}}}`)
	a := phaseByRole(rep, "auditor")
	if a.PinViolation == "" || !strings.Contains(a.PinViolation, "envelope") {
		t.Errorf("expected envelope violation, got %q", a.PinViolation)
	}
}

// TestDetect_PolicyPinNoProfile: a pin for a phase with no profile file can't be
// floor-checked, so detect reports a violation rather than a false green.
func TestDetect_PolicyPinNoProfile(t *testing.T) {
	// fixtureRepo writes builder/auditor/scout profiles only — "intent" has none.
	rep := detectWithPolicy(t, `{"pins":{"intent":{"cli":"claude","model":"opus"}}}`)
	i := phaseByRole(rep, "intent")
	if i.Source != "policy-pin" {
		t.Errorf("pin should overlay even without a profile, got %+v", i)
	}
	if i.PinViolation == "" || !strings.Contains(i.PinViolation, "not found") {
		t.Errorf("expected a profile-not-found violation, got %q", i.PinViolation)
	}
}

// TestDetect_MalformedPolicy: a present-but-unparseable policy.json sets
// PolicyError and disables pin overlay (phases stay profile-sourced).
func TestDetect_MalformedPolicy(t *testing.T) {
	rep := detectWithPolicy(t, `{"pins": {not json`)
	if rep.PolicyError == "" {
		t.Error("malformed policy.json should set PolicyError")
	}
	if b := phaseByRole(rep, "builder"); b.Source != "profile" {
		t.Errorf("malformed policy should leave builder profile-sourced, got %+v", b)
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
