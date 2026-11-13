// Package gateregistry lists each gate that has a typed entry and each pack pattern that has none; see docs/architecture/gate-contract.md.
package gateregistry

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

func Entries() []gatecontract.Entry {
	return append([]gatecontract.Entry{testdupEntry(), sizeratchetEntry(), rawgitratchetEntry()}, floorEntries()...)
}

func Unregistered() []string {
	return []string{
		"./internal/phasespec/...",
		"./internal/profiles/...",
		"./internal/phasecoherence/...",
		"./internal/routingtest/...",
		"./internal/rawgitratchet/...",
		"./internal/sizeratchet/...",
		"./internal/testmainexit/...",
		"./internal/repocontract/...",
		"./internal/policy/...",
		"./internal/guards/...",
		"./internal/acssuite/...",
		"./internal/fleet/...",
		"./internal/evalqualitycheck/...",
		"./internal/inboxrank/...",
		"./internal/ipcenv/...",
	}
}

func verdictOf(judged error, notes ...string) gatecontract.Verdict {
	verdict := gatecontract.Verdict{Notes: notes}
	if judged == nil {
		return verdict
	}
	for _, line := range strings.Split(judged.Error(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			verdict.Reds = append(verdict.Reds, line)
		}
	}
	return verdict
}
