package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var errCycleStartPolicyRefused = errors.New("the cycle start refused policy.json")

var cycleStartDiskUsageFn = looppreflight.DiskUsageOf

type cycleStartHooks struct {
	trim    func() error
	usage   func(string) (looppreflight.DiskUsage, error)
	wait    func(context.Context) error
	signals *signalcenter.Center
}

func buildCycleStartOptions(root, evolveDir string, warn io.Writer, hooks cycleStartHooks) (looppreflight.CycleStartOptions, error) {
	if warn == nil {
		warn = io.Discard
	}
	if evolveDir == "" && root != "" {
		evolveDir = paths.EvolveDirOf(root)
	}
	if root == "" && evolveDir != "" {
		root = filepath.Dir(evolveDir)
	}
	if root == "" && evolveDir == "" {
		root = "."
		evolveDir = paths.EvolveDirOf(".")
	}
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return looppreflight.CycleStartOptions{}, fmt.Errorf("%w: %w", errCycleStartPolicyRefused, err)
	}
	trim := hooks.trim
	if trim == nil {
		trim = cycleStartTrim(evolveDir, gcPolicyOf(pol))
	}
	usage := hooks.usage
	if usage == nil {
		usage = cycleStartDiskUsageFn
	}
	return looppreflight.CycleStartOptions{
		Trim:          trim,
		Guard:         looppreflight.NewDiskGuard(root, looppreflight.DefaultDiskGuardConfig(), usage, warn),
		Wait:          hooks.wait,
		Warn:          warn,
		MaxPause:      pol.PreflightConfig().DiskPauseMax,
		EmitDiskPause: emitHostDiskLow(hooks.signals),
	}, nil
}

func emitHostDiskLow(signals *signalcenter.Center) func(looppreflight.DiskPauseEvent) {
	return func(e looppreflight.DiskPauseEvent) {
		signals.Emit(signalcenter.Event{
			Module: signalcenter.ModuleHost, Origin: "cycleStart.hostDiskLow", Kind: signalcenter.KindLoopWarning,
			Severity: signalcenter.SeverityWarn, Code: signalcenter.CodeHostDiskLow,
			Reason: "free disk space is below the pause line, so the cycle start " + hostDiskLowVerb(e.Outcome),
			Fields: map[string]string{
				"free_pct":  strconv.FormatFloat(e.FreePct, 'f', 1, 64),
				"pause_pct": strconv.FormatFloat(e.PausePct, 'f', 1, 64),
				"path":      e.Path,
				"outcome":   e.Outcome,
			},
		})
	}
}

func hostDiskLowVerb(outcome string) string {
	if outcome == looppreflight.DiskPauseOutcomeGaveUp {
		return "stopped after the maximum pause"
	}
	return "waits"
}

func cycleStartTrim(evolveDir string, pol gc.Policy) func() error {
	return func() error {
		rep := trimTestCache(evolveDir, pol, os.Remove)
		if len(rep.Errors) > 0 {
			return errors.New(strings.Join(rep.Errors, "; "))
		}
		return nil
	}
}

func gcPolicyOf(pol policy.Policy) gc.Policy {
	if pol.GC == nil {
		return gc.Policy{}
	}
	return *pol.GC
}
