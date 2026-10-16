// Package backfill reconstructs missing phase artifacts from stdout.clean.txt
// when a phase's Write tool call timed out but the content was emitted to stdout.
package backfill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

var phaseHeaders = map[string]string{
	"scout":         "# Scout Report",
	"build":         "# Build Report",
	"audit":         "# Audit Report",
	"tdd":           "# TDD",
	"intent":        "# Intent",
	"triage":        "# Triage",
	"retro":         "# Retrospective Report",
	"build-planner": "# Build Plan",
}

func TryExtract(workspace, phase, artifactPath string, minLen int) (bool, error) {
	header, ok := phaseHeaders[phase]
	if !ok {
		return false, nil
	}

	cleanPath := filepath.Join(workspace, phase+"-stdout.clean.txt")
	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return false, nil
	}

	text := string(raw)
	idx := strings.LastIndex(text, header)
	if idx < 0 {
		return false, nil
	}

	content := strings.TrimSpace(text[idx:])
	if len(content) < minLen {
		return false, nil
	}

	if err := atomicwrite.Bytes(artifactPath, []byte(content)); err != nil {
		return false, fmt.Errorf("backfill write %s: %w", artifactPath, err)
	}
	return true, nil
}
