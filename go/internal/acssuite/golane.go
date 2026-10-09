package acssuite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type goExecFunc func(ctx context.Context, moduleDir, pkgPattern string, env []string) (string, error)

type goLane struct {
	exec      goExecFunc
	moduleDir string
	patterns  []string
	env       []string
	cycle     int
	budget    laneBudget
}

func runGoTest(opts Options, cfg policy.ACSConfig) []Result {
	lane, noScope := discoverGoLane(opts)
	if noScope != "" {
		return []Result{noPredicatesRed(noScope)}
	}
	lane.env = predicateEnv(predicateExports{
		stateRoot:    opts.stateRoot(),
		sourceRoot:   opts.Root,
		changedPkgs:  changedPackagesForCycle(opts.stateRoot(), opts.Cycle),
		operatorKeys: cfg.PredicateEnv,
	})
	lane.budget = newLaneBudget(goLaneTimeout(opts.GoTimeout, cfg))
	defer lane.budget.cancel()

	var all []Result
	for _, pattern := range lane.patterns {
		all = append(all, lane.runScope(pattern)...)
	}
	if len(all) == 0 {
		return []Result{noPredicatesRed(fmt.Sprintf("no predicate in scopes %v produced a result for cycle %d", lane.patterns, opts.Cycle))}
	}
	return all
}

func loadLaneConfig(stateRoot string) (policy.ACSConfig, []string) {
	pol, err := policy.Load(filepath.Join(stateRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[acs] WARN: %v — the Go lane runs under the default budget and forwards no operator key to predicates\n", err)
	}
	cfg := pol.ACSTimeoutConfig()
	forward, refusals := forwardableKeys(cfg.PredicateEnv)
	for _, refusal := range refusals {
		fmt.Fprintf(os.Stderr, "[acs] WARN: %s\n", refusal)
	}
	cfg.PredicateEnv = forward
	return cfg, refusals
}

func discoverGoLane(opts Options) (goLane, string) {
	lane := goLane{exec: opts.GoExec, moduleDir: moduleRoot(opts), cycle: opts.Cycle}
	if lane.exec != nil {
		lane.patterns = []string{CyclePackage(opts.Cycle), "./acs/regression/...", "./acs/redteam"}
		return lane, ""
	}
	if !hasGoACSTree(lane.moduleDir) {
		return lane, fmt.Sprintf("no Go predicate tree: %s needs a go.mod and an acs/ directory", lane.moduleDir)
	}
	lane.exec = executeCompleteGoScope
	lane.patterns = goLanePatterns(lane.moduleDir, opts.Cycle)
	if len(lane.patterns) == 0 {
		return lane, fmt.Sprintf("no active predicate scope under %s for cycle %d", filepath.Join(lane.moduleDir, "acs"), opts.Cycle)
	}
	return lane, ""
}

func (l goLane) runScope(pattern string) []Result {
	if l.budget.spent() {
		return []Result{l.scopeRed(pattern, l.budget.notStartedReason(), "")}
	}
	raw, execErr := l.exec(l.budget.ctx, l.moduleDir, pattern, l.env)
	results := parseGoTestJSON(strings.NewReader(raw), l.cycle)
	if !l.scopeFailed(results, execErr) {
		return l.retryFlakyReds(pattern, results)
	}
	reason := fmt.Sprintf("predicate scope %s produced no complete evidence: %v%s", pattern, execErr, l.budget.expiryNote())
	return append(results, l.scopeRed(pattern, reason, raw))
}

func (l goLane) scopeFailed(results []Result, execErr error) bool {
	if execErr == nil {
		return false
	}
	return errors.Is(execErr, errIncompleteInventory) || !hasRed(results) || l.budget.spent()
}

func hasRed(results []Result) bool {
	for _, r := range results {
		if r.ResultStr == "red" {
			return true
		}
	}
	return false
}

func (l goLane) scopeRed(pattern, reason, output string) Result {
	r := syntheticRed(acsverdict.SyntheticRedPrefix+"go-lane-scope-failed/"+strings.TrimPrefix(pattern, "./"), reason, output)
	r.IsRegression, r.IsRedTeam = classifyGoPkg(path.Base(pattern), l.cycle)
	return r
}

func syntheticRed(acid, reason, output string) Result {
	full := reason
	if output != "" {
		full += "\noutput:\n" + output
	}
	return Result{ACID: acid, Predicate: acid, ExitCode: 1, ResultStr: "red", EvidenceExcerpt: excerpt(full), fullEvidence: full}
}

func noPredicatesRed(reason string) Result {
	return syntheticRed(acsverdict.NoPredicatesID, reason, "")
}

type laneBudget struct {
	ctx     context.Context
	cancel  context.CancelFunc
	timeout time.Duration
}

func newLaneBudget(timeout time.Duration) laneBudget {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return laneBudget{ctx: ctx, cancel: cancel, timeout: timeout}
}

func (b laneBudget) spent() bool { return b.ctx.Err() != nil }

func (b laneBudget) notStartedReason() string {
	return fmt.Sprintf("not run: the Go lane's shared %s budget (acs.go_timeout_s) was spent before this scope started: %v", b.timeout, b.ctx.Err())
}

func (b laneBudget) expiryNote() string {
	if !b.spent() {
		return ""
	}
	return fmt.Sprintf("; the Go lane's shared %s budget (acs.go_timeout_s) expired while this scope ran: %v", b.timeout, b.ctx.Err())
}

func (l goLane) retryFlakyReds(pattern string, results []Result) []Result {
	if !isRetryable(results) {
		return results
	}
	raw, retryErr := l.exec(l.budget.ctx, l.moduleDir, pattern, l.env)
	retry := parseGoTestJSON(strings.NewReader(raw), l.cycle)
	if hasHarnessResult(retry) {
		retryErr = fmt.Errorf("incomplete retry evidence")
	}
	greenOnRetry := make(map[string]bool, len(retry))
	retryRan := make(map[string]bool, len(retry))
	retryEvidence := make(map[string]string, len(retry))
	for _, r := range retry {
		if retryErr == nil || r.ResultStr == "red" || r.ResultStr == "skip" {
			retryRan[r.ACID] = true
		}
		if r.ResultStr == "green" && retryErr == nil {
			greenOnRetry[r.ACID] = true
		}
		if r.fullEvidence != "" {
			retryEvidence[r.ACID] = r.fullEvidence
		}
	}
	for i := range results {
		if results[i].ResultStr != "red" {
			continue
		}
		switch {
		case greenOnRetry[results[i].ACID]:
			results[i].ResultStr = "green"
			results[i].ExitCode = 0
			results[i].Flaky = "passed-on-retry"
		case retryRan[results[i].ACID]:
			results[i].RetryOutcome = "red-on-retry"
			if re := retryEvidence[results[i].ACID]; re != "" {
				results[i].fullEvidence += "\n--- RETRY RUN (still red) ---\n" + re
			}
		default:
			results[i].RetryOutcome = "retry-inconclusive"
		}
	}
	return results
}

func isRetryable(results []Result) bool {
	hasTestRed := false
	for _, r := range results {
		if r.ResultStr != "red" {
			continue
		}
		if strings.HasPrefix(r.ACID, acsverdict.SyntheticRedPrefix) {
			return false
		}
		hasTestRed = true
	}
	return hasTestRed
}

func hasHarnessResult(results []Result) bool {
	for _, r := range results {
		if strings.HasPrefix(r.ACID, acsverdict.SyntheticRedPrefix) {
			return true
		}
	}
	return false
}
