// Package phaseobserver hosts the standalone `evolve phase-observer` stall detector, driving internal/observerengine.
// See docs/architecture/packages/internal-phaseobserver.md.
package phaseobserver

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	ExitOK          = 0
	ExitInvalidArgs = 10
)

type Scope string

const (
	ScopePhase Scope = observerengine.ScopePhase
	ScopeCycle Scope = observerengine.ScopeCycle
)

type Config struct {
	Workspace    string
	SubagentPGID int
	Cycle        int
	Phase        string
	Agent        string
	CycleState   string
	Scope        Scope
	Enforce      bool

	PollS          int
	StallS         int
	NudgeS         int
	NudgeBody      string
	LoopN          int
	LoopWindowS    int
	ErrorRate      float64
	CostSigma      float64
	ThrottleN      int
	EOFGraceS      int
	HeartbeatEvery int

	MaxNoProgressS int

	StallPolicy recovery.StallPolicy

	ProcessAlive func(pgid int) bool

	Signals func() *signalcenter.Center

	Now         func() time.Time
	KillPgrp    func(pgid int, sig syscall.Signal) error
	ShutdownSig <-chan struct{}
	StopAfterMS int
}

func DefaultProcessAlive(pgid int) bool {
	return syscall.Kill(-pgid, 0) == nil
}

func stopChan(stopTimer *time.Timer) <-chan time.Time {
	if stopTimer != nil {
		return stopTimer.C
	}
	return nil
}

func Run(cfg Config, stdoutPath string, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[phase-observer] "+format+"\n", args...)
	}

	if cfg.Workspace == "" || cfg.Phase == "" || cfg.Agent == "" {
		logf("usage: phase-observer <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
		return ExitInvalidArgs
	}
	if info, err := os.Stat(cfg.Workspace); err != nil || !info.IsDir() {
		logf("workspace not a directory: %s", cfg.Workspace)
		return ExitInvalidArgs
	}
	if cfg.Cycle <= 0 {
		logf("cycle must be integer")
		return ExitInvalidArgs
	}

	cfg = withDefaults(cfg)
	paths := observerengine.PathsFor(cfg.Workspace, cfg.Agent)
	if stdoutPath != "" {
		paths.Stdout = stdoutPath
	}
	eng := wiredEngine(cfg, paths)
	eng.Start()
	pollLoop(cfg, eng)
	_ = eng.WriteReport()
	return ExitOK
}

func withDefaults(cfg Config) Config {
	if cfg.PollS == 0 {
		cfg.PollS = 5
	}
	if cfg.StallS == 0 {
		cfg.StallS = 600
	}
	if cfg.NudgeS > 0 && cfg.NudgeBody == "" {
		cfg.NudgeBody = "You appear stalled. Summarize your current state, then either continue or finalize your artifact."
	}
	if cfg.EOFGraceS == 0 {
		cfg.EOFGraceS = 10
	}
	if cfg.HeartbeatEvery == 0 {
		cfg.HeartbeatEvery = 12
	}
	if cfg.Scope == "" {
		cfg.Scope = ScopePhase
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.KillPgrp == nil {
		cfg.KillPgrp = func(pgid int, sig syscall.Signal) error {
			return syscall.Kill(-pgid, sig)
		}
	}
	return cfg
}

func wiredEngine(cfg Config, paths observerengine.Paths) *observerengine.Engine {
	return observerengine.New(settingsOf(cfg, paths), observerengine.Deps{
		Now:          cfg.Now,
		ProcessAlive: cfg.ProcessAlive,
		Kill:         cfg.KillPgrp,
		Nudge: func(body string) error {
			return inbox.Append(cfg.Workspace, cfg.Agent, inbox.Envelope{
				Kind:   inbox.KindNudge,
				Body:   body,
				Source: "observer",
			}, cfg.Now)
		},
		Policy: cfg.StallPolicy,
	}, observerengine.WithSignals(cfg.Signals))
}

func settingsOf(cfg Config, paths observerengine.Paths) observerengine.Settings {
	return observerengine.Settings{
		Cycle: cfg.Cycle, Phase: cfg.Phase, Agent: cfg.Agent, Scope: string(cfg.Scope),
		Enforce: cfg.Enforce, PGID: cfg.SubagentPGID, PID: os.Getpid(),
		PollS: cfg.PollS, StallS: cfg.StallS, NudgeS: cfg.NudgeS, EOFGraceS: cfg.EOFGraceS,
		HeartbeatEvery: cfg.HeartbeatEvery, MaxNoProgressS: cfg.MaxNoProgressS,
		NudgeBody: cfg.NudgeBody, Paths: paths,
	}
}

func pollLoop(cfg Config, eng *observerengine.Engine) {
	tickerInterval := time.Duration(cfg.PollS) * time.Second
	if cfg.StopAfterMS > 0 {
		tickerInterval = time.Duration(cfg.StopAfterMS) * time.Millisecond / 4
	}
	pollTicker := time.NewTicker(tickerInterval)
	defer pollTicker.Stop()

	var stopTimer *time.Timer
	if cfg.StopAfterMS > 0 {
		stopTimer = time.NewTimer(time.Duration(cfg.StopAfterMS) * time.Millisecond)
	}
	stopCh := stopChan(stopTimer)

	for {
		select {
		case <-cfg.ShutdownSig:
			eng.Shutdown("sigusr1")
			return
		case <-stopCh:
			eng.Shutdown("stop-timer")
			return
		case <-pollTicker.C:
			if eng.Tick() == observerengine.StopEOFGrace {
				eng.Shutdown("eof_grace")
				return
			}
		}
	}
}
