package phasecontract

import (
	"os"
	"path/filepath"
	"strings"
)

// ChallengeToken reads <workspace>/challenge-token.txt — the per-cycle
// anti-gaming token the orchestrator mints at cycle start (core/cyclerun.go)
// and the bridge reuses-or-mints (bridge/driver_common.go, launch_modes.go) —
// trimmed; only a non-empty token counts. It sits beside
// Contract.RequireChallengeToken, the rule that makes a report echo it. It is
// the reader the phase runner's prompt preparation (the proof-of-read block)
// and the verdict engine's ACS floor (the echo check) share (ADR-0103 unit
// 11). deliverable.Verify's echo violation, bridge/completion.go's git-evidence
// detector, bridge/report.go's ArtifactRef and driver_common.go's
// read-or-mint still spell the read inline — report.go keeps the
// file-present-but-empty distinction this signature folds — so consolidating
// them is a separate edit (unit 11 follow-up F1b), not a claim this comment
// makes.
func ChallengeToken(workspace string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(workspace, "challenge-token.txt"))
	if err != nil {
		return "", false
	}
	tok := strings.TrimSpace(string(raw))
	return tok, tok != ""
}
