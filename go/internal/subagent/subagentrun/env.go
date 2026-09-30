package subagentrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type AdapterEnv struct {
	AdapterPath string

	ProfilePath   string
	ResolvedModel string
	PromptFile    string
	Cycle         int
	WorkspacePath string
	WorktreePath  string
	StdoutLog     string
	StderrLog     string
	ArtifactPath  string

	ResolvedCLI         string
	CLIResolutionSource string
	CapBudgetNative     bool
	ToolsOverride       string
	ExtraFlagsOverride  string
	ChallengeToken      string
	ProjectRoot         string
}

func (e AdapterEnv) Map() map[string]string {
	env := map[string]string{
		"PROFILE_PATH":                 e.ProfilePath,
		"RESOLVED_MODEL":               e.ResolvedModel,
		"PROMPT_FILE":                  e.PromptFile,
		"CYCLE":                        strconv.Itoa(e.Cycle),
		"WORKSPACE_PATH":               e.WorkspacePath,
		"WORKTREE_PATH":                e.WorktreePath,
		"STDOUT_LOG":                   e.StdoutLog,
		"STDERR_LOG":                   e.StderrLog,
		"ARTIFACT_PATH":                e.ArtifactPath,
		"RESOLVED_CLI":                 e.ResolvedCLI,
		"CLI_RESOLUTION_SOURCE":        e.CLIResolutionSource,
		"CAP_BUDGET_NATIVE":            BoolEnv(e.CapBudgetNative),
		"ADAPTER_TOOLS_OVERRIDE":       e.ToolsOverride,
		"ADAPTER_EXTRA_FLAGS_OVERRIDE": e.ExtraFlagsOverride,
		"VALIDATE_ONLY":                "0",
		"CHALLENGE_TOKEN":              e.ChallengeToken,
	}
	if e.ProjectRoot != "" {
		env["EVOLVE_PROJECT_ROOT"] = e.ProjectRoot
	}
	return env
}

func BoolEnv(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func adapterEnv(req Request, p plan, prov provenance, promptPath, worktree string) AdapterEnv {
	tools, extra := p.profile.Overrides(p.cli)
	return AdapterEnv{
		AdapterPath:   p.adapterPath,
		ProfilePath:   p.profilePath,
		ResolvedModel: p.model,
		PromptFile:    promptPath,
		Cycle:         req.Cycle,
		WorkspacePath: req.WorkspacePath,
		WorktreePath:  worktree,
		StdoutLog:     filepath.Join(req.WorkspacePath, req.Agent+".stdout.log"),
		StderrLog:     filepath.Join(req.WorkspacePath, req.Agent+".stderr.log"),
		ArtifactPath:  prov.artifactPath,

		ResolvedCLI:         p.cli,
		CLIResolutionSource: p.source,
		CapBudgetNative:     p.cap.BudgetNative,
		ToolsOverride:       tools,
		ExtraFlagsOverride:  extra,
		ChallengeToken:      prov.token,
		ProjectRoot:         req.ProjectRoot,
	}
}

type Adapter interface {
	Exec(ctx context.Context, env AdapterEnv) (exitCode int, err error)
}

type AdapterFunc func(ctx context.Context, env AdapterEnv) (int, error)

func (f AdapterFunc) Exec(ctx context.Context, env AdapterEnv) (int, error) { return f(ctx, env) }

type PromptStager interface {
	Stage(prompt string) (path string, cleanup func(), err error)
}

type tempFileStager struct {
	create func(dir, pattern string) (*os.File, error)
}

func (s tempFileStager) Stage(prompt string) (string, func(), error) {
	f, err := s.create("", "evolve-subagent-prompt-*.txt")
	if err != nil {
		return "", nil, &stageFault{op: "create", err: fmt.Errorf("subagent/run: prompt tempfile: %w", err)}
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := f.WriteString(prompt); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, &stageFault{op: "write", err: fmt.Errorf("subagent/run: write prompt tempfile: %w", err)}
	}
	_ = f.Close()
	return path, cleanup, nil
}

type stageFault struct {
	op  string
	err error
}

func (f *stageFault) Error() string { return f.err.Error() }
func (f *stageFault) Unwrap() error { return f.err }
