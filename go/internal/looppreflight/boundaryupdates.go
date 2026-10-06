package looppreflight

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

const unrecordedDriftNote = "not recorded by the boundary updater (evolve cli update): a self-update or a manual install changed these"

type versionChanges struct {
	drift       []string
	selfUpdated []string
	expected    []string
	recordErr   error
}

func classifyVersionChanges(evolveDir string, prev, current map[string]string) versionChanges {
	records, err := cliupdate.LoadRecords(evolveDir)
	records = recordedSinceBaseline(records, filepath.Join(evolveDir, "cli-versions.json"))
	out := versionChanges{recordErr: err}
	for bin, cur := range current {
		was, had := prev[bin]
		if !had || was == cur {
			continue
		}
		switch cliupdate.CauseOf(records, bin, was, cur) {
		case cliupdate.CauseBoundaryUpdate:
			out.expected = append(out.expected, fmt.Sprintf("%s %s → %s", bin, was, cur))
		case cliupdate.CauseSelfUpdate:
			out.selfUpdated = append(out.selfUpdated, fmt.Sprintf("%s self-updated, smoke OK: %s → %s (outside a boundary; the boundary update step smoke-booted it before this check)", bin, was, cur))
		default:
			out.drift = append(out.drift, fmt.Sprintf("%s changed: %s → %s", bin, was, cur))
		}
	}
	sort.Strings(out.drift)
	sort.Strings(out.selfUpdated)
	sort.Strings(out.expected)
	return out
}

func recordedSinceBaseline(records []cliupdate.Record, baselinePath string) []cliupdate.Record {
	info, err := os.Stat(baselinePath)
	if err != nil {
		return records
	}
	return slices.DeleteFunc(records, func(r cliupdate.Record) bool { return !r.At.After(info.ModTime()) })
}

func (c versionChanges) result(name string) (CheckResult, bool) {
	switch {
	case len(c.drift) > 0:
		return c.warn(name, fmt.Sprintf("%d CLI(s) changed version since last batch", len(c.drift))), true
	case len(c.selfUpdated) > 0:
		return c.warn(name, fmt.Sprintf("%d CLI(s) self-updated outside a boundary since last batch (smoke OK at the boundary)", len(c.selfUpdated))), true
	case len(c.expected) > 0:
		return CheckResult{
			Name:    name,
			Level:   LevelPass,
			Message: fmt.Sprintf("%d CLI(s) updated by the boundary updater since last batch (expected)", len(c.expected)),
			Detail:  strings.Join(c.expectedLines(), "\n"),
		}, true
	}
	return CheckResult{}, false
}

func (c versionChanges) warn(name, message string) CheckResult {
	lines := append([]string{}, c.drift...)
	if len(c.drift) > 0 {
		lines = append(lines, unrecordedDriftNote)
	}
	if c.recordErr != nil {
		lines = append(lines, "boundary update record unreadable: "+c.recordErr.Error())
	}
	lines = append(lines, c.selfUpdated...)
	lines = append(lines, c.expectedLines()...)
	return CheckResult{Name: name, Level: LevelWarn, Message: message, Detail: strings.Join(lines, "\n")}
}

func (c versionChanges) expectedLines() []string {
	lines := make([]string, len(c.expected))
	for i, e := range c.expected {
		lines[i] = "expected (boundary update): " + e
	}
	return lines
}
