package cycleoutcome

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeStaleDropRepeated signalcenter.Code = "OUTCOME_STALE_DROP_REPEATED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOutcome, CodeStaleDropRepeated, "a lane triage stale-dropped the same inbox item a second time since its premise was last verified; the item stays console-routed and the wave planner holds it off the lanes until `evolve inbox verify` stamps its premise; fields.task_id, stale_drops, evidence_verified")
}

const maxCitedChecks = 8

var (
	updateItemJSON       = inboxmover.UpdateItemJSON
	evidenceCheckTimeout = 10 * time.Second
)

var (
	citedSHA   = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	citedPath  = regexp.MustCompile(`\b(?:[\w.-]+/)+[\w.-]+\.(?:go|md|json|jsonl|yaml|yml|sh|toml|txt|mod|sum)\b`)
	absentWord = regexp.MustCompile(`(?i)\b(absent|not found|not present|does not exist|invalid)\b`)
)

type dropEvidence struct {
	verified bool
	note     string
	stale    bool
}

func judgeDrop(root, workspace string, d committedset.Disposition) (dropEvidence, bool) {
	if d.Bucket != "dropped" {
		return dropEvidence{}, false
	}
	if strings.HasPrefix(strings.ToLower(d.Reason), "stale:") {
		ev := verifyStaleEvidence(root, d.Reason)
		ev.stale = true
		return ev, true
	}
	if absentWord.MatchString(d.Reason) && (strings.Contains(strings.ToLower(d.Reason), "assigned") || strings.Contains(d.Reason, d.ID)) {
		return verifyAbsentClaim(workspace, d.ID), true
	}
	return dropEvidence{}, false
}

func verifyStaleEvidence(root, reason string) dropEvidence {
	ctx, cancel := context.WithTimeout(context.Background(), evidenceCheckTimeout)
	defer cancel()
	git := gitexec.Default(root)
	var shas []string
	for _, s := range citedSHA.FindAllString(reason, -1) {
		if strings.ContainsAny(s, "0123456789") && len(shas) < maxCitedChecks {
			shas = append(shas, s)
		}
	}
	paths := citedPath.FindAllString(reason, maxCitedChecks)
	if len(shas) == 0 && len(paths) == 0 {
		return dropEvidence{note: "the stale drop cites no commit and no file"}
	}
	var failed []string
	for _, sha := range shas {
		if _, _, code, err := git.Capture(ctx, "merge-base", "--is-ancestor", sha, "HEAD"); err != nil || code != 0 {
			failed = append(failed, sha+" is not an ancestor of HEAD")
		}
	}
	for _, p := range paths {
		if _, _, code, err := git.Capture(ctx, "cat-file", "-e", "HEAD:"+p); err != nil || code != 0 {
			failed = append(failed, p+" is not at HEAD")
		}
	}
	if ctx.Err() != nil {
		return dropEvidence{note: "the evidence check ran out of time, so the evidence is unverified"}
	}
	if len(failed) > 0 {
		return dropEvidence{note: strings.Join(failed, "; ")}
	}
	return dropEvidence{verified: true, note: fmt.Sprintf("checked %d commit(s) and %d file(s) at HEAD", len(shas), len(paths))}
}

func verifyAbsentClaim(workspace, id string) dropEvidence {
	body, err := os.ReadFile(filepath.Join(workspace, "scout-report.md"))
	if err != nil {
		return dropEvidence{note: "the drop claims the id is absent, and there is no scout-report.md to check it against"}
	}
	n := len(regexp.MustCompile(`(?i)(^|[^a-z0-9])`+regexp.QuoteMeta(id)+`([^a-z0-9-]|$)`).FindAllString(string(body), -1))
	if n > 0 {
		return dropEvidence{note: fmt.Sprintf("the drop claims the id is absent, but scout-report.md names %s %d time(s)", id, n)}
	}
	return dropEvidence{verified: true, note: "scout-report.md does not name the id"}
}

func stampDropEvidence(in NoWorkInputs, path string, d committedset.Disposition) error {
	ev, judged := judgeDrop(in.ProjectRoot, in.Workspace, d)
	if !judged {
		return nil
	}
	now := time.Now().UTC()
	drops := 0
	err := updateItemJSON(path, func(item map[string]json.RawMessage) {
		item[inboxbatch.EvidenceVerifiedField] = json.RawMessage(fmt.Sprint(ev.verified))
		item[inboxbatch.EvidenceNoteField], _ = json.Marshal(ev.note)
		if !ev.stale {
			return
		}
		drops = staleDropsBefore(item) + 1
		item[inboxbatch.StaleDropsField] = json.RawMessage(fmt.Sprint(drops))
		item[inboxbatch.LastStaleDropAtField], _ = json.Marshal(now.Format(time.RFC3339))
	})
	if err != nil {
		return fmt.Errorf("record the drop evidence of %s: %w", d.ID, err)
	}
	if drops >= inboxbatch.StaleDropHold {
		in.Signals.Emit(signalcenter.Event{
			Cycle: in.Cycle, Module: signalcenter.ModuleOutcome, Origin: "ApplyNoWork", Kind: signalcenter.KindOutcomeWarning,
			Severity: signalcenter.SeverityWarn, Code: CodeStaleDropRepeated,
			Reason: fmt.Sprintf("%s was stale-dropped %d times since its premise was last verified; it stays off the lanes until `evolve inbox verify %s`", d.ID, drops, d.ID),
			Fields: map[string]string{"task_id": d.ID, "stale_drops": fmt.Sprint(drops), "evidence_verified": fmt.Sprint(ev.verified)},
		})
	}
	return nil
}

func staleDropsBefore(item map[string]json.RawMessage) int {
	var drops int
	var last, verified string
	_ = json.Unmarshal(item[inboxbatch.StaleDropsField], &drops)
	_ = json.Unmarshal(item[inboxbatch.LastStaleDropAtField], &last)
	_ = json.Unmarshal(item[inboxbatch.PremiseVerifiedAtField], &verified)
	if inboxbatch.VerifiedAfter(verified, last) {
		return 0
	}
	return drops
}
