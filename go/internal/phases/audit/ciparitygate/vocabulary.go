package ciparitygate

import "strings"

// The three field vocabularies the codes' docs enumerate — TYPED, so a call
// site spells a declared value (the typed producers stepFailed / failed /
// lockDegraded accept nothing else) and the registered doc strings render
// from the SAME closed sets: one home, two projections (the event field and
// docs/architecture/signal-codes.md, the operator's triage vocabulary).
// TestVocabulary_TypedConstsAreTheRenderedClosedSets holds every declared
// const to its set, so a value added beside a call site cannot leave the
// rendered doc stale.

// gateStep is GATE_STEP_FAILED's fields.step — the pre-verdict step that
// could not run (the gate then fails OPEN).
type gateStep string

const (
	stepExec           gateStep = "exec"             // the gate's one command could not start (runGate)
	stepTierAttempt    gateStep = "tier_attempt"     // the integration tier's first attempt could not start
	stepTierList       gateStep = "tier_list"        // `go list ./...` for the module-root fallback
	stepBinDir         gateStep = "bin_dir"          // ensuring <module>/bin for the scratch profile
	stepCoverRun       gateStep = "cover_run"        // the scoped coverage run
	stepCoverFunc      gateStep = "cover_func"       // `go tool cover -func`
	stepWriteFuncCover gateStep = "write_func_cover" // writing the -func profile
	stepPkgDirs        gateStep = "pkg_dirs"         // `go list -e -f {{.Dir}}`
	stepMeasure        gateStep = "measure"          // the in-process apicover measurement was interrupted
)

// gateCause is GATE_FAILED's fields.cause — why the gate returned offenders.
type gateCause string

const (
	causeExit                gateCause = "exit"                  // the command ran and exited non-zero
	causeRetakeExecFailed    gateCause = "retake_exec_failed"    // attempt-1 offenders stand: the retake could not start
	causeDeadlineWithMarkers gateCause = "deadline_with_markers" // the retake was killed at its budget but had flushed a verdict
	causeRetakeRed           gateCause = "retake_red"            // red twice — the serialized retake is the truthful attempt
	causeUnderivable         gateCause = "underivable"           // an apicover gate could not prove its set empty (hard FAIL)
	causeApicover            gateCause = "apicover"              // apicover -enforce offenders or a measurement error
	causeUngraduated         gateCause = "ungraduated"           // a new package absent from .apicover-enforce
)

// lockReason is TIER_LOCK_UNAVAILABLE's fields.reason — how the cross-lane
// retake lock degraded (best-effort by design: the retake ran unserialized).
type lockReason string

const (
	lockNoRoot  lockReason = "no_root" // no ProjectRoot and no Worktree to host the lock file
	lockError   lockReason = "error"   // flock.TryLock failed
	lockTimeout lockReason = "timeout" // held past TierLockWait
)

// The closed sets the docs render (§5).
var (
	gateSteps   = []gateStep{stepExec, stepTierAttempt, stepTierList, stepBinDir, stepCoverRun, stepCoverFunc, stepWriteFuncCover, stepPkgDirs, stepMeasure}
	gateCauses  = []gateCause{causeExit, causeRetakeExecFailed, causeDeadlineWithMarkers, causeRetakeRed, causeUnderivable, causeApicover, causeUngraduated}
	lockReasons = []lockReason{lockNoRoot, lockError, lockTimeout}
)

// vocabulary renders a closed set the way the docs enumerate it — `{a, b, c}`.
// Commas, never pipes: the registry renders each doc into a markdown table
// CELL unescaped (signalcenter/render.go:29), so a `|` splits the row.
func vocabulary[T ~string](values []T) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = string(v)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
