package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

const hostLoadavgPath = "/proc/loadavg"

type hostProbe func(context.Context) (float64, error)

var hostReleaseScope = map[hostsentinel.Metric]string{
	hostsentinel.MetricDiskFree: gcScopeCache,
	hostsentinel.MetricGoCache:  gcScopeCache,
	hostsentinel.MetricOrphans:  gcScopeOrphans,
}

func startHostSentinel(ctx context.Context, cfg loopConfig, signals *signalcenter.Center, stderr io.Writer) (*hostsentinel.Sentinel, func()) {
	host := newHostSentinel(cfg, signals, stderr)
	if host == nil {
		return nil, func() {}
	}
	return host, host.Start(ctx)
}

func newHostSentinel(cfg loopConfig, signals *signalcenter.Center, stderr io.Writer) *hostsentinel.Sentinel {
	pol := policy.LoadOrDefaults(filepath.Join(cfg.EvolveDir, "policy.json"), stderr)
	gcPol := gcBlock(pol)
	block := hostsentinel.Policy{}
	if pol.HostSentinel != nil {
		block = *pol.HostSentinel
	}
	hc, err := block.Config(gcPol.GoCacheMaxBytes())
	if err != nil {
		fmt.Fprintf(stderr, "[host] WARN: %v; the host sentinel does not run\n", err)
		return nil
	}
	if hc.Stage == hostsentinel.StageOff {
		return nil
	}
	return hostsentinel.New(hc, hostsentinel.Ports{
		Sample:  sampleHost(hostProbes(cfg.ProjectRoot, gcPol)),
		Release: hostRelease(cfg.ProjectRoot),
		Emit:    signals.Emit,
		Save:    func(st hostsentinel.Status) error { return hostsentinel.WriteStatus(cfg.EvolveDir, st) },
		Warn:    func(m string) { fmt.Fprintf(stderr, "[host] WARN: %s\n", m) },
	})
}

func hostProbes(projectRoot string, gcPol gc.Policy) map[hostsentinel.Metric]hostProbe {
	opts := worktreeGCOptions(projectRoot, filepath.Join(projectRoot, ".evolve"), gcPol.Worktrees, io.Discard)
	return map[hostsentinel.Metric]hostProbe{
		hostsentinel.MetricDiskFree: func(context.Context) (float64, error) {
			free, err := looppreflight.DiskFreeBytes(projectRoot)
			return float64(free), err
		},
		hostsentinel.MetricGoCache: probeGoCache,
		hostsentinel.MetricTmux: func(ctx context.Context) (float64, error) {
			sessions, err := swarm.ExecListBridgeSessions(ctx)
			return float64(len(sessions)), err
		},
		hostsentinel.MetricOrphans: probeOrphans(opts),
		hostsentinel.MetricLoad:    probeLoad(hostLoadavgPath),
	}
}

func sampleHost(probes map[hostsentinel.Metric]hostProbe) func(context.Context) hostsentinel.Sample {
	return func(ctx context.Context) hostsentinel.Sample {
		s := hostsentinel.Sample{At: time.Now(), Readings: map[hostsentinel.Metric]hostsentinel.Reading{}}
		for metric, probe := range probes {
			s.Readings[metric] = hostsentinel.Measured(probe(ctx))
		}
		return s
	}
}

func probeGoCache(ctx context.Context) (float64, error) {
	dir, err := gcGoCacheDir(ctx)
	if err != nil {
		return 0, err
	}
	n, err := gc.GoCacheBytes(dir)
	return float64(n), err
}

func probeOrphans(opts gc.WorktreeOptions) hostProbe {
	return func(ctx context.Context) (float64, error) {
		procs, err := proctree.ListCwds(ctx, opts.Exec, opts.ProjectRoot)
		return float64(len(opts.FinishedCycleOrphans(procs))), err
	}
}

func probeLoad(path string) hostProbe {
	return func(context.Context) (float64, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		return hostsentinel.ParseLoadavg(data, runtime.NumCPU())
	}
}

func hostScopes(breaches []hostsentinel.Breach) []string {
	var scopes []string
	seen := map[string]bool{}
	for _, b := range breaches {
		if scope, ok := hostReleaseScope[b.Metric]; ok && !seen[scope] {
			seen[scope] = true
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func hostRelease(projectRoot string) func(context.Context, []hostsentinel.Breach, bool) []hostsentinel.Release {
	return func(_ context.Context, breaches []hostsentinel.Breach, apply bool) []hostsentinel.Release {
		var out []hostsentinel.Release
		for _, scope := range hostScopes(breaches) {
			out = append(out, runHostScope(projectRoot, scope, apply))
		}
		return out
	}
}

func runHostScope(projectRoot, scope string, apply bool) hostsentinel.Release {
	args := []string{"--only", scope, "--project-root", projectRoot}
	if !apply {
		args = append(args, "--dry-run")
	}
	var out strings.Builder
	rc := runGC(args, nil, &out, &out)
	return hostsentinel.Release{
		Scope:     scope,
		Applied:   apply && rc == cliExitOK,
		LeaseHeld: rc == cliExitRefused,
		Report:    strings.FieldsFunc(out.String(), func(r rune) bool { return r == '\n' }),
	}
}

func hostGatedLaunch(host *hostsentinel.Sentinel, launch fleet.LaunchFn) fleet.LaunchFn {
	return func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		if host.HoldLanes() {
			return fleet.ExitDeferred, nil
		}
		return launch(ctx, spec)
	}
}
