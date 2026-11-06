package commitgate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

const (
	againStep      = "then run record-pr again."
	reseatStep     = "run the review seats and evolve commit-gate run on this change, " + againStep
	fixStep        = "fix the cause, run the review seats and evolve commit-gate run again, " + againStep
	capabilityStep = "run a review seat with that capability and evolve commit-gate run with --reviewers run:<label>, " + againStep
)

func CheckReviewedChange(rec ReviewRecord, label string, paths []string) string {
	att := rec.Attestation
	if att.ReviewWaiver != "" {
		return fmt.Sprintf("the record of change %s holds the waiver %q, and a waiver is not a review. Next: %s", label, att.ReviewWaiver, reseatStep)
	}
	ids := namedRunIDs(att)
	if len(ids) == 0 {
		return fmt.Sprintf("the record of change %s names no review run. Next: run evolve commit-gate run with --reviewers run:<label>, %s", label, againStep)
	}
	profiles := make([]string, 0, len(ids))
	for _, id := range ids {
		run, refused := checkCopiedRun(rec, id)
		if refused != "" {
			return fmt.Sprintf("the record of change %s names review run %s: %s. Next: %s", label, id, refused, fixStep)
		}
		profiles = append(profiles, run.Profile)
	}
	missing := MissingReviewCapabilities(paths, detectLangs(paths), profiles)
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("the review runs of change %s prove no %s (profiles: %s). Next: %s", label, strings.Join(missing, " and "), strings.Join(profiles, ", "), capabilityStep)
}

func namedRunIDs(att Attestation) []string {
	var ids []string
	for _, run := range att.ReviewRuns {
		ids = append(ids, run.RunID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func checkCopiedRun(rec ReviewRecord, id string) (ReviewRun, string) {
	i := slices.IndexFunc(rec.LaunchRecords, func(l launchrecord.Record) bool { return l.RunID == id })
	if i < 0 {
		return ReviewRun{}, "no launch record of the run is copied in the record"
	}
	return CheckReviewRun(rec.LaunchRecords[i], rec.Attestation.TreeStateSHA)
}
