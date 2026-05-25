package runner

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// CostGuardOptions configures a CostGuardDecorator. The threshold is
// read from req.Env[ThresholdEnvKey] at call time when non-empty,
// otherwise from DefaultThresholdUSD. Strict mode (StrictEnvKey="1")
// promotes the overrun from advisory diagnostic to FAIL verdict.
//
// Why decorator: the cost-overrun check is cross-cutting — every
// LLM-dispatching phase has a budget. Embedding the check in build's
// Hooks.Classify ties the logic to one phase; wrapping the BaseRunner
// lets any phase opt in by name-spacing its own env-var keys (and
// keeps Classify focused on artifact semantics).
type CostGuardOptions struct {
	// ThresholdEnvKey is the env-var name carrying the per-phase
	// USD threshold (e.g., "EVOLVE_BUILDER_COST_THRESHOLD").
	ThresholdEnvKey string

	// StrictEnvKey is the env-var name that, when set to "1",
	// promotes overruns from warning to FAIL.
	StrictEnvKey string

	// DefaultThresholdUSD is the fallback when ThresholdEnvKey is
	// unset or unparseable.
	DefaultThresholdUSD float64
}

// CostGuardDecorator wraps any core.PhaseRunner and appends a cost-
// overrun diagnostic when the wrapped runner's response carries a
// CostUSD above the configured threshold. In strict mode it also
// promotes the verdict to FAIL.
//
// Pattern: Decorator (GoF). The wrapped runner is treated as a black
// box — only its returned PhaseResponse.CostUSD is examined.
type CostGuardDecorator struct {
	inner core.PhaseRunner
	opts  CostGuardOptions
}

// WithCostGuard wraps the inner runner. Opts must specify all three
// fields; nothing is auto-defaulted because the env-key choice is
// load-bearing for the per-phase namespacing.
func WithCostGuard(inner core.PhaseRunner, opts CostGuardOptions) *CostGuardDecorator {
	return &CostGuardDecorator{inner: inner, opts: opts}
}

// Name delegates to the wrapped runner.
func (d *CostGuardDecorator) Name() string { return d.inner.Name() }

// Run delegates to the wrapped runner, then post-processes the
// response. Wrapped-runner errors flow through unchanged so the
// decorator does not mask transport failures.
func (d *CostGuardDecorator) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	resp, err := d.inner.Run(ctx, req)
	if err != nil {
		return resp, err
	}
	threshold := parseFloatOrDefault(req.Env[d.opts.ThresholdEnvKey], d.opts.DefaultThresholdUSD)
	if resp.CostUSD <= threshold {
		return resp, nil
	}
	msg := fmt.Sprintf("cost %.2f exceeded threshold %.2f", resp.CostUSD, threshold)
	severity := "warning"
	if req.Env[d.opts.StrictEnvKey] == "1" {
		severity = "error"
		resp.Verdict = core.VerdictFAIL
	}
	resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{Severity: severity, Message: msg})
	return resp, nil
}

// parseFloatOrDefault returns d on empty or malformed input — same
// silent-fallback semantics as the build phase's parser so operators
// who set "auto" or similar non-numeric values keep working.
func parseFloatOrDefault(s string, d float64) float64 {
	if s == "" {
		return d
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return d
	}
	return v
}
