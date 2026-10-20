package ciwatch

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	BaseRedItemPrefix = "ci-base-red-"
	BaseRedSource     = "ciclassify"
	baseRedSlugMax    = 48
)

var nonSlugRun = regexp.MustCompile(`[^a-z0-9]+`)

var pendingFolders = []string{"retry", "quarantine"}

func FileBaseRedItems(inboxDir string, rep ClassifyReport, now time.Time) ([]string, error) {
	var reds []ClassifiedFailure
	for _, f := range rep.Failures {
		if f.Evidence.BaseRed == FactYes && f.Evidence.RerunGreen == FactNo {
			reds = append(reds, f)
		}
	}
	if len(reds) == 0 {
		return nil, nil
	}
	known, err := knownItemIDs(inboxDir)
	if err != nil {
		return nil, err
	}
	now = now.UTC()
	var filed []string
	for _, f := range reds {
		item := baseRedItem(rep, f.FailingTest, now)
		if known[item.ID] {
			continue
		}
		if err := writeInboxItem(inboxDir, fmt.Sprintf("%s-%s.json", now.Format(inboxbatch.FilenameStampLayout), item.ID), item); err != nil {
			return filed, err
		}
		known[item.ID] = true
		filed = append(filed, item.ID)
	}
	return filed, nil
}

func knownItemIDs(inboxDir string) (map[string]bool, error) {
	known := map[string]bool{}
	dirs := []string{inboxDir}
	for _, folder := range pendingFolders {
		dirs = append(dirs, filepath.Join(inboxDir, folder))
	}
	dirs = append(dirs, inboxbatch.ProcessingCycleDirs(inboxDir)...)
	for _, dir := range dirs {
		scan, err := inboxbatch.ScanDir(dir)
		if err != nil {
			return nil, fmt.Errorf("ciwatch: base-red dedup: %w", err)
		}
		for _, it := range scan.Items {
			known[it.ID] = true
		}
	}
	return known, nil
}

func baseRedItemID(f FailingTest) string {
	sum := sha256.Sum256([]byte(failureKey(f)))
	slug := strings.Trim(nonSlugRun.ReplaceAllString(strings.ToLower(failureName(f)), "-"), "-")
	if len(slug) > baseRedSlugMax {
		slug = strings.TrimRight(slug[:baseRedSlugMax], "-")
	}
	return BaseRedItemPrefix + slug + "-" + hex.EncodeToString(sum[:4])
}

func failureName(f FailingTest) string {
	return cmp.Or(f.Test, f.Step, strings.Join(f.Jobs, ","))
}

func baseRedItem(rep ClassifyReport, f FailingTest, now time.Time) escalationItem {
	name := failureName(f)
	where := cmp.Or(f.Package, "jobs "+strings.Join(f.Jobs, ", "))
	return escalationItem{
		ID:            baseRedItemID(f),
		CreatedAt:     now.Format(time.RFC3339),
		Weight:        0.95,
		Kind:          "fix",
		Priority:      "critical",
		PriorityClass: inboxbatch.ClassCorrectness,
		Title:         fmt.Sprintf("Fix first: %s is red on the base of %s", name, rep.Target),
		Summary: fmt.Sprintf(
			"evolve ci classify found %s (%s) red on run %d of %s and also red on its base %s. The change did not cause it, so each PR on this base pays for it until main is fixed. Fix it on main first.",
			name, where, rep.RunID, rep.Target, shortCommit(rep.BaseSHA)),
		Evidence: rep.RunURL,
		Source:   BaseRedSource,
	}
}

func shortCommit(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
