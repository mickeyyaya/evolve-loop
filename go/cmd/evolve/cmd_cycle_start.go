package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type cycleStartHooks struct {
	trim  func() error
	usage func(string) (looppreflight.DiskUsage, error)
	wait  func(context.Context) error
}

func buildCycleStartOptions(root, evolveDir string, warn io.Writer, hooks cycleStartHooks) looppreflight.CycleStartOptions {
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
	trim := hooks.trim
	if trim == nil {
		trim = cycleStartTrim(evolveDir, warn)
	}
	guard := looppreflight.NewDiskGuard(root, looppreflight.DefaultDiskGuardConfig(), hooks.usage, warn)
	return looppreflight.CycleStartOptions{
		Trim:  trim,
		Guard: guard,
		Wait:  hooks.wait,
		Warn:  warn,
	}
}

func cycleStartTrim(evolveDir string, warn io.Writer) func() error {
	return func() error {
		pol := loadCycleStartGCPolicy(evolveDir, warn)
		rep := trimTestCache(evolveDir, pol, os.Remove)
		if len(rep.Errors) > 0 {
			return errors.New(strings.Join(rep.Errors, "; "))
		}
		return nil
	}
}

func loadCycleStartGCPolicy(evolveDir string, warn io.Writer) gc.Policy {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(warn, "[cycle-start] WARN: policy load failed: %v; using default gc policy\n", err)
	}
	if pol.GC == nil {
		return gc.Policy{}
	}
	return *pol.GC
}
