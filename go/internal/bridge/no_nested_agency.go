package bridge

import (
	"errors"
	"fmt"
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

func (deps Deps) execDeny(cli, goos string) ([]string, bool) {
	stage := deps.execStage()
	if stage == envStageOff {
		return nil, false
	}
	own := ""
	if goos == "linux" {
		own = manifestBinary(cli)
	}
	return deps.llmBinaryPaths(own), stage == envStageEnforce
}

func manifestBinary(cli string) string {
	m, err := loadManifestRaw(cli)
	if err != nil {
		return ""
	}
	return m.Binary
}

func (deps Deps) llmBinaryPaths(skip string) []string {
	lookPath := deps.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	var out []string
	for _, name := range DriverNames() {
		bin := manifestBinary(name)
		if bin == "" || bin == skip {
			continue
		}
		found, err := lookPath(bin)
		if err != nil {
			continue
		}
		if real, err := filepath.EvalSymlinks(found); err == nil && !slices.Contains(out, real) {
			out = append(out, real)
		}
	}
	return out
}
