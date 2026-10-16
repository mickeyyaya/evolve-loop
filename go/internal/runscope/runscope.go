// Package runscope is the single source of every run-, cycle- and lane-scoped name the loop emits.
// See docs/architecture/packages/internal-runscope.md.
package runscope

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/projecthash"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

const EnvLane = "EVOLVE_" + "LANE"

type Lane string

type RunScope struct {
	lane  Lane
	runID string
	cycle int
}

func New(lane Lane, runID string, cycle int) RunScope {
	return RunScope{lane: lane, runID: runID, cycle: cycle}
}

func ResolveLane(explicit, root string, getenv func(string) string) Lane {
	if s := sanitizeLane(explicit); s != "" {
		return Lane(s)
	}
	if getenv != nil {
		if s := sanitizeLane(getenv(EnvLane)); s != "" {
			return Lane(s)
		}
	}
	return LaneFromRoot(root)
}

func LaneFromRoot(root string) Lane {
	return Lane(projecthash.ForProjectRoot(absClean(root)))
}

func (s RunScope) LaneToken() string { return string(s.lane) }

func (s RunScope) Cycle() int { return s.cycle }

func (s RunScope) RunID() string { return s.runID }

func (s RunScope) CycleBranch() string {
	return "cycle-" + string(s.lane) + "-" + strconv.Itoa(s.cycle)
}

func (s RunScope) WorktreeDir(base string) string {
	return filepath.Join(base, s.CycleBranch())
}

func (s RunScope) IntegrationBranch() string {
	return s.CycleBranch() + "-integration"
}

func (s RunScope) WorkerBranch(workerID string) string {
	return s.CycleBranch() + "-" + workerID
}

func (s RunScope) WorkspacePath(root string) string {
	return filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(s.cycle))
}

func (s RunScope) RunToken() string {
	return sessionrecord.RunScopeToken(s.runID)
}

func (s RunScope) SessionPrefix() string {
	if s.runID == "" {
		return ""
	}
	return s.RunToken() + "-"
}

func absClean(root string) string {
	c := filepath.Clean(root)
	if abs, err := filepath.Abs(c); err == nil {
		return abs
	}
	return c
}

func sanitizeLane(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}
