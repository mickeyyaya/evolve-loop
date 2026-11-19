package research

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFileKB_RecallsIncidentLessonAndSkipsLegacyAndIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "incidents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `---
schema: incident-record/1
id: 2026-11-19-sample-recall
title: sample incident recall
level: record
status: fixed
fingerprint: sample:recall
signals: ["cycle:10:1"]
prs: [101]
items: [item-1]
actions: []
effectiveness:
  metric: {code: SAMPLE_CODE, filters: [], unit: wave, since: abc123}
  baseline: {value: 2, units: 1}
  target: 0
  checks: 2
  results: []
verified_at: ""
---

# sample incident recall

## Summary

## Trigger and level

## Timeline

## Key logs

## Root cause and contributing factors

## Analysis

## Approaches taken

## Actions

## Effectiveness

## Lessons

First lesson line.

Second paragraph holds capybara keyword.

## References

A narwhal reference outside Lessons.
`
	if err := os.WriteFile(filepath.Join(dir, "2026-11-19-sample-recall.md"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-08-09-legacy.md"), []byte("# legacy\n\n## Lessons\n\nA platypus must not surface.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "REGRESSION-COVERAGE-INDEX.md"), []byte("# index\n\nplatypus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-19-other-schema.md"), []byte("---\nschema: other/1\ntitle: platypus\n---\n\n## Lessons\n\nplatypus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-11-19-malformed.md"), []byte("---\nschema: incident-record/1\nbogus: 1\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	var roots []string
	for _, rel := range SearchPathsFromEnv(policy.PathsConfig{}) {
		roots = append(roots, filepath.Join(root, rel))
	}
	kb := NewFileKB(roots)

	got, err := kb.Lookup(context.Background(), Query{Keywords: []string{"capybara"}})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !slices.Equal(ids(got), []string{"2026-11-19-sample-recall"}) {
		t.Errorf("Lookup(capybara) = %v, want [2026-11-19-sample-recall]", ids(got))
	}

	for _, kw := range []string{"platypus", "narwhal"} {
		excluded, err := kb.Lookup(context.Background(), Query{Keywords: []string{kw}})
		if err != nil {
			t.Fatalf("Lookup(%s): %v", kw, err)
		}
		if len(excluded) != 0 {
			t.Errorf("Lookup(%s) = %v, want empty", kw, ids(excluded))
		}
	}

	if _, err := parseLessonFile(filepath.Join(dir, "missing.md")); !os.IsNotExist(err) {
		t.Errorf("parseLessonFile(missing.md) err = %v, want not-exist", err)
	}
}
