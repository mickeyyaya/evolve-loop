package bridge

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	logShowPath     = sandbox.LogShowPath
	sandboxExecPath = sandbox.SandboxExecPath
)

func (deps Deps) readStage() string {
	return deps.isolationStage(deps.ReadStage, policy.Policy.IsolationReadStage)
}

func (deps Deps) readIsolation(req SandboxWrapRequest, home string) sandbox.ReadIsolation {
	stage := deps.readStage()
	if stage == envStageOff || req.Workspace == "" || req.RepoRoot == "" {
		return sandbox.ReadIsolation{}
	}
	ri := sandbox.ReadIsolation{
		Enforce: stage == envStageEnforce, Tag: req.Tag, RawSuffixes: inputindex.RawOutputSuffixes(),
		Workspace: canonicalOrSelf(req.Workspace),
	}
	if runs := canonicalOrSelf(filepath.Join(req.RepoRoot, ".evolve", "runs")); isDir(runs) {
		ri.RunsDir = runs
	}
	if bridgeDir := filepath.Join(ri.Workspace, ".bridge"); isDir(bridgeDir) {
		ri.RawDirs = []string{bridgeDir}
	}
	ri.Grants, ri.ReportOnly = deps.indexGrants(req, ri.RunsDir)
	ri.RawFiles = rawFilesOf(ri.Workspace)
	ri.SecretDirs, ri.SecretFiles = secretsOf(home)
	ri.PinnedFiles = []string{filepath.Join(ri.Workspace, committedset.LanePinFile)}
	return ri
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalOrSelf(path string) string {
	if real, err := canonicalSandboxPath(path); err == nil {
		return real
	}
	return path
}

func (deps Deps) indexGrants(req SandboxWrapRequest, runsDir string) (grants, reportOnly []string) {
	paths, err := inputindex.ReadablePaths(req.RepoRoot, req.Workspace)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) && deps.Stderr != nil {
			fmt.Fprintf(deps.Stderr, "[bridge] WARN input index of %s: %v (the cell gets no index grants)\n", req.Phase, err)
		}
		return nil, nil
	}
	root := canonicalOrSelf(req.RepoRoot)
	grants = make([]string, 0, len(paths))
	for _, p := range paths {
		real := canonicalOrSelf(p)
		switch {
		case inputindex.IsRawOutput(real):
		case runsDir != "" && within(runsDir, real):
			grants = append(grants, real)
		case runsDir != "" && within(root, real):
			reportOnly = append(reportOnly, real)
		}
	}
	return grants, reportOnly
}

func rawFilesOf(workspace string) []string {
	entries, _ := os.ReadDir(workspace)
	var out []string
	for _, e := range entries {
		if !e.IsDir() && inputindex.IsRawOutput(e.Name()) {
			out = append(out, filepath.Join(workspace, e.Name()))
		}
	}
	return out
}

func secretsOf(home string) (dirs, files []string) {
	if home == "" {
		return nil, nil
	}
	for _, rel := range sandbox.HomeSecrets {
		path := canonicalOrSelf(filepath.Join(home, rel))
		info, err := os.Stat(path)
		switch {
		case err != nil:
		case info.IsDir():
			dirs = append(dirs, path)
		default:
			files = append(files, path)
		}
	}
	return dirs, files
}

var cellTag = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)

func (deps Deps) reportReadViolations(ctx context.Context, cfg *Config, since time.Time) {
	stage := deps.readStage()
	if !deps.readReportsApply(cfg) || stage == envStageOff {
		deps.recordContextAccess(cfg, nil, inputindex.AccessSourceNone)
		return
	}
	raw, complete, err := deps.cellReports(ctx, cfg.DispatchID, since)
	if err != nil {
		deps.warnReports("%v", err)
		deps.recordContextAccess(cfg, nil, inputindex.AccessSourceNone)
		return
	}
	for _, r := range sandbox.ParseReports(raw) {
		deps.emitReadViolation(cfg, stage, r)
	}
	source := inputindex.AccessSourceNone
	if complete {
		source = inputindex.AccessSourceSandbox
	}
	deps.recordContextAccess(cfg, parseGrantReports(raw), source)
}

func (deps Deps) warnReports(format string, args ...any) {
	if deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN isolation read reports: "+format+"\n", args...)
	}
}

func (deps Deps) cellReports(parent context.Context, tag string, since time.Time) ([]byte, bool, error) {
	return sandbox.ReportSource{
		Reports: deps.SandboxReports, Fence: deps.SandboxFence, Now: deps.Now, Sleep: deps.Sleep,
		Deadline: deps.ReportFenceDeadline, Warn: deps.warnReports,
	}.Cell(parent, tag, since)
}

func sandboxFence(binary string) func(context.Context, string) error {
	return func(ctx context.Context, tag string) error { return sandbox.WriteFence(ctx, binary, tag) }
}

func (deps Deps) readReportsApply(cfg *Config) bool {
	sandboxed := cfg.Worktree != "" || cfg.RequireSandbox
	return deps.SandboxReports != nil && cmp.Or(deps.hostOS, runtime.GOOS) == "darwin" && cellTag.MatchString(cfg.DispatchID) && cfg.Workspace != "" &&
		sandboxed && strings.TrimSpace(deps.SandboxMode) != config.SandboxModeOff
}

func relToProject(root, target string) string {
	if root == "" {
		return target
	}
	canonical := canonicalOrSelf(root)
	if !filepath.IsAbs(target) || !within(canonical, target) {
		return target
	}
	rel, _ := filepath.Rel(canonical, target)
	return filepath.ToSlash(rel)
}

func (deps Deps) emitReadViolation(cfg *Config, stage string, r sandbox.Report) {
	id := configIdentity(cfg)
	action := "logged"
	if r.Denied {
		action = "refused"
	}
	target := relToProject(cfg.ProjectRoot, r.Target)
	n, _ := strconv.Atoi(strings.TrimPrefix(r.Invariant, "I"))
	deps.Signals.Emit(signalcenter.Event{
		Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleIsolation, Origin: "bridge.reportReadViolations",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(n),
		Reason: "the cell " + cfg.Agent + " read " + target + " outside its grants",
		Fields: map[string]string{
			"invariant": r.Invariant, "stage": stage, "action": action, "cell": cfg.Agent, "family": cfg.CLI,
			"target": target, "rule": r.Rule, "source": "sbpl-report", "process": r.Process, "pid": r.PID,
		},
	})
}

func logShowReports(binary string) func(context.Context, string, time.Time) ([]byte, error) {
	return func(ctx context.Context, tag string, since time.Time) ([]byte, error) {
		return sandbox.Reports(ctx, binary, tag, since)
	}
}
