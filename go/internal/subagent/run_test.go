package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// runHappyOpts returns a RunOptions blob where every seam is configured
// for a successful path. Tests override one or two seams at a time.
func runHappyOpts(t *testing.T) RunOptions {
	t.Helper()
	clock := fixedClock(t, "2026-05-23T17:00:00Z")
	return RunOptions{
		ReadProfile: func(string) (string, error) {
			return `{"role":"scout","cli":"claude","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/scout.md"}`, nil
		},
		ResolveLLM: func(string) (resolvellm.Result, error) {
			return resolvellm.Result{CLI: "claude", ModelTier: "sonnet", Source: "profile"}, nil
		},
		InspectCapability: func(string, string) (capability.Inspection, error) {
			return capability.Inspection{
				Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true},
			}, nil
		},
		ResolveModelTier: func(ResolveModelTierRequest, ResolveModelTierOptions) (string, error) {
			return "sonnet", nil
		},
		AdapterExists: func(string) bool { return true },
		ExecAdapter: func(_ context.Context, _ string, env map[string]string) (int, error) {
			// Materialize a valid artifact so verify passes.
			path := env["ARTIFACT_PATH"]
			if path == "" {
				return 1, nil
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return 1, err
			}
			body := "<!-- challenge-token: " + env["CHALLENGE_TOKEN"] + " -->\nbody\n"
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return 1, err
			}
			now := clock()
			_ = os.Chtimes(path, now, now)
			return 0, nil
		},
		WriteFile: os.WriteFile,
		GitState: func(context.Context, string) (string, string, error) {
			return "abc123", "def456", nil
		},
		StatMTime: defaultStatMTime,
		ReadFile:  os.ReadFile,
		HashFile:  defaultHashFile,
		Now:       clock,
		Rand: func(b []byte) (int, error) {
			// Deterministic token — fill with 0xaa.
			for i := range b {
				b[i] = 0xaa
			}
			return len(b), nil
		},
	}
}

func TestRunCmd_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	opts := runHappyOpts(t)
	res, err := Run(context.Background(), RunRequest{
		Agent:            "scout",
		Cycle:            5,
		WorkspacePath:    ws,
		ProfilesDir:      "/p",
		AdaptersDir:      "/a",
		ProjectRoot:      tmp,
		PluginRoot:       tmp,
		PromptReader:     strings.NewReader("Do the thing.\n"),
		AdversarialAudit: true,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Verdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.Verdict)
	}
	if res.CLI != "claude" || res.Model != "sonnet" {
		t.Errorf("CLI=%s Model=%s", res.CLI, res.Model)
	}
	if res.ChallengeToken == "" || res.ArtifactPath == "" {
		t.Errorf("token/artifact path not populated")
	}
}

func TestRun_NilPromptReaderFails(t *testing.T) {
	_, err := Run(context.Background(), RunRequest{}, RunOptions{})
	if err == nil || !strings.Contains(err.Error(), "PromptReader required") {
		t.Errorf("expected PromptReader required, got %v", err)
	}
}

func TestRun_UnknownAgent(t *testing.T) {
	tmp := t.TempDir()
	_, err := Run(context.Background(), RunRequest{
		Agent:         "not-a-real-agent",
		Cycle:         0,
		WorkspacePath: tmp,
		PromptReader:  strings.NewReader("hi"),
	}, runHappyOpts(t))
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("expected unknown-agent error, got %v", err)
	}
}

func TestRun_NegativeCycle(t *testing.T) {
	tmp := t.TempDir()
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         -1,
		WorkspacePath: tmp,
		PromptReader:  strings.NewReader("hi"),
	}, runHappyOpts(t))
	if err == nil || !strings.Contains(err.Error(), "cycle must be >= 0") {
		t.Errorf("expected cycle error, got %v", err)
	}
}

func TestRun_MissingWorkspaceDir(t *testing.T) {
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: "/non/existent",
		PromptReader:  strings.NewReader("hi"),
	}, runHappyOpts(t))
	if err == nil || !strings.Contains(err.Error(), "workspace dir") {
		t.Errorf("expected workspace error, got %v", err)
	}
}

func TestRun_ProfileNotFound(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ReadProfile = func(string) (string, error) { return "", os.ErrNotExist }
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "profile not found") {
		t.Errorf("got %v", err)
	}
}

func TestRun_WorkerNameRoute(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	// Capture artifact path inside ExecAdapter so we can assert worker route.
	var capturedArtifact string
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		capturedArtifact = env["ARTIFACT_PATH"]
		path := env["ARTIFACT_PATH"]
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return 1, err
		}
		body := "<!-- challenge-token: " + env["CHALLENGE_TOKEN"] + " -->\nbody\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return 1, err
		}
		now := opts.Now()
		_ = os.Chtimes(path, now, now)
		return 0, nil
	}
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout-worker-codebase",
		Cycle:         3,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := filepath.Join(tmp, "workers", "scout-worker-codebase.md")
	if capturedArtifact != want {
		t.Errorf("worker artifact path=%s, want %s", capturedArtifact, want)
	}
}

func TestRun_AntigravityRemappedToAgy(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{CLI: "antigravity", ModelTier: "sonnet", Source: "profile"}, nil
	}
	res, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.CLI != "agy" {
		t.Errorf("expected agy remap, got %s", res.CLI)
	}
}

// TestRun_InProcessDispatchBanned pins the B1 bridge-only invariant: requesting
// the retired in-process dispatch path (LEGACY_AGENT_DISPATCH=1) is a hard error,
// never a soft RunResult that signals the orchestrator to fall back in-process.
func TestRun_InProcessDispatchBanned(t *testing.T) {
	tmp := t.TempDir()
	res, err := Run(context.Background(), RunRequest{
		Agent:               "scout",
		Cycle:               0,
		WorkspacePath:       tmp,
		ProjectRoot:         tmp,
		PromptReader:        strings.NewReader("hi"),
		LegacyAgentDispatch: true,
	}, runHappyOpts(t))
	if !errors.Is(err, ErrInProcessDispatchBanned) {
		t.Fatalf("expected ErrInProcessDispatchBanned, got err=%v res=%+v", err, res)
	}
	if res.Verdict != "" || res.CLI != "" {
		t.Errorf("expected zero RunResult on banned dispatch, got %+v", res)
	}
}

// TestRun_BridgeOnlyInvariant_AllRoles proves the in-process escape hatch is
// unreachable for EVERY registered agent role. It iterates the agentRoles
// registry SSOT, so a role added there is genuinely auto-covered here.
func TestRun_BridgeOnlyInvariant_AllRoles(t *testing.T) {
	if len(agentRoles) == 0 {
		t.Fatal("agentRoles registry is empty")
	}
	for _, role := range agentRoles {
		t.Run(role, func(t *testing.T) {
			tmp := t.TempDir()
			_, err := Run(context.Background(), RunRequest{
				Agent:               role,
				Cycle:               0,
				WorkspacePath:       tmp,
				ProjectRoot:         tmp,
				PromptReader:        strings.NewReader("hi"),
				LegacyAgentDispatch: true,
			}, runHappyOpts(t))
			if !errors.Is(err, ErrInProcessDispatchBanned) {
				t.Errorf("role %s: in-process dispatch must be banned, got err=%v", role, err)
			}
		})
	}
}

func TestRun_AdapterMissing(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.AdapterExists = func(string) bool { return false }
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "adapter not executable") {
		t.Errorf("got %v", err)
	}
}

func TestRun_AdversarialAuditFramingFiresForAuditor(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ReadProfile = func(string) (string, error) {
		return `{"role":"auditor","cli":"claude","model_tier_default":"opus","output_artifact":".evolve/runs/cycle-{cycle}/audit.md"}`, nil
	}
	var capturedPrompt string
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		body, _ := os.ReadFile(env["PROMPT_FILE"])
		capturedPrompt = string(body)
		path := env["ARTIFACT_PATH"]
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		artifactBody := "<!-- challenge-token: " + env["CHALLENGE_TOKEN"] + " -->\nbody\n"
		_ = os.WriteFile(path, []byte(artifactBody), 0o644)
		now := opts.Now()
		_ = os.Chtimes(path, now, now)
		return 0, nil
	}
	_, err := Run(context.Background(), RunRequest{
		Agent:            "auditor",
		Cycle:            5,
		WorkspacePath:    tmp,
		ProjectRoot:      tmp,
		PromptReader:     strings.NewReader("audit body"),
		AdversarialAudit: true,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(capturedPrompt, "ADVERSARIAL AUDIT MODE") {
		t.Errorf("auditor prompt missing adversarial framing")
	}
}

func TestRun_AdversarialAuditDisabledNotFires(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ReadProfile = func(string) (string, error) {
		return `{"role":"auditor","cli":"claude","model_tier_default":"opus","output_artifact":".evolve/runs/cycle-{cycle}/audit.md"}`, nil
	}
	var capturedPrompt string
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		body, _ := os.ReadFile(env["PROMPT_FILE"])
		capturedPrompt = string(body)
		path := env["ARTIFACT_PATH"]
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		artifactBody := "<!-- challenge-token: " + env["CHALLENGE_TOKEN"] + " -->\n"
		_ = os.WriteFile(path, []byte(artifactBody), 0o644)
		now := opts.Now()
		_ = os.Chtimes(path, now, now)
		return 0, nil
	}
	_, err := Run(context.Background(), RunRequest{
		Agent:            "auditor",
		Cycle:            5,
		WorkspacePath:    tmp,
		ProjectRoot:      tmp,
		PromptReader:     strings.NewReader("audit body"),
		AdversarialAudit: false,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if strings.Contains(capturedPrompt, "ADVERSARIAL AUDIT MODE") {
		t.Errorf("adversarial framing should be suppressed when disabled")
	}
}

func TestRun_AdapterNonZeroExitFailsVerdict(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		path := env["ARTIFACT_PATH"]
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		body := "<!-- challenge-token: " + env["CHALLENGE_TOKEN"] + " -->\n"
		_ = os.WriteFile(path, []byte(body), 0o644)
		now := opts.Now()
		_ = os.Chtimes(path, now, now)
		return 5, nil
	}
	res, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Verdict != VerdictFAIL {
		t.Errorf("verdict=%s, want FAIL", res.Verdict)
	}
	if res.ExitCode != 5 {
		t.Errorf("ExitCode=%d", res.ExitCode)
	}
}

func TestRun_MissingArtifactIsIntegrityFail(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ExecAdapter = func(_ context.Context, _ string, _ map[string]string) (int, error) {
		// Write no artifact.
		return 0, nil
	}
	res, _ := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if res.Verdict != VerdictIntegrityFail {
		t.Errorf("verdict=%s, want INTEGRITY_FAIL", res.Verdict)
	}
}

func TestRun_TokenAbsentIsIntegrityFail(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		path := env["ARTIFACT_PATH"]
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte("body without token\n"), 0o644)
		now := opts.Now()
		_ = os.Chtimes(path, now, now)
		return 0, nil
	}
	res, _ := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
	}, opts)
	if res.Verdict != VerdictIntegrityFail {
		t.Errorf("verdict=%s, want INTEGRITY_FAIL", res.Verdict)
	}
}

func TestRun_LedgerEntryWritten(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	opts := runHappyOpts(t)
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         7,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
		LedgerPath:    ledger,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"agent_subprocess"`) {
		t.Errorf("ledger missing kind: %s", body)
	}
	if !strings.Contains(string(body), `"cycle":7`) {
		t.Errorf("ledger missing cycle: %s", body)
	}
	if _, err := os.Stat(filepath.Join(tmp, "ledger.tip")); err != nil {
		t.Errorf("tip not written: %v", err)
	}
}

// TestRun_CLIFromProfileWhenResolverFails covers run.go:156-159 — when the
// LLM resolver errors, cli falls back to the profile's "cli" field and source
// becomes "profile".
func TestRun_CLIFromProfileWhenResolverFails(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{}, errors.New("no llm_config")
	}
	// Profile still declares cli=claude; model falls to ResolveModelTier.
	res, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.CLI != "claude" {
		t.Errorf("CLI=%q, want claude (from profile fallback)", res.CLI)
	}
}

// TestRun_CLIUnresolvedFails covers run.go:163-165 — resolver fails AND the
// profile has no cli field, so cli is unresolvable.
func TestRun_CLIUnresolvedFails(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{}, errors.New("no llm_config")
	}
	opts.ReadProfile = func(string) (string, error) {
		return `{"role":"scout","output_artifact":".evolve/runs/cycle-{cycle}/scout.md"}`, nil
	}
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "cli unresolved") {
		t.Errorf("got %v", err)
	}
}

// TestRun_ResolveModelTierInvokedWhenResolverHasNoModel covers run.go:187-199
// — when the resolver returns a CLI but no model/tier, Run delegates to the
// adaptive ResolveModelTier seam.
func TestRun_ResolveModelTierInvokedWhenResolverHasNoModel(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		// CLI present, no Model/ModelTier → forces the tier-resolver branch.
		return resolvellm.Result{CLI: "claude", Source: "profile"}, nil
	}
	called := false
	opts.ResolveModelTier = func(ResolveModelTierRequest, ResolveModelTierOptions) (string, error) {
		called = true
		return "haiku", nil
	}
	res, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !called {
		t.Errorf("ResolveModelTier seam was not invoked")
	}
	if res.Model != "haiku" {
		t.Errorf("Model=%q, want haiku (from tier resolver)", res.Model)
	}
}

// TestRun_ResolveModelTierErrorPropagates covers run.go:200-202.
func TestRun_ResolveModelTierErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.ResolveLLM = func(string) (resolvellm.Result, error) {
		return resolvellm.Result{CLI: "claude", Source: "profile"}, nil
	}
	opts.ResolveModelTier = func(ResolveModelTierRequest, ResolveModelTierOptions) (string, error) {
		return "", errors.New("tier resolution failed")
	}
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "resolve tier") {
		t.Errorf("got %v", err)
	}
}

// TestRun_CapabilityInspectErrorFails covers run.go:211-213.
func TestRun_CapabilityInspectErrorFails(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.InspectCapability = func(string, string) (capability.Inspection, error) {
		return capability.Inspection{}, errors.New("manifest parse error")
	}
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "capability inspect") {
		t.Errorf("got %v", err)
	}
}

// TestRun_GitStateEmptyFallsBackToUnknown covers run.go:233-238 — empty
// git head/diff strings are normalized to "unknown" in the ledger entry.
func TestRun_GitStateEmptyFallsBackToUnknown(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	opts := runHappyOpts(t)
	opts.GitState = func(context.Context, string) (string, string, error) {
		return "", "", nil // empty, not an error
	}
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"), LedgerPath: ledger,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, _ := os.ReadFile(ledger)
	if !strings.Contains(string(body), `"git_head":"unknown"`) {
		t.Errorf("empty git head not normalized to unknown: %s", body)
	}
	if !strings.Contains(string(body), `"tree_state_sha":"unknown"`) {
		t.Errorf("empty tree diff not normalized to unknown: %s", body)
	}
}

// erroringReader fails on Read to drive run.go:242-244.
type erroringReader struct{}

func (erroringReader) Read([]byte) (int, error) { return 0, errors.New("pipe broken") }

// TestRun_PromptReadErrorFails covers run.go:242-244.
func TestRun_PromptReadErrorFails(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: erroringReader{},
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "read prompt") {
		t.Errorf("got %v", err)
	}
}

// TestRun_LedgerWriteErrorPropagates covers run.go:325-327 — a ledger path
// whose parent is a regular file makes writeSubprocessLedger's MkdirAll fail.
func TestRun_LedgerWriteErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	opts := runHappyOpts(t)
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
		LedgerPath:   filepath.Join(blocker, "sub", "ledger.jsonl"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "ledger write") {
		t.Errorf("got %v", err)
	}
}

// TestRun_TokenGenerateErrorAborts covers run.go:229-231 — a failing Rand
// source makes generateRunToken error before the adapter is invoked.
func TestRun_TokenGenerateErrorAborts(t *testing.T) {
	tmp := t.TempDir()
	opts := runHappyOpts(t)
	opts.Rand = func([]byte) (int, error) { return 0, errors.New("entropy depleted") }
	_, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Errorf("got %v", err)
	}
}

// TestRun_AdapterExecErrorReturnsAfterLedger covers run.go:330-332 — when the
// adapter exec itself errors, the ledger entry is still written (if a path is
// set) and the error is returned with the result populated.
func TestRun_AdapterExecErrorReturnsAfterLedger(t *testing.T) {
	tmp := t.TempDir()
	ledger := filepath.Join(tmp, "ledger.jsonl")
	opts := runHappyOpts(t)
	opts.ExecAdapter = func(_ context.Context, _ string, env map[string]string) (int, error) {
		// Still write a valid artifact so verdict classification runs, but
		// return a hard error to exercise the execErr return branch.
		path := env["ARTIFACT_PATH"]
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte("<!-- challenge-token: "+env["CHALLENGE_TOKEN"]+" -->\nbody\n"), 0o644)
		now := opts.Now()
		_ = os.Chtimes(path, now, now)
		return 126, errors.New("adapter crashed")
	}
	res, err := Run(context.Background(), RunRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: tmp, ProjectRoot: tmp,
		PromptReader: strings.NewReader("hi"), LedgerPath: ledger,
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "adapter exec") {
		t.Errorf("got %v", err)
	}
	if res.ExitCode != 126 {
		t.Errorf("ExitCode=%d, want 126 (result populated despite error)", res.ExitCode)
	}
	// Ledger entry written before the error return.
	if _, statErr := os.Stat(ledger); statErr != nil {
		t.Errorf("ledger not written before exec-error return: %v", statErr)
	}
}
