package core

// crossartifact_invariants.go — the ADVISORY half of the cycle-1676
// cross-artifact invariant stack. internal/coherence computes the four weak
// verifiers; this is the one place the real cycle-close path records them.
//
// Two properties are load-bearing and are pinned at this seam
// (crossartifact_invariants_wiring_test.go):
//
//  1. The stack is bound to the LANE worktree (cs.ActiveWorktree), never the
//     projectRoot argument — in fleet mode a project-root snapshot names a tree
//     this lane did not write, which is the #612 lesson.
//  2. A finding NEVER blocks. It changes no verdict, raises no system failure,
//     and is recorded on EVERY cycle, all-ok included: a false-positive rate
//     that is never recorded can never be evidenced, and that evidence is the
//     only door to graduating any of these invariants to blocking (the
//     1054/1060 breaker lesson, and the inbox record's own rule).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
)

// crossArtifactInvariantsFile is the advisory record's workspace filename.
const crossArtifactInvariantsFile = "crossartifact-invariants.json"

// recordCrossArtifactInvariants evaluates the advisory stack over the cycle's
// workspace + lane worktree and writes the result to the workspace. It is
// best-effort and LOUD: a write failure warns on stderr and is otherwise
// ignored, because an advisory observer must never be able to fail a cycle.
func recordCrossArtifactInvariants(cycle int, workspace, worktree string) {
	if strings.TrimSpace(workspace) == "" {
		return
	}
	report := coherence.CheckCrossArtifactInvariants(workspace, worktree)
	path := filepath.Join(workspace, crossArtifactInvariantsFile)
	buf, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		buf = append(buf, '\n')
		tmp := path + ".tmp"
		if err = os.WriteFile(tmp, buf, 0o644); err == nil {
			err = os.Rename(tmp, path)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d could not record %s: %v (advisory only; the cycle is unaffected)\n", cycle, crossArtifactInvariantsFile, err)
		return
	}
	for _, v := range report.Violations() {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d cross-artifact ADVISORY %s: %s (recorded in %s; advisory — this does not block the cycle)\n", cycle, v.Name, v.Evidence, path)
	}
}
