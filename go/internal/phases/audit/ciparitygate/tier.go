package ciparitygate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const tierParallelism = 4

func tierArgs(pkgs []string) []string {
	p := strconv.Itoa(tierParallelism)
	return append([]string{"test", "-race", "-count=1", "-p", p, "-parallel", p, "-tags", "integration"}, pkgs...)
}

type attempt struct {
	out, errOut string
	code        int
	err         error
	deadlineHit bool
}

func runAttempt(ctx context.Context, run sysexec.RunFunc, dir string, args []string) attempt {
	out, errOut, code, err := sysexec.Capture(ctx, run, dir, "go", args...)
	return attempt{out: out, errOut: errOut, code: code, err: err, deadlineHit: errors.Is(ctx.Err(), context.DeadlineExceeded)}
}

func (g *Gates) IntegrationTier(req Request) ([]string, error) {
	changed, shouldRun, scopeErr := g.scope(gateTier, req)
	if scopeErr != nil || !shouldRun {
		return nil, scopeErr
	}
	dir := moduleDir(req.root())
	ctx, cancel := context.WithTimeout(context.Background(), g.timeouts.TierAttempt)
	defer cancel()
	pkgs, err := g.tierPackages(ctx, req, dir, changed)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, nil
	}
	args := tierArgs(pkgs)
	scrubbed := scrubbedRun(g.run)
	first := runAttempt(ctx, scrubbed, dir, args)
	if first.err != nil {
		wrapped := fmt.Errorf("integration-tier gate could not run: %w", first.err)
		g.stepFailed(gateTier, req, stepTierAttempt, wrapped.Error(), first.err, "cmd", "go "+strings.Join(args, " "))
		return nil, wrapped
	}
	if first.code == 0 {
		return nil, nil
	}
	return g.retake(req, dir, args, scrubbed, first)
}

func (g *Gates) tierPackages(ctx context.Context, req Request, dir string, changed []string) ([]string, error) {
	scoped, envExclusive, wholeModule := tierScope(changed)
	if wholeModule {
		pkgs, err := g.wholeSuite(ctx, dir)
		if err != nil {
			g.stepFailed(gateTier, req, stepTierList, err.Error(), err, "cmd", "go list ./...")
		}
		return pkgs, err
	}
	if len(envExclusive) == 0 {
		return scoped, nil
	}
	skipped, backstop := strings.Join(envExclusive, ", "), envExclusiveBackstopNote(envExclusive)
	if len(scoped) == 0 {
		err := fmt.Errorf("touched package(s) %s are env-exclusive under a live loop. Backstop: %s (ADR-0069)", skipped, backstop)
		g.warn(gateTier, req, CodeTierEnvExclusiveSkipped, err.Error(), "scope", "all", "pkgs", skipped, "remainder", "0", "backstop", backstop)
		return nil, err
	}
	g.warn(gateTier, req, CodeTierEnvExclusiveSkipped, "skipping env-exclusive package(s) under a live loop: "+skipped+". Backstop: "+backstop,
		"scope", "mixed", "pkgs", skipped, "remainder", strconv.Itoa(len(scoped)), "backstop", backstop)
	return scoped, nil
}

func (g *Gates) retake(req Request, dir string, args []string, run sysexec.RunFunc, first attempt) ([]string, error) {
	log := &tierLog{workspace: req.Workspace, args: args}
	g.logAttempt(req, log, 1, " (lane env, contended)", first)
	release, lockNote := g.acquireLock(gateTier, req)
	retakeCtx, retakeCancel := context.WithTimeout(context.Background(), g.timeouts.TierAttempt)
	second := runAttempt(retakeCtx, run, dir, args)
	retakeCancel()
	release()
	if second.err == nil {
		g.logAttempt(req, log, 2, " (serialized retake"+lockNote+")", second)
	}
	d := decideTier(first, second, log.path, g.timeouts.TierAttempt)
	switch d.code {
	case CodeTierRetakeExecFailed:
		g.warn(gateTier, req, d.code, "serialized retake could not run: "+second.err.Error(), "err", second.err.Error(), "log", log.path)
	case CodeTierFlakeAbsorbed:
		g.warn(gateTier, req, d.code, d.err.Error(), "attempt1_exit", strconv.Itoa(first.code), "log", log.path, "lock_note", lockNote)
	case CodeTierDeadlineNoVerdict:
		g.warn(gateTier, req, d.code, d.err.Error(), "budget", g.timeouts.TierAttempt.String(), "log", log.path, "lock_note", lockNote)
	}
	if d.err != nil {
		return nil, d.err
	}
	return g.failed(gateTier, req, d.cause, d.offenders, "log", log.path), nil
}

type tierDecision struct {
	offenders []string
	err       error
	code      signalcenter.Code
	cause     gateCause
}

func decideTier(first, second attempt, logPath string, budget time.Duration) tierDecision {
	if second.err != nil {
		return tierDecision{offenders: offendersWithLogPointer(first, logPath), code: CodeTierRetakeExecFailed, cause: causeRetakeExecFailed}
	}
	if second.code == 0 {
		return tierDecision{code: CodeTierFlakeAbsorbed, err: fmt.Errorf("integration tier was RED under fleet contention but GREEN on a serialized clean-env retake — contention flake absorbed, not a code defect (%s)", tierWhere(logPath))}
	}
	if second.deadlineHit {
		if hasOffenderMarker(second.out + "\n" + second.errOut) {
			return tierDecision{offenders: offendersWithLogPointer(second, logPath), cause: causeDeadlineWithMarkers}
		}
		return tierDecision{code: CodeTierDeadlineNoVerdict, err: fmt.Errorf("integration tier exceeded its %s budget on the serialized retake with no test verdict in the truncated output — a deadline kill is not a judgment; degraded to WARN (%s)", budget, tierWhere(logPath))}
	}
	return tierDecision{offenders: offendersWithLogPointer(second, logPath), cause: causeRetakeRed}
}

func tierWhere(logPath string) string {
	if logPath != "" {
		return "both attempts: " + logPath
	}
	return "integration-tier.log unavailable"
}
