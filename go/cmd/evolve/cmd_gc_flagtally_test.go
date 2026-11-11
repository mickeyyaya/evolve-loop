package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestGCFlagCountSuffix(t *testing.T) {
	t.Parallel()
	it := gc.WorktreeItem{Branch: "b", Action: gc.WorktreeActionFlagUnmerged}
	if got := gcFlagCountSuffix(map[string]int{gc.FlagKey(it): 3}, it); got != " count=3" {
		t.Errorf("suffix = %q", got)
	}
	if got := gcFlagCountSuffix(nil, it); got != "" {
		t.Errorf("a dry run prints no count, got %q", got)
	}
}

func TestGCRunFlagCounts_DryRunTouchesNothingAndAFailureIsReported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := gc.WorktreeManifest{Items: []gc.WorktreeItem{{Branch: "b", Action: gc.WorktreeActionFlagUnmerged}}}
	var out, errb bytes.Buffer
	dry := newGCRun(context.Background(), true, &out, &errb)
	if counts, failed := dry.flagCounts(gc.WorktreeOptions{EvolveDir: dir}, m); counts != nil || failed {
		t.Errorf("dry run counts=%v failed=%v", counts, failed)
	}
	if _, err := os.Stat(filepath.Join(dir, gc.FlagTallyFile)); err == nil {
		t.Error("a dry run wrote the tally")
	}
	if err := os.WriteFile(filepath.Join(dir, gc.FlagTallyFile), []byte("[1]"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := newGCRun(context.Background(), false, &out, &errb)
	if counts, failed := live.flagCounts(gc.WorktreeOptions{EvolveDir: dir}, m); counts != nil || !failed {
		t.Errorf("a failed tally must report: counts=%v failed=%v", counts, failed)
	}
	if !strings.Contains(errb.String(), "flag tally failed") {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestGCRunFileRepeatedFlags_ReportsFilingAndFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	it := gc.WorktreeItem{Branch: "b", Action: gc.WorktreeActionFlagUnmerged, Reason: "r"}
	m := gc.WorktreeManifest{Items: []gc.WorktreeItem{it}}
	counts := map[string]int{gc.FlagKey(it): gc.FlagRepeatThreshold}
	var out, errb bytes.Buffer
	r := newGCRun(context.Background(), false, &out, &errb)
	if r.fileRepeatedFlags(gc.WorktreeOptions{EvolveDir: dir}, m, counts) || !strings.Contains(out.String(), "filed inbox item") {
		t.Errorf("out=%q err=%q", out.String(), errb.String())
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.fileRepeatedFlags(gc.WorktreeOptions{EvolveDir: blocker}, m, counts) || !strings.Contains(errb.String(), "flag filing failed") {
		t.Errorf("err=%q", errb.String())
	}
}
