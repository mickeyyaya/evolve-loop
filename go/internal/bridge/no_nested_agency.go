package bridge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var ErrSandboxOffInPhase = errors.New("EVOLVE_SANDBOX=off is refused inside a phase: a phase agent must not start an unconfined agent")

func (deps Deps) refuseSandboxOffInPhase(cfg *Config) error {
	phase, _ := lookupEnv(deps, ipcenv.PhaseKey)
	mode, _ := lookupEnv(deps, envSandboxMode)
	if phase == "" || strings.TrimSpace(mode) != config.SandboxModeOff {
		return nil
	}
	id := configIdentity(cfg)
	deps.Signals.Emit(signalcenter.Event{
		Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleIsolation, Origin: "bridge.launchSwept",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(3),
		Reason: "the phase " + phase + " launched " + cfg.CLI + " with " + envSandboxMode + "=off",
		Fields: map[string]string{
			"invariant": "I3", "stage": envStageEnforce, "action": "refused", "cell": cfg.Agent, "family": cfg.CLI, "phase": phase,
			"target": envSandboxMode, "rule": "sandbox.off.phase", "source": "launcher",
		},
	})
	return fmt.Errorf("%w (phase %s, cell %s)", ErrSandboxOffInPhase, phase, cfg.Agent)
}

func (deps Deps) execDeny(cli, goos string) ([]string, []string, bool) {
	stage := deps.execStage()
	if stage == envStageOff {
		return nil, nil, false
	}
	own := ""
	if goos == "linux" {
		own = manifestBinary(cli)
	}
	files, roots := deps.execDenyTargets(own)
	return files, roots, stage == envStageEnforce
}

func manifestBinary(cli string) string {
	m, err := loadManifestRaw(cli)
	if err != nil {
		return ""
	}
	return m.Binary
}

func (deps Deps) execDenyTargets(skip string) (files, roots []string) {
	lookPath := deps.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	home, _ := os.UserHomeDir()
	for _, name := range DriverNames() {
		m, err := loadManifestRaw(name)
		if err != nil || m.Binary == skip {
			continue
		}
		if found, err := lookPath(m.Binary); err == nil {
			files = appendRealPath(files, found)
		}
		for _, root := range m.InstallRoots {
			roots = appendRealPath(roots, expandHomeRoot(root, home))
		}
	}
	return files, roots
}

func expandHomeRoot(root, home string) string {
	rel, underHome := strings.CutPrefix(root, "~/")
	switch {
	case !underHome:
		return root
	case home == "":
		return ""
	default:
		return filepath.Join(home, rel)
	}
}

func appendRealPath(paths []string, path string) []string {
	if path == "" {
		return paths
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || slices.Contains(paths, real) {
		return paths
	}
	return append(paths, real)
}
