package verdict

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Snapshot is the (size, mtime) identity of the artifact at one instant; only StatSnapshot mints one.
// It is the bridge stability window's key, so the two notions of "unchanged" cannot drift.
type Snapshot struct {
	size    int64
	modTime time.Time
}

// StatSnapshot snapshots path if it is a non-empty regular file.
func StatSnapshot(path string) (Snapshot, bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		return Snapshot{}, false
	}
	return Snapshot{size: fi.Size(), modTime: fi.ModTime()}, true
}

// unchangedSince reports whether path still matches snap by size and mtime.
// Any error reads as changed, so a doubt never refuses a reconcile.
func unchangedSince(path string, snap Snapshot) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	return fi.Size() == snap.size && fi.ModTime().Equal(snap.modTime)
}

// classifiedArtifact returns the bytes Classify judges for a contracted phase, reading the disk at
// most once. The verified bytes win only when they are this artifact's and non-empty, so a racing
// writer cannot slip past the gate.
func classifiedArtifact(res deliverable.Result, artifactPath, pane string) string {
	if res.ArtifactPath == artifactPath && res.Content != "" {
		return res.Content
	}
	if data, err := os.ReadFile(artifactPath); err == nil {
		return string(data)
	}
	if !res.OK {
		return "" // contracted file genuinely absent → Classify sees no sentinel → FAIL
	}
	return pane
}

// forensicSnapshot renders a file as "absent" or its size and last tailN bytes, where the verdict
// sentinel lives. The read error is discarded on purpose: a directory renders tail="", and the
// bytes are pinned by a golden.
func forensicSnapshot(path string, tailN int) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "absent"
	}
	data, _ := os.ReadFile(path)
	tail := string(data)
	if len(tail) > tailN {
		tail = tail[len(tail)-tailN:]
	}
	return fmt.Sprintf("size=%d tail=%q", fi.Size(), tail)
}

// forensicCodes renders a violation set as its comma-joined codes.
func forensicCodes(vs []deliverable.Violation) string {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		parts = append(parts, string(v.Code))
	}
	return strings.Join(parts, ",")
}

// acsFloorRescues reports whether the deterministic ACS verdict overrides a teardown-time not-OK Verify:
// only for audit, only when audit and acssuite both read PASS, and only when report (the verified bytes)
// echoes this cycle's challenge token, so a stale or forged report cannot be laundered to PASS.
func acsFloorRescues(phase, workspace, report string) bool {
	if phase != string(core.PhaseAudit) {
		return false
	}
	audit, acs, auditRan := coherence.ReadCycleVerdicts(workspace)
	if !auditRan || audit != "PASS" || acs != "PASS" {
		return false
	}
	tok, ok := phasecontract.ChallengeToken(workspace)
	if !ok {
		return false
	}
	return strings.Contains(report, tok)
}
