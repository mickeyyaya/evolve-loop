package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	roleIntegration  = "integration"
	batchEventOrigin = "evolve.integrationBatch"
)

type stopKeyRun func(projectRoot string, warn io.Writer) selfheal.KeyRun

type stopEmit func(projectRoot string, console io.Writer, e signalcenter.Event)

func attributedKeyRun(run baseattr.Runner) stopKeyRun {
	return func(projectRoot string, warn io.Writer) selfheal.KeyRun {
		return core.BaseRedAttributor(projectRoot, run, warn).RunAt
	}
}

func emitIntegrationSignal(projectRoot string, console io.Writer, e signalcenter.Event) {
	signals, closeSignals := newRoleSignalCenter(roleIntegration, projectRoot, paths.EvolveDirOf(projectRoot), console)
	defer closeSignals()
	signals.Emit(e)
}

func (e integrationEnv) projectRoot(op, flag string, stderr io.Writer) (string, int) {
	root := paths.AbsoluteRoot("--project-root", chooseRoot(flag, e.ProjectRootEnv, e.Root), nil)
	return root, checkRootDir(op, root, stderr)
}

type stopWiring struct {
	env         integrationEnv
	projectRoot string
	warn        io.Writer
}

func (e integrationEnv) stopGate(projectRoot string, warn io.Writer) integration.StopGate {
	w := stopWiring{env: e, projectRoot: projectRoot, warn: warn}
	hold := w.hold()
	return integration.StopGate{
		Red: func(pr integration.PR) error {
			return hold.Red(fmt.Sprintf("batch pull request %d", pr.Number))
		},
		Admit: func(ctx context.Context, main, tip string) (func(), error) {
			green, err := hold.Admit(ctx, main, tip)
			return w.landed(green, tip), err
		},
	}
}

func (w stopWiring) emit(e signalcenter.Event) {
	w.env.Emit(w.projectRoot, w.warn, e)
}

func (w stopWiring) hold() *selfheal.StopHold {
	evolveDir := paths.EvolveDirOf(w.projectRoot)
	gates := policy.LoadOrDefaults(filepath.Join(evolveDir, "policy.json"), w.warn).GatesConfig()
	return &selfheal.StopHold{
		EvolveDir: evolveDir, Enforced: gates.MainRedStopEnforced(), Warn: w.warn, Emit: w.emit,
		Event: signalcenter.Event{Origin: batchEventOrigin},
		Run: func(ctx context.Context, rev string, reds baseattr.Reds) ([]baseattr.Key, error) {
			return w.env.KeyRun(w.projectRoot, w.warn)(ctx, rev, reds)
		},
	}
}

func (w stopWiring) landed(green []baseattr.Key, tip string) func() {
	release := selfheal.StopRelease{
		EvolveDir: paths.EvolveDirOf(w.projectRoot), Now: w.env.Now, Emit: w.emit,
		Event:   signalcenter.Event{Origin: batchEventOrigin},
		Consume: fixItemCloser(w.projectRoot, w.warn),
	}
	return func() {
		if err := release.Keys(green, "batch main "+tip); err != nil {
			fmt.Fprintf(w.warn, "[integration] WARN main-red stop release: %v\n", err)
		}
	}
}
