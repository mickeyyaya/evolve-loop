// Package subagent ports the orchestration loop from
// legacy/scripts/dispatch/subagent-run.sh into Go. Its job is to:
//
//  1. Load the agent profile JSON (via profiles.Loader)
//  2. Generate a 16-hex challenge token (provenance proof)
//  3. Capture git state (HEAD + tree-diff sha256) for audit-binding
//  4. Compose the prompt — caller supplies body, subagent prepends the
//     challenge-token + artifact-path context block
//  5. Call core.Bridge.Launch() (the bridge binary internally handles
//     sandbox-exec/bwrap wrapping per profile.sandbox; the Go runner
//     does not double-wrap)
//  6. Verify the artifact: exists, non-empty, age < 300s, contains
//     challenge token
//  7. Append a kind=agent_subprocess ledger entry
//
// Out of scope for v11.5.0 M2 (deferred to later milestones):
//   - parallel sibling fan-out (cmd_dispatch_parallel in bash)
//   - phase-observer spawn/reap
//   - cache-prefix v2 prompt rewriter
//   - dispatch-plan log emission
//   - fast-fail consecutive-failure counter
//
// These remain available via bash subagent-run.sh while the Go path
// expands.
package subagent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Request is the typed input to Runner.Run. Caller is responsible for
// supplying the persona task body (Prompt); the runner prepends a
// CHALLENGE TOKEN block so the LLM knows it must echo the token in the
// artifact's first line.
type Request struct {
	Agent       string            // intent | scout | tdd-engineer | builder | auditor | …
	Cycle       int               // current cycle number; appears in artifact path + ledger
	ProjectRoot string            // writable project root (host repo)
	PluginRoot  string            // immutable plugin root (profiles, prompts live here)
	Workspace   string            // .evolve/runs/cycle-N/ — bridge writes outputs here
	Worktree    string            // optional per-cycle worktree
	Prompt      string            // user task body; runner prepends context block
	Model       string            // optional model override; empty → profile default
	CLI         string            // optional CLI override; empty → profile default
	Env         map[string]string // propagated to bridge subprocess
}

// Result captures everything Run() observed. The LedgerEntry is the
// exact line that was appended (so callers can inspect entry_seq etc.).
type Result struct {
	Verdict        string   // PASS | FAIL | INTEGRITY_FAIL
	ArtifactPath   string   // resolved absolute path
	ArtifactSHA256 string   // sha256 of artifact bytes (empty if missing)
	ChallengeToken string   // 16-hex token embedded in prompt + artifact
	CostUSD        float64  // from bridge response
	DurationMS     int64    // wall-clock duration including bridge launch
	ExitCode       int      // CLI exit code as reported by bridge
	LedgerEntry    core.LedgerEntry
	Diagnostics    []core.Diagnostic
}

// Verdict constants returned by Run.
const (
	VerdictPASS           = "PASS"
	VerdictFAIL           = "FAIL"
	VerdictIntegrityFail  = "INTEGRITY_FAIL"
)

// ArtifactMaxAge mirrors verify_artifact() at subagent-run.sh:451 — the
// artifact must have been written within the last 5 minutes to be
// considered fresh.
const ArtifactMaxAge = 5 * time.Minute

// ChallengeTokenBytes is the size of the random source used for the
// 16-hex token (8 bytes → 16 hex chars).
const ChallengeTokenBytes = 8

// Config wires in all the injectable seams. Production constructs the
// runner with NewDefault() which fills in real implementations; tests
// supply doubles for Bridge, Ledger, Profiles, Now, Rand, and GitState.
type Config struct {
	Profiles *profiles.Loader
	Bridge   core.Bridge
	Ledger   core.Ledger
	// Now returns the current wall clock. Defaults to time.Now.
	Now func() time.Time
	// Rand returns ChallengeTokenBytes random bytes. Defaults to
	// crypto/rand.Read.
	Rand func([]byte) (int, error)
	// GitState returns ("<head>", "<tree-diff-sha256>", err) for the
	// given project root. Defaults to running `git rev-parse HEAD` +
	// `git diff HEAD | sha256sum`.
	GitState func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	// HashFile returns the sha256 hex of the file at path. Defaults to
	// reading and hashing via sha256.New().
	HashFile func(path string) (string, error)
	// StatMTime returns the modification time of path. Defaults to
	// os.Stat.
	StatMTime func(path string) (time.Time, error)
	// ReadFile returns the bytes at path. Defaults to os.ReadFile.
	ReadFile func(path string) ([]byte, error)
}

// Runner is the subagent dispatcher. Constructed via New() with a fully
// populated Config.
type Runner struct {
	cfg Config
}

// New constructs a Runner. Required fields: Profiles, Bridge, Ledger.
// Other seams default to production implementations.
func New(cfg Config) (*Runner, error) {
	if cfg.Profiles == nil {
		return nil, errors.New("subagent: Profiles required")
	}
	if cfg.Bridge == nil {
		return nil, errors.New("subagent: Bridge required")
	}
	if cfg.Ledger == nil {
		return nil, errors.New("subagent: Ledger required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Rand == nil {
		cfg.Rand = rand.Read
	}
	if cfg.GitState == nil {
		cfg.GitState = defaultGitState
	}
	if cfg.HashFile == nil {
		cfg.HashFile = defaultHashFile
	}
	if cfg.StatMTime == nil {
		cfg.StatMTime = defaultStatMTime
	}
	if cfg.ReadFile == nil {
		cfg.ReadFile = os.ReadFile
	}
	return &Runner{cfg: cfg}, nil
}

// Run drives one subagent invocation end-to-end. The returned Result
// always carries the ledger entry, even on failure — the bash equivalent
// always writes a ledger entry so post-mortem analysis has provenance.
func (r *Runner) Run(ctx context.Context, req Request) (Result, error) {
	if err := validateRequest(req); err != nil {
		return Result{}, err
	}

	prof, err := r.cfg.Profiles.Get(req.Agent)
	if err != nil {
		return Result{}, fmt.Errorf("subagent: load profile %q: %w", req.Agent, err)
	}

	token, err := r.generateToken()
	if err != nil {
		return Result{}, fmt.Errorf("subagent: generate challenge token: %w", err)
	}

	gitHead, treeDiff, err := r.cfg.GitState(ctx, req.ProjectRoot)
	if err != nil {
		// Non-fatal: git state is "unknown:unknown" so the ledger entry
		// still records what we have. Matches bash subagent-run.sh:281.
		gitHead, treeDiff = "unknown", "unknown"
	}

	artifactPath := resolveArtifactPath(prof.OutputArtifact, req.Cycle, req.ProjectRoot)
	if artifactPath == "" {
		return Result{}, fmt.Errorf("subagent: profile %q has no output_artifact", req.Agent)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("subagent: prepare artifact dir: %w", err)
	}

	cli := req.CLI
	if cli == "" {
		cli = prof.CLI
	}
	if cli == "" {
		cli = "claude-p"
	}
	model := req.Model
	if model == "" {
		model = prof.ModelTierDefault
	}
	if model == "" {
		model = "auto"
	}

	profilePath := filepath.Join(req.PluginRoot, ".evolve", "profiles", req.Agent+".json")
	if req.PluginRoot == "" {
		profilePath = filepath.Join(req.ProjectRoot, ".evolve", "profiles", req.Agent+".json")
	}

	fullPrompt := composePrompt(req.Prompt, token, artifactPath, req.Agent, req.Cycle)

	start := r.cfg.Now()
	bres, bridgeErr := r.cfg.Bridge.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      profilePath,
		Model:        model,
		Prompt:       fullPrompt,
		Workspace:    req.Workspace,
		Worktree:     req.Worktree,
		ArtifactPath: artifactPath,
		Agent:        req.Agent,
		Cycle:        req.Cycle,
		Env:          req.Env,
	})
	durationMS := r.cfg.Now().Sub(start).Milliseconds()

	res := Result{
		ArtifactPath:   artifactPath,
		ChallengeToken: token,
		CostUSD:        bres.CostUSD,
		DurationMS:     durationMS,
		ExitCode:       bres.ExitCode,
	}

	verdict, diagnostics := r.classify(bridgeErr, artifactPath, token, bres.ExitCode)
	res.Verdict = verdict
	res.Diagnostics = diagnostics

	if sha, hashErr := r.cfg.HashFile(artifactPath); hashErr == nil {
		res.ArtifactSHA256 = sha
	}

	entry := core.LedgerEntry{
		TS:             r.cfg.Now().UTC().Format(time.RFC3339),
		Cycle:          req.Cycle,
		Role:           req.Agent,
		Kind:           "agent_subprocess",
		Model:          model,
		ExitCode:       bres.ExitCode,
		DurationS:      strconv.FormatInt(durationMS/1000, 10),
		ArtifactPath:   artifactPath,
		ArtifactSHA256: res.ArtifactSHA256,
		ChallengeToken: token,
		GitHEAD:        gitHead,
		TreeStateSHA:   treeDiff,
	}
	if ledgerErr := r.cfg.Ledger.Append(ctx, entry); ledgerErr != nil {
		// Surface ledger-append failure as a diagnostic but don't shadow
		// the upstream verdict. The bash equivalent treats this as a
		// hard fail; we follow.
		res.Diagnostics = append(res.Diagnostics, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("ledger append: %v", ledgerErr),
		})
		return res, fmt.Errorf("subagent: ledger append: %w", ledgerErr)
	}
	res.LedgerEntry = entry

	if bridgeErr != nil {
		return res, fmt.Errorf("subagent: bridge: %w", bridgeErr)
	}
	return res, nil
}

func validateRequest(req Request) error {
	switch "" {
	case req.Agent:
		return errors.New("subagent: Agent required")
	case req.ProjectRoot:
		return errors.New("subagent: ProjectRoot required")
	case req.Workspace:
		return errors.New("subagent: Workspace required")
	case req.Prompt:
		return errors.New("subagent: Prompt required")
	}
	if req.Cycle < 0 {
		return fmt.Errorf("subagent: Cycle must be >= 0, got %d", req.Cycle)
	}
	return nil
}

// classify is the integrity check ported from verify_artifact() at
// subagent-run.sh:451. Returns (verdict, diagnostics). When bridgeErr is
// non-nil, the artifact may not exist — the verdict is FAIL with a
// bridge-error diagnostic.
func (r *Runner) classify(bridgeErr error, artifactPath, token string, exitCode int) (string, []core.Diagnostic) {
	var diags []core.Diagnostic
	if bridgeErr != nil {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("bridge launch failed (exit=%d): %v", exitCode, bridgeErr),
		})
	}

	info, statErr := r.cfg.StatMTime(artifactPath)
	if statErr != nil {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("artifact missing: %s", artifactPath),
		})
		return VerdictIntegrityFail, diags
	}
	age := r.cfg.Now().Sub(info)
	if age > ArtifactMaxAge {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("artifact stale (%s old): %s", age.Round(time.Second), artifactPath),
		})
		return VerdictIntegrityFail, diags
	}

	body, readErr := r.cfg.ReadFile(artifactPath)
	if readErr != nil {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("artifact unreadable: %v", readErr),
		})
		return VerdictIntegrityFail, diags
	}
	if len(body) == 0 {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("artifact empty: %s", artifactPath),
		})
		return VerdictIntegrityFail, diags
	}
	if !strings.Contains(string(body), token) {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("challenge token %q missing from artifact", token),
		})
		return VerdictIntegrityFail, diags
	}
	if bridgeErr != nil || exitCode != 0 {
		return VerdictFAIL, diags
	}
	return VerdictPASS, diags
}

// generateToken returns 16 lowercase hex chars (8 random bytes encoded).
// Mirrors generate_challenge_token() at subagent-run.sh:263.
func (r *Runner) generateToken() (string, error) {
	buf := make([]byte, ChallengeTokenBytes)
	n, err := r.cfg.Rand(buf)
	if err != nil {
		return "", err
	}
	if n != ChallengeTokenBytes {
		return "", fmt.Errorf("subagent: rand returned %d bytes, want %d", n, ChallengeTokenBytes)
	}
	return hex.EncodeToString(buf), nil
}

// resolveArtifactPath expands {cycle} in the profile's output_artifact
// template and returns an absolute path under projectRoot. Returns ""
// when the template is empty (profile has no defined artifact).
func resolveArtifactPath(template string, cycle int, projectRoot string) string {
	if template == "" {
		return ""
	}
	expanded := strings.ReplaceAll(template, "{cycle}", strconv.Itoa(cycle))
	if filepath.IsAbs(expanded) {
		return expanded
	}
	return filepath.Join(projectRoot, expanded)
}

// composePrompt prepends the CHALLENGE TOKEN context block to the user
// prompt. Order matches subagent-run.sh:803-818 — token first, artifact
// path second, then the task prompt body.
//
// v11.5.2 fix: section markers use `## ... ##` not `--- ... ---`.
// claude CLI 2.1.149's flag parser rejects any prompt value whose first
// argv-character is `-` ("unknown option" error), and the bridge
// driver passes the composed prompt as `-p "$prompt_content"` — so a
// leading `--` prefix tripped the parser. Switching to `## ... ##`
// keeps the visual section-marker convention while avoiding the
// flag-parser collision. The fix is structural: any future prompt
// content the runner prepends must not start with `--`.
func composePrompt(body, token, artifactPath, agent string, cycle int) string {
	var b strings.Builder
	b.WriteString("## INVOCATION CONTEXT ##\n")
	fmt.Fprintf(&b, "Agent: %s\n", agent)
	fmt.Fprintf(&b, "Cycle: %d\n", cycle)
	fmt.Fprintf(&b, "Challenge token: %s\n", token)
	fmt.Fprintf(&b, "Artifact path: %s\n", artifactPath)
	b.WriteString("\n")
	b.WriteString("Your output artifact MUST be written to the artifact path above.\n")
	b.WriteString("The first line of that file MUST contain the challenge token.\n")
	fmt.Fprintf(&b, "(Suggested header: \"<!-- challenge-token: %s -->\")\n", token)
	b.WriteString("\n## BEGIN TASK PROMPT ##\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("## END TASK PROMPT ##\n")
	return b.String()
}

// defaultGitState shells `git rev-parse HEAD` + `git diff HEAD` and
// returns the sha256 of the diff. Mirrors capture_git_state() at
// subagent-run.sh:269. Errors collapse to ("unknown", "unknown", err) so
// callers can log the failure without losing the rest of the entry.
func defaultGitState(ctx context.Context, projectRoot string) (string, string, error) {
	head, err := runGit(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		return "unknown", "unknown", err
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(head), "unknown", err
	}
	sum := sha256.Sum256(out)
	return strings.TrimSpace(head), hex.EncodeToString(sum[:]), nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

// defaultHashFile streams the file at path through sha256 and returns
// the hex digest. Empty path or missing file returns ("", err).
func defaultHashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func defaultStatMTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
