package core

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
)

const phaseMaxAttempts = envchain.DefPhaseMaxAttempts

// resolvePhaseMaxAttempts reads EVOLVE_PHASE_MAX_ATTEMPTS and clamps it to
// [1, MaxPhaseMaxAttempts]. A sub-1 or unparseable value is treated as
// invalid → default; an over-max value is capped. (IntMin handles the lower
// bound + invalid-input → default; the explicit cap handles the upper bound.)
func resolvePhaseMaxAttempts(env map[string]string) int {
	n := envchain.IntMin(envchain.KeyPhaseMaxAttempts, env, phaseMaxAttempts, 1)
	if n > envchain.MaxPhaseMaxAttempts {
		return envchain.MaxPhaseMaxAttempts
	}
	return n
}

// composeCorrection turns a deliverable-reject reason into the correction
// directive injected into the phase re-dispatch (## Correction prompt block).
func composeCorrection(reason string) string {
	return "Your previous output for this phase was REJECTED by the deliverable contract check:\n\n" +
		reason +
		"\n\nFix the deliverable so it satisfies the contract — write it at the EXACT contracted path " +
		"with all required sections / valid structure — then finish. Do not change unrelated files."
}

// resolveContractCorrectionRetries reads EVOLVE_CONTRACT_CORRECTION_RETRIES.
// 0 is valid (disable → immediate abort). Negative/unparseable → default 2;
// above the ceiling → clamped to 5. (envchain.Int returns the default on an
// empty/unparseable value; the explicit guards handle the negative-but-parseable
// and over-max cases.)
func resolveContractCorrectionRetries(env map[string]string) int {
	n := envchain.Int(envchain.KeyContractCorrectionRetries, env, envchain.DefContractCorrectionRetries)
	if n < 0 {
		return envchain.DefContractCorrectionRetries
	}
	if n > envchain.MaxContractCorrectionRetries {
		return envchain.MaxContractCorrectionRetries
	}
	return n
}

// resolveRetryBackoffBase reads EVOLVE_RETRY_BACKOFF_BASE_S, flooring a
// negative value at 0 (disabled). Unset / empty / unparseable → default.
func resolveRetryBackoffBase(env map[string]string) int {
	n := envchain.Int(envchain.KeyRetryBackoffBaseS, env, envchain.DefRetryBackoffBaseS)
	if n < 0 {
		return 0
	}
	return n
}

// backoffSleep is the sleep seam for executeRetryBackoff. Production uses the
// real time.Sleep; the core test suite swaps in a no-op (see TestMain) so the
// ~13 retry/transient/backfill/timeout tests don't each sleep the multi-second
// backoff for real — the single highest-leverage knob for core-suite latency
// (~254s → ~8s). Set once before any test runs, so concurrent reads by parallel
// tests are safe.
//
// Why a package var and not an Orchestrator field (the `now` convention)?
// executeRetryBackoff is a free function, and — decisively — only TestMain can
// zero a package var for the WHOLE suite. A per-instance field would force every
// retry test (and every future one) to inject a no-op at construction, which is
// the exact per-test churn this seam removes.
var backoffSleep = time.Sleep

func executeRetryBackoff(attempt int, env map[string]string) {
	base := resolveRetryBackoffBase(env)
	if base <= 0 {
		return
	}
	nextAttempt := attempt + 1
	if nextAttempt < 2 {
		return
	}
	sleepSecs := base * (1 << (nextAttempt - 2))
	limitSecs := base
	if limitSecs < 30 {
		limitSecs = 30
	}
	if sleepSecs > limitSecs {
		sleepSecs = limitSecs
	}
	if sleepSecs > 0 {
		backoffSleep(time.Duration(sleepSecs) * time.Second)
	}
}

func isTransientBridgeError(err error) bool {
	return errors.Is(err, ErrTransientBridgeFailure)
}

func bridgeExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrArtifactTimeout) {
		return 81
	}
	errStr := err.Error()
	const target = "bridge: launch exit="
	idx := strings.Index(errStr, target)
	if idx != -1 {
		start := idx + len(target)
		end := start
		for end < len(errStr) && errStr[end] >= '0' && errStr[end] <= '9' {
			end++
		}
		if end > start {
			code, _ := strconv.Atoi(errStr[start:end])
			return code
		}
	}
	return 0
}

// maxRecoveryDepth bounds advisor-driven ship-error recovery per cycle
// (Component #5/#7). Ship is a pure executor: a structured ShipError is
// resolved by routing to a recovery phase (re-audit / retry-ship / debugger),
// not by aborting. This caps ship→recover→ship so a persistent blocker cannot
// loop forever; on exhaustion the orchestrator aborts loud with the accumulated
// ShipError. A safety invariant, not a flag (the outer safety<32 loop backstops).
const maxRecoveryDepth = 2

// phaseTimingEntry records per-phase latency + outcome for phase-timing.json.
