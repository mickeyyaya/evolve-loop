package core

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func isContentionShipCode(code ShipErrorCode) bool {
	return strings.HasPrefix(string(code), "AUDIT_BINDING_") || code == CodeGitFleetRebaseNeeded
}

func shipRecoveryBudget(code ShipErrorCode, fleetWidth int) int {
	if !isContentionShipCode(code) {
		return maxRecoveryDepth
	}
	if b := fleetWidth + 1; b > maxRecoveryDepth {
		return b
	}
	return maxRecoveryDepth
}

func fleetWidthFromEnv(env map[string]string) int {
	w, err := strconv.Atoi(strings.TrimSpace(env[ipcenv.FleetWidthKey]))
	if err != nil || w < 1 {
		return 1
	}
	return w
}

const (
	contentionBackoffBase = 2 * time.Second
	contentionBackoffCap  = 30 * time.Second
)

func contentionBackoff(depth int) time.Duration {
	window := time.Duration(depth+1) * contentionBackoffBase
	if window > contentionBackoffCap {
		window = contentionBackoffCap
	}
	return time.Millisecond + time.Duration(rand.Int64N(int64(window)))
}
