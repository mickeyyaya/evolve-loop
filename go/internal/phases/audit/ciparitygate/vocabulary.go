package ciparitygate

import "strings"

type gateStep string

const (
	stepExec           gateStep = "exec"
	stepTierAttempt    gateStep = "tier_attempt"
	stepTierList       gateStep = "tier_list"
	stepBinDir         gateStep = "bin_dir"
	stepCoverRun       gateStep = "cover_run"
	stepCoverFunc      gateStep = "cover_func"
	stepWriteFuncCover gateStep = "write_func_cover"
	stepPkgDirs        gateStep = "pkg_dirs"
	stepMeasure        gateStep = "measure"
)

type gateCause string

const (
	causeExit                gateCause = "exit"
	causeRetakeExecFailed    gateCause = "retake_exec_failed"
	causeDeadlineWithMarkers gateCause = "deadline_with_markers"
	causeRetakeRed           gateCause = "retake_red"
	causeUnderivable         gateCause = "underivable"
	causeApicover            gateCause = "apicover"
	causeUngraduated         gateCause = "ungraduated"
)

type lockReason string

const (
	lockNoRoot  lockReason = "no_root"
	lockError   lockReason = "error"
	lockTimeout lockReason = "timeout"
)

var (
	gateSteps   = []gateStep{stepExec, stepTierAttempt, stepTierList, stepBinDir, stepCoverRun, stepCoverFunc, stepWriteFuncCover, stepPkgDirs, stepMeasure}
	gateCauses  = []gateCause{causeExit, causeRetakeExecFailed, causeDeadlineWithMarkers, causeRetakeRed, causeUnderivable, causeApicover, causeUngraduated}
	lockReasons = []lockReason{lockNoRoot, lockError, lockTimeout}
)

func vocabulary[T ~string](values []T) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = string(v)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
