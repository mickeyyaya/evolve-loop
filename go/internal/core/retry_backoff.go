package core

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// composeCorrection turns a deliverable-reject reason into the correction
// directive injected into the phase re-dispatch (## Correction prompt block).
func composeCorrection(reason, remediation string) string {
	head := "Your previous output for this phase was REJECTED by the deliverable contract check:\n\n" + reason
	if remediation != "" {
		return head + "\n\n" + remediation +
			"\n\nThen finish. Change nothing else beyond what this remedy requires."
	}
	return head +
		"\n\nFix the deliverable so it satisfies the contract — write it at the EXACT contracted path " +
		"with all required sections / valid structure — then finish. Do not change unrelated files."
}

var backoffSleep = time.Sleep

func executeRetryBackoff(attempt, base int) {
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

const maxRecoveryDepth = 2

// phaseTimingEntry records per-phase latency + outcome for phase-timing.json.
