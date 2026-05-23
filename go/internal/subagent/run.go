package subagent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// RunRequest captures every input cmd_run reads from argv + environment.
// Mirrors the bash signature subagent-run.sh <agent> <cycle> <workspace>
// + env overrides (PROMPT_FILE_OVERRIDE, MODEL_TIER_HINT, WORKTREE_PATH,
// ADVERSARIAL_AUDIT, EVOLVE_CACHE_PREFIX_V2, LEGACY_AGENT_DISPATCH).
type RunRequest struct {
	Agent         string
	Cycle         int
	WorkspacePath string

	ProfilesDir   string
	AdaptersDir   string
	CapabilityDir string
	ProjectRoot   string
	PluginRoot    string
	WorktreePath  string
	LLMConfigPath string
	LedgerPath    string

	// PromptReader supplies the user task prompt. Caller can pass an
	// *os.File (PROMPT_FILE_OVERRIDE), os.Stdin, or a bytes.Buffer.
	// MUST be non-nil — bash fails fast when no prompt source is configured.
	PromptReader io.Reader

	// Env overrides bash reads at function entry.
	ModelTierHint          string // MODEL_TIER_HINT
	AuditorTierOverride    string // EVOLVE_AUDITOR_TIER_OVERRIDE
	DiffComplexityDisabled bool   // EVOLVE_DIFF_COMPLEXITY_DISABLE=1
	CachePrefixV2          bool   // EVOLVE_CACHE_PREFIX_V2 (default true)
	AdversarialAudit       bool   // ADVERSARIAL_AUDIT (default true)
	LegacyAgentDispatch    bool   // LEGACY_AGENT_DISPATCH=1 — escape hatch
}

// RunOptions injects the I/O + sub-process seams. Production wires
// defaults; tests substitute doubles.
type RunOptions struct {
	ReadProfile       func(path string) (string, error)
	ResolveLLM        func(agent, configPath string) (resolvellm.Result, error)
	InspectCapability func(adaptersDir, cli string) (capability.Inspection, error)
	ResolveModelTier  func(req ResolveModelTierRequest, opts ResolveModelTierOptions) (string, error)
	AdapterExists     func(path string) bool
	ExecAdapter       func(ctx context.Context, adapterPath string, env map[string]string) (exitCode int, err error)
	WriteFile         func(path string, data []byte, mode os.FileMode) error
	GitState          func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	StatMTime         func(path string) (time.Time, error)
	ReadFile          func(path string) ([]byte, error)
	HashFile          func(path string) (string, error)
	Now               func() time.Time
	Rand              func([]byte) (int, error)
}

// RunResult carries everything cmd_run printed + the side effects.
// Verdict is one of VerdictPASS / VerdictFAIL / VerdictIntegrityFail.
type RunResult struct {
	Verdict         string
	CLI             string
	Model           string
	ArtifactPath    string
	ArtifactSHA256  string
	ChallengeToken  string
	ExitCode        int
	DurationMS      int64
	Warns           []string
	LegacyDispatch  bool   // true ⇒ LEGACY_DISPATCH escape hatch fired
	Stderr          string // collected adapter stderr for caller logging
}

// workerNameRE matches fan-out worker names: <role>-worker-<subtask>.
// Subtask names may include digits and hyphens after the first letter.
var workerNameRE = regexp.MustCompile(`^([a-z][a-z-]+)-worker-([a-z][a-z0-9-]+)$`)

// agentRolePattern is the bash allow-list of canonical agent roles. Must
// stay in sync with subagent-run.sh:631.
var agentRolePattern = regexp.MustCompile(
	`^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer|intent|triage|memo|tester|build-planner)$`,
)

// Run ports cmd_run from legacy/scripts/dispatch/subagent-run.sh:619.
//
// Production-path scope (v11.6.5):
//   - argument validation (agent + cycle + workspace)
//   - worker-name regex parsing
//   - profile load + JSON validate
//   - resolve-llm + cli/model resolution + antigravity→agy
//   - adapter exists check + LEGACY_AGENT_DISPATCH escape hatch
//   - model tier resolution
//   - artifact path resolution (worker variant)
//   - challenge token + git state capture
//   - prompt source from PromptReader (PROMPT_FILE_OVERRIDE or stdin)
//   - v2 cache-prefix prompt assembly (INVOCATION CONTEXT + task envelope)
//   - adversarial auditor framing for auditor agent
//   - adapter exec with VALIDATE_ONLY=0 + full env propagation
//   - artifact verification (exists, fresh <5min, token-bearing)
//   - kind="agent_subprocess" ledger entry write
//
// Deferred to follow-on ships:
//   - v1 legacy prompt path (EVOLVE_CACHE_PREFIX_V2=0) — fossil per v10.6
//   - prompt-size guard (EVOLVE_PROMPT_MAX_TOKENS / autotrim)
//   - fast-fail consecutive-failure counter (v9.1.0 cycle-94)
//   - phase timing sidecar
//   - cache-prefix v2 file emission (handled by separate evolve subagent cache-prefix)
//   - context-monitor sidecar (cycle-6 v9.1.0)
func Run(ctx context.Context, req RunRequest, opts RunOptions) (RunResult, error) {
	fillRunDefaults(&opts)

	// Step 1: validate inputs.
	if req.PromptReader == nil {
		return RunResult{}, fmt.Errorf("subagent/run: PromptReader required (PROMPT_FILE_OVERRIDE or stdin)")
	}
	role, worker := parseAgentName(req.Agent)
	if !agentRolePattern.MatchString(role) {
		return RunResult{}, fmt.Errorf("subagent/run: unknown agent: %s", req.Agent)
	}
	if req.Cycle < 0 {
		return RunResult{}, fmt.Errorf("subagent/run: cycle must be >= 0, got %d", req.Cycle)
	}
	if info, err := os.Stat(req.WorkspacePath); err != nil || !info.IsDir() {
		return RunResult{}, fmt.Errorf("subagent/run: workspace dir does not exist: %s", req.WorkspacePath)
	}

	// Step 2: load + validate profile.
	profilePath := filepath.Join(req.ProfilesDir, role+".json")
	profileBody, err := opts.ReadProfile(profilePath)
	if err != nil {
		return RunResult{}, fmt.Errorf("subagent/run: profile not found: %s", profilePath)
	}

	// Step 3: resolve cli + model via LLM router (with profile fallback).
	llm, llmErr := opts.ResolveLLM(role, req.LLMConfigPath)
	var cli, source, resolvedModel string
	if llmErr == nil && llm.CLI != "" {
		cli, source = llm.CLI, llm.Source
		resolvedModel = llm.Model
		if resolvedModel == "" {
			resolvedModel = llm.ModelTier
		}
	} else {
		cli = extractProfileString(profileBody, "cli")
		source = "profile"
	}
	if cli == "antigravity" {
		cli = "agy"
	}
	if cli == "" {
		return RunResult{}, fmt.Errorf("subagent/run: cli unresolved for agent %s", req.Agent)
	}

	// Step 4: LEGACY_AGENT_DISPATCH escape hatch.
	if req.LegacyAgentDispatch {
		return RunResult{
			Verdict:        VerdictFAIL,
			CLI:            cli,
			LegacyDispatch: true,
		}, nil
	}

	// Step 5: adapter exists.
	adapterPath := filepath.Join(req.AdaptersDir, cli+".sh")
	if !opts.AdapterExists(adapterPath) {
		return RunResult{}, fmt.Errorf("subagent/run: adapter not executable: %s", adapterPath)
	}

	// Step 6: model tier resolution. llm_config override wins; otherwise the
	// adaptive resolver evaluates profile + mastery gate + diff complexity.
	var model string
	if resolvedModel != "" {
		model = resolvedModel
	} else {
		model, err = opts.ResolveModelTier(
			ResolveModelTierRequest{
				ProfilePath:            profilePath,
				Cycle:                  req.Cycle,
				ProjectRoot:            req.ProjectRoot,
				WorktreePath:           req.WorktreePath,
				ModelTierHint:          req.ModelTierHint,
				AuditorTierOverride:    req.AuditorTierOverride,
				DiffComplexityDisabled: req.DiffComplexityDisabled,
			},
			ResolveModelTierOptions{},
		)
		if err != nil {
			return RunResult{}, fmt.Errorf("subagent/run: resolve tier: %w", err)
		}
	}

	// Step 7: capability inspection — adds WARN strings + plan log fodder.
	capDir := req.CapabilityDir
	if capDir == "" {
		capDir = req.AdaptersDir
	}
	insp, err := opts.InspectCapability(capDir, cli)
	if err != nil {
		return RunResult{}, fmt.Errorf("subagent/run: capability inspect: %w", err)
	}

	// Step 8: artifact path. Workers override the profile template.
	var artifactPath string
	if worker != "" {
		artifactPath = filepath.Join(req.WorkspacePath, "workers", req.Agent+".md")
	} else {
		template := extractProfileString(profileBody, "output_artifact")
		artifactPath = resolveArtifactPath(template, req.Cycle, req.ProjectRoot)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return RunResult{}, fmt.Errorf("subagent/run: mkdir artifact dir: %w", err)
	}

	// Step 9: challenge token + git state.
	token, err := generateRunToken(opts.Rand)
	if err != nil {
		return RunResult{}, fmt.Errorf("subagent/run: token: %w", err)
	}
	gitHead, treeDiff, _ := opts.GitState(ctx, req.ProjectRoot)
	if gitHead == "" {
		gitHead = "unknown"
	}
	if treeDiff == "" {
		treeDiff = "unknown"
	}

	// Step 10: read user prompt body.
	promptBody, err := io.ReadAll(req.PromptReader)
	if err != nil {
		return RunResult{}, fmt.Errorf("subagent/run: read prompt: %w", err)
	}

	// Step 11: assemble v2 cache-prefix prompt.
	fullPrompt := assembleV2Prompt(req.Agent, req.Cycle, req.WorkspacePath, artifactPath, token, filepath.Base(profilePath), string(promptBody))
	if role == "auditor" && req.AdversarialAudit {
		fullPrompt += adversarialAuditFraming()
	}

	// Step 12: build adapter env + exec.
	worktreePath := req.WorktreePath
	if worktreePath == "" {
		worktreePath = req.ProjectRoot
	}
	promptFile, err := os.CreateTemp("", "evolve-subagent-prompt-*.txt")
	if err != nil {
		return RunResult{}, fmt.Errorf("subagent/run: prompt tempfile: %w", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)
	if _, err := promptFile.WriteString(fullPrompt); err != nil {
		promptFile.Close()
		return RunResult{}, fmt.Errorf("subagent/run: write prompt tempfile: %w", err)
	}
	promptFile.Close()

	overrides := extractAdapterOverrides(profileBody, cli)
	env := map[string]string{
		"PROFILE_PATH":                 profilePath,
		"RESOLVED_MODEL":               model,
		"PROMPT_FILE":                  promptPath,
		"CYCLE":                        strconv.Itoa(req.Cycle),
		"WORKSPACE_PATH":               req.WorkspacePath,
		"WORKTREE_PATH":                worktreePath,
		"STDOUT_LOG":                   filepath.Join(req.WorkspacePath, req.Agent+".stdout.log"),
		"STDERR_LOG":                   filepath.Join(req.WorkspacePath, req.Agent+".stderr.log"),
		"ARTIFACT_PATH":                artifactPath,
		"RESOLVED_CLI":                 cli,
		"CLI_RESOLUTION_SOURCE":        source,
		"CAP_BUDGET_NATIVE":            capBoolEnv(insp.Manifest.BudgetNative),
		"ADAPTER_TOOLS_OVERRIDE":       overrides.ToolsJSON,
		"ADAPTER_EXTRA_FLAGS_OVERRIDE": overrides.ExtraFlagsJSON,
		"VALIDATE_ONLY":                "0",
		"CHALLENGE_TOKEN":              token,
	}

	start := opts.Now()
	exitCode, execErr := opts.ExecAdapter(ctx, adapterPath, env)
	durationMS := opts.Now().Sub(start).Milliseconds()

	res := RunResult{
		CLI:            cli,
		Model:          model,
		ArtifactPath:   artifactPath,
		ChallengeToken: token,
		ExitCode:       exitCode,
		DurationMS:     durationMS,
		Warns:          insp.Warns,
	}

	// Step 13: verify artifact (exists, fresh, contains token).
	verdict := classifyArtifact(opts, artifactPath, token, exitCode, execErr)
	res.Verdict = verdict
	if sha, hashErr := opts.HashFile(artifactPath); hashErr == nil {
		res.ArtifactSHA256 = sha
	}

	// Step 14: write ledger entry. Always (bash always logs an entry to
	// preserve the audit chain even on failure).
	if req.LedgerPath != "" {
		if err := writeSubprocessLedger(req.LedgerPath, subprocessLedger{
			Cycle:          req.Cycle,
			Role:           req.Agent,
			Model:          model,
			ExitCode:       exitCode,
			DurationS:      strconv.FormatInt(durationMS/1000, 10),
			ArtifactPath:   artifactPath,
			ArtifactSHA256: res.ArtifactSHA256,
			ChallengeToken: token,
			GitHEAD:        gitHead,
			TreeStateSHA:   treeDiff,
			QualityTier:    capabilityTier(insp.Manifest),
		}, opts.Now); err != nil {
			return res, fmt.Errorf("subagent/run: ledger write: %w", err)
		}
	}

	if execErr != nil {
		return res, fmt.Errorf("subagent/run: adapter exec: %w", execErr)
	}
	return res, nil
}

func parseAgentName(agent string) (role, worker string) {
	if m := workerNameRE.FindStringSubmatch(agent); len(m) == 3 {
		return m[1], m[2]
	}
	return agent, ""
}

func assembleV2Prompt(agent string, cycle int, workspace, artifactPath, token, profileBase, body string) string {
	var b strings.Builder
	b.WriteString("## INVOCATION CONTEXT\n\n")
	fmt.Fprintf(&b, "- Agent: %s\n", agent)
	fmt.Fprintf(&b, "- Cycle: %d\n", cycle)
	fmt.Fprintf(&b, "- Workspace: %s\n", workspace)
	fmt.Fprintf(&b, "- Artifact path: %s\n", artifactPath)
	fmt.Fprintf(&b, "- Challenge token: %s\n", token)
	fmt.Fprintf(&b, "- Profile: %s\n\n", profileBase)
	b.WriteString("--- BEGIN TASK PROMPT ---\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("--- END TASK PROMPT ---\n")
	return b.String()
}

// adversarialAuditFraming returns the auditor anti-sycophancy block bash
// emits when agent=auditor && ADVERSARIAL_AUDIT!=0. Verbatim copy of bash
// here-doc for prompt-cache parity.
func adversarialAuditFraming() string {
	return `ADVERSARIAL AUDIT MODE (default-on)

Your role is not to confirm correctness; it is to find a real defect.

Treat the build as guilty until proven innocent. Specifically:
- A "PASS" verdict requires positive evidence that each acceptance criterion is
  met by executable behavior — not by the presence of expected strings in source
  code. Cite the test output, the diff hunk, or the command that demonstrates it.
- Confidence below 0.85 → WARN, not PASS. "I see no problems" is not 0.85
  confidence; it is the absence of evidence, which is the absence of an audit.
- If you have produced ≥5 consecutive PASS verdicts in this loop, the prior is
  now SHIFTED toward latent defects — go deeper than your routine checklist.
- A vague affirmative review is itself a failure. Output ` + "`NO_DEFECT_FOUND`" + ` with
  explicit per-criterion evidence, OR list at least one concrete defect with
  file:line and a reproduction command.

`
}

func classifyArtifact(opts RunOptions, artifactPath, token string, exitCode int, execErr error) string {
	info, statErr := opts.StatMTime(artifactPath)
	if statErr != nil {
		return VerdictIntegrityFail
	}
	if opts.Now().Sub(info) > ArtifactMaxAge {
		return VerdictIntegrityFail
	}
	body, readErr := opts.ReadFile(artifactPath)
	if readErr != nil || len(body) == 0 {
		return VerdictIntegrityFail
	}
	if !bytes.Contains(body, []byte(token)) {
		return VerdictIntegrityFail
	}
	if execErr != nil || exitCode != 0 {
		return VerdictFAIL
	}
	return VerdictPASS
}

type subprocessLedger struct {
	Cycle          int
	Role           string
	Model          string
	ExitCode       int
	DurationS      string
	ArtifactPath   string
	ArtifactSHA256 string
	ChallengeToken string
	GitHEAD        string
	TreeStateSHA   string
	QualityTier    string
}

// writeSubprocessLedger appends a `kind: "agent_subprocess"` entry. Field
// order matches bash write_ledger_entry at subagent-run.sh:436 — chain
// hash determinism depends on it.
func writeSubprocessLedger(ledgerPath string, e subprocessLedger, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return err
	}
	prevHash, entrySeq, err := readChainLink(ledgerPath)
	if err != nil {
		return err
	}
	line := fmt.Sprintf(
		`{"ts":"%s","cycle":%d,"role":"%s","kind":"agent_subprocess","model":"%s","exit_code":%d,`+
			`"duration_s":"%s","artifact_path":"%s","artifact_sha256":"%s","challenge_token":"%s",`+
			`"git_head":"%s","tree_state_sha":"%s","entry_seq":%d,"prev_hash":"%s","quality_tier":"%s","cli_resolution":null}`,
		jsonStringEscape(now().UTC().Format("2006-01-02T15:04:05Z")),
		e.Cycle,
		jsonStringEscape(e.Role),
		jsonStringEscape(e.Model),
		e.ExitCode,
		jsonStringEscape(e.DurationS),
		jsonStringEscape(e.ArtifactPath),
		e.ArtifactSHA256,
		jsonStringEscape(e.ChallengeToken),
		jsonStringEscape(e.GitHEAD),
		jsonStringEscape(e.TreeStateSHA),
		entrySeq,
		prevHash,
		jsonStringEscape(e.QualityTier),
	)
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	tipPath := filepath.Join(filepath.Dir(ledgerPath), "ledger.tip")
	tip := fmt.Sprintf("%d:%s\n", entrySeq, sha256Hex(line))
	tmp := tipPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(tip), 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, tipPath)
}

// capabilityTier maps Manifest support flags to the v8.51.0 quality_tier
// label used by ledger entries. full = both supports true; degraded = both
// false; hybrid = one of each.
func capabilityTier(m capability.Manifest) string {
	if m.BudgetNative && m.PermissionScoping {
		return "full"
	}
	if !m.BudgetNative && !m.PermissionScoping {
		return "degraded"
	}
	return "hybrid"
}

func generateRunToken(rng func([]byte) (int, error)) (string, error) {
	if rng == nil {
		rng = rand.Read
	}
	buf := make([]byte, ChallengeTokenBytes)
	n, err := rng(buf)
	if err != nil {
		return "", err
	}
	if n != ChallengeTokenBytes {
		return "", fmt.Errorf("rand returned %d bytes, want %d", n, ChallengeTokenBytes)
	}
	return hex.EncodeToString(buf), nil
}

func fillRunDefaults(opts *RunOptions) {
	if opts.ReadProfile == nil {
		opts.ReadProfile = defaultReadProfile
	}
	if opts.ResolveLLM == nil {
		opts.ResolveLLM = defaultResolveLLM
	}
	if opts.InspectCapability == nil {
		opts.InspectCapability = capability.Inspect
	}
	if opts.ResolveModelTier == nil {
		opts.ResolveModelTier = ResolveModelTier
	}
	if opts.AdapterExists == nil {
		opts.AdapterExists = defaultAdapterExists
	}
	if opts.ExecAdapter == nil {
		opts.ExecAdapter = defaultExecAdapter
	}
	if opts.WriteFile == nil {
		opts.WriteFile = os.WriteFile
	}
	if opts.GitState == nil {
		opts.GitState = defaultGitState
	}
	if opts.StatMTime == nil {
		opts.StatMTime = defaultStatMTime
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	if opts.HashFile == nil {
		opts.HashFile = defaultHashFile
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Rand == nil {
		opts.Rand = rand.Read
	}
}
