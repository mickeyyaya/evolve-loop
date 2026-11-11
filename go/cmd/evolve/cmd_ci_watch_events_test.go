package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const ciWatchEventSHA = "0123456789abcdef0123456789abcdef01234567"

func fakeGHReportingRun(t *testing.T, conclusion string) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
api) printf '{"sha":"` + ciWatchEventSHA + `"}' ;;
run)
  if [ "$2" = "list" ]; then
    printf '[{"status":"completed","conclusion":"` + conclusion + `","url":"https://github.com/o/r/actions/runs/9","databaseId":9}]'
  else
    exit 1
  fi ;;
*) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCIWatchVerb_PublishesCICompletedForEachWatchedWorkflow(t *testing.T) {
	useRealRootEvents(t)
	fakeGHReportingRun(t, "success")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runCIWatch([]string{"--sha", ciWatchEventSHA, "--workflow", "required.yml", "--workflow", "release.yml", "--project-root", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	got := channelledEvents(t, filepath.Join(root, ".evolve"), "signals", signalcenter.KindCICompleted)
	if len(got) != 2 {
		t.Fatalf("ci.completed events = %+v, want one for each watched workflow", got)
	}
	workflows := map[string]bool{}
	for _, e := range got {
		if e.Module != signalcenter.ModuleCI || e.Severity != signalcenter.SeverityInfo || e.Fields["sha"] != ciWatchEventSHA || e.Fields["conclusion"] != "success" || e.Fields["run_id"] != "9" {
			t.Errorf("event = %+v, want an INFO ci event for sha %s, conclusion success, run 9", e, ciWatchEventSHA)
		}
		workflows[e.Fields["workflow"]] = true
	}
	if !workflows["required.yml"] || !workflows["release.yml"] {
		t.Errorf("workflows named by the events = %v, want required.yml and release.yml", workflows)
	}
}

func TestCIWatchVerb_ARedRunReachesTheErrorsChannelWithItsCode(t *testing.T) {
	useRealRootEvents(t)
	fakeGHReportingRun(t, "failure")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runCIWatch([]string{"--sha", ciWatchEventSHA, "--project-root", root}, &stdout, &stderr)

	if code != exitRefused {
		t.Fatalf("exit = %d, want %d for a red run; stderr:\n%s", code, exitRefused, stderr.String())
	}
	got := channelledEvents(t, filepath.Join(root, ".evolve"), "errors", signalcenter.KindCICompleted)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn || got[0].Code != signalcenter.CodeCIRunRed || got[0].Fields["workflow"] != "required.yml" {
		t.Errorf("errors channel ci.completed events = %+v, want one WARN CI_RUN_RED for required.yml", got)
	}
}

func TestCIWatchVerb_ATagWatchPublishesOneEventForEachReleaseWorkflow(t *testing.T) {
	useRealRootEvents(t)
	fakeGHReportingRun(t, "success")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runCIWatch([]string{"--tag", "v1.2.3", "--project-root", root}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if got := channelledEvents(t, filepath.Join(root, ".evolve"), "signals", signalcenter.KindCICompleted); len(got) != 2 {
		t.Errorf("ci.completed events = %+v, want one for required.yml and one for release.yml", got)
	}
}
