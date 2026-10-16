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

type Snapshot struct {
	size    int64
	modTime time.Time
}

func StatSnapshot(path string) (Snapshot, bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		return Snapshot{}, false
	}
	return Snapshot{size: fi.Size(), modTime: fi.ModTime()}, true
}

func unchangedSince(path string, snap Snapshot) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	return fi.Size() == snap.size && fi.ModTime().Equal(snap.modTime)
}

func classifiedArtifact(res deliverable.Result, artifactPath, pane string) string {
	if res.ArtifactPath == artifactPath && res.Content != "" {
		return res.Content
	}
	if data, err := os.ReadFile(artifactPath); err == nil {
		return string(data)
	}
	if !res.OK {
		return ""
	}
	return pane
}

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

func forensicCodes(vs []deliverable.Violation) string {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		parts = append(parts, string(v.Code))
	}
	return strings.Join(parts, ",")
}

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
