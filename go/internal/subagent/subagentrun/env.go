package subagentrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// AdapterEnv is the typed wire to the bridge exec port: everything the
// adapter receives. Map renders the 16-key env the bash adapter contract
// defined (17 with EVOLVE_PROJECT_ROOT). AdapterPath is the vestigial
// <AdaptersDir>/<cli>.sh argument the host's func-shaped seam still receives;
// it is NOT in the map.
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

// Map reproduces the adapter env byte-for-byte: VALIDATE_ONLY is always "0"
// on this path, and EVOLVE_PROJECT_ROOT is exported only when a project root
// is known — never an empty export, because an unset variable falls back to
// cwd by the subprocess contract (core/phase.go). Headless drivers inherit it
// via driverEnv; the tmux drivers export it into the pane shell themselves.
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

// BoolEnv mirrors bash's "true"/"false" env emission for booleans.
func BoolEnv(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// adapterEnv is step 12's pure assembly of the wire from the plan, the
// provenance, the staged prompt and the effective worktree.
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

// Adapter is the leaf-owned bridge exec port: the exit code the driver
// reported and the infrastructure error, if any (the host's default returns
// -1 on an infrastructure error so an "exit 0 + error" never misleads the
// verification ladder).
type Adapter interface {
	Exec(ctx context.Context, env AdapterEnv) (exitCode int, err error)
}

// AdapterFunc adapts a plain function to the Adapter port.
type AdapterFunc func(ctx context.Context, env AdapterEnv) (int, error)

// Exec calls f.
func (f AdapterFunc) Exec(ctx context.Context, env AdapterEnv) (int, error) { return f(ctx, env) }

// PromptStager materialises the composed prompt for the adapter and returns
// where it is plus the cleanup that removes it after the exec.
type PromptStager interface {
	Stage(prompt string) (path string, cleanup func(), err error)
}

// tempFileStager is the production stager: an os.CreateTemp under the system
// temp dir with the evolve-subagent-prompt-*.txt pattern, one WriteString,
// one Close, removed after the exec — the same syscalls and bytes as before.
// create is injectable so both fault branches are reachable.
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

// stageFault names which staging syscall failed (create | write) so the
// PREPARE_FAILED signal can say so; the text stays the run's own.
type stageFault struct {
	op  string
	err error
}

func (f *stageFault) Error() string { return f.err.Error() }
func (f *stageFault) Unwrap() error { return f.err }
