package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var scoutArtifactName = func() string {
	c, _ := phasecontract.For("scout")
	return c.ArtifactName
}()

const LaneScopeFile = "lane-scope.json"

type LaneScope = committedset.LanePinDoc

func loadLaneScope(workspace string) *LaneScope {
	if workspace == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(workspace, LaneScopeFile))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s unreadable: %v (falling back to env fleet scope)\n", LaneScopeFile, err)
		}
		return nil
	}
	var ls LaneScope
	if err := json.Unmarshal(b, &ls); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s malformed: %v (falling back to env fleet scope)\n", LaneScopeFile, err)
		return nil
	}
	return &ls
}

func LaneScopeIDs(workspace string) []string {
	if scope := loadLaneScope(workspace); scope != nil && len(scope.TodoIDs) > 0 {
		return scope.TodoIDs
	}
	return nil
}

func materializeLaneScope(workspace, scope, goalHash string) {
	if workspace == "" {
		return
	}
	ids := strings.Split(scope, ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}
	b, err := json.Marshal(LaneScope{TodoIDs: ids, GoalHash: goalHash})
	if err == nil {
		if err = os.MkdirAll(workspace, 0o755); err == nil {
			err = os.WriteFile(filepath.Join(workspace, LaneScopeFile), b, 0o644)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s materialize failed: %v (Context injection still carries the scope)\n", LaneScopeFile, err)
	}
}

var fencedJSONRe = regexp.MustCompile("(?s)```json\\s*(.*?)```")

func scoutReportGoalHash(workspace string) string {
	b, err := os.ReadFile(filepath.Join(workspace, scoutArtifactName))
	if err != nil {
		return ""
	}
	return scoutReportGoalHashFromBytes(b)
}

func scoutReportGoalHashFromBytes(b []byte) string {
	hash := ""
	for _, m := range fencedJSONRe.FindAllSubmatch(b, -1) {
		var trace struct {
			GoalHash string `json:"goal_hash"`
		}
		if json.Unmarshal(m[1], &trace) == nil && trace.GoalHash != "" {
			hash = trace.GoalHash
		}
	}
	return hash
}

var canonicalGoalHashRe = regexp.MustCompile("^[0-9a-f]{64}$")

func normalizeScoutGoalHash(workspace string, warn io.Writer) {
	ls := loadLaneScope(workspace)
	if ls == nil || ls.GoalHash == "" {
		return
	}
	reportPath := filepath.Join(workspace, scoutArtifactName)
	b, err := os.ReadFile(reportPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(warn, "[orchestrator] WARN %s unreadable during goal_hash normalize: %v (lane identity still pinned in %s)\n", scoutArtifactName, err, LaneScopeFile)
		}
		return
	}
	got := scoutReportGoalHashFromBytes(b)
	if got == "" || got == ls.GoalHash {
		return
	}
	if !canonicalGoalHashRe.MatchString(got) {
		fmt.Fprintf(warn, "[orchestrator] WARN scout-report goal_hash %q is not a canonical 64-hex hash — NOT machine-stamping (blast-radius guard); lane identity is the pin %q in %s.\n", got, ls.GoalHash, LaneScopeFile)
		return
	}
	fixed := bytes.ReplaceAll(b, []byte(got), []byte(ls.GoalHash))
	if err := os.WriteFile(reportPath, fixed, 0o644); err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN scout goal_hash normalize write failed: %v (lane identity still pinned in %s)\n", err, LaneScopeFile)
		return
	}
	fmt.Fprintf(warn, "[orchestrator] WARN scout-report Decision Trace goal_hash %q != pinned %q — machine-stamped the authoritative pin (scout mis-echoed the hash; lane identity is the pin, not the LLM transcription).\n", got, ls.GoalHash)
}
