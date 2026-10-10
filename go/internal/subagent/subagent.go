// Package subagent dispatches one subagent invocation: profile load,
// challenge-token compose, bridge launch, artifact verify and ledger
// append. See docs/architecture/packages/internal-subagent.md.
package subagent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Request struct {
	Agent       string
	Cycle       int
	ProjectRoot string
	PluginRoot  string
	Workspace   string
	Worktree    string
	Prompt      string
	Model       string
	CLI         string
	Env         map[string]string
}

type Result struct {
	Verdict        string
	ArtifactPath   string
	ArtifactSHA256 string
	ChallengeToken string
	CostUSD        float64
	Tokens         core.TokenUsage
	DurationMS     int64
	ExitCode       int
	LedgerEntry    core.LedgerEntry
	Diagnostics    []core.Diagnostic
}

const (
	VerdictPASS          = subagentrun.VerdictPASS
	VerdictFAIL          = subagentrun.VerdictFAIL
	VerdictIntegrityFail = subagentrun.VerdictIntegrityFail
)

const ArtifactMaxAge = subagentrun.ArtifactMaxAge

const ChallengeTokenBytes = subagentrun.ChallengeTokenBytes

type Config struct {
	Profiles  *profiles.Loader
	Bridge    core.Bridge
	Ledger    core.Ledger
	Now       func() time.Time
	Rand      func([]byte) (int, error)
	GitState  func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	HashFile  func(path string) (string, error)
	StatMTime func(path string) (time.Time, error)
	ReadFile  func(path string) ([]byte, error)
}

type Runner struct {
	cfg Config
}

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
		cli = "claude-tmux"
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

	overlaySkills := policy.ResolveLaunchOverlaysFailOpen(req.ProjectRoot, req.Agent, cli, model)

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
		Skills:       overlaySkills,
	})
	durationMS := r.cfg.Now().Sub(start).Milliseconds()

	res := Result{
		ArtifactPath:   artifactPath,
		ChallengeToken: token,
		CostUSD:        bres.CostUSD,
		Tokens:         bres.Tokens,
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
		RunID:          core.RunIDFromWorkspace(req.Workspace),
	}
	if ledgerErr := r.cfg.Ledger.Append(ctx, entry); ledgerErr != nil {
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

func (r *Runner) classify(bridgeErr error, artifactPath, token string, exitCode int) (string, []core.Diagnostic) {
	res := VerifyArtifact(r.cfg.StatMTime, r.cfg.ReadFile, r.cfg.Now, artifactPath, token, exitCode, bridgeErr)
	return res.Verdict, res.Diagnostics
}

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

func resolveArtifactPath(template string, cycle int, projectRoot string) string {
	return subagentrun.ResolveArtifactPath(template, cycle, projectRoot)
}

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

func defaultGitState(ctx context.Context, projectRoot string) (string, string, error) {
	head, err := runGit(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		return "unknown", "unknown", err
	}
	cmd := sysexec.Command(ctx, "git", "diff", "HEAD")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(head), "unknown", err
	}
	sum := sha256.Sum256(out)
	return strings.TrimSpace(head), hex.EncodeToString(sum[:]), nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := sysexec.Command(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func defaultHashFile(path string) (string, error) { return subagentrun.HashFile(path) }

func defaultStatMTime(path string) (time.Time, error) { return subagentrun.StatMTime(path) }
