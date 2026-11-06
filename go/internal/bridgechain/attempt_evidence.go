package bridgechain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

var evidenceNameRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func AttemptEvidence(e Escalation, pattern string, now time.Time) (string, time.Time) {
	src := filepath.Join(e.Workspace, "escalation-report.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		return "", time.Time{}
	}
	var rep escalationReport
	if json.Unmarshal(raw, &rep) != nil {
		return "", time.Time{}
	}
	if rep.CLI != e.CLI || rep.Pattern != pattern || rep.CapturedAt.Before(e.DispatchStart) {
		return "", time.Time{}
	}
	name := "escalation-" + evidenceNameRE.ReplaceAllString(llmroute.Family(e.CLI)+"-"+e.Tier, "_") + ".json"
	dst := filepath.Join(e.Workspace, name)
	if os.WriteFile(dst, raw, 0o644) != nil {
		return "", time.Time{}
	}
	if !clihealth.QuotaPattern(pattern) {
		return dst, time.Time{}
	}
	reset, ok := quotareset.ParseHint(rep.PaneTail, now)
	if !ok {
		return dst, time.Time{}
	}
	return dst, reset
}
