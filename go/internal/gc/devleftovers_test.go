package gc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebuildableIgnored_PipelineOutputAtAnyDepth(t *testing.T) {
	cases := map[string]bool{
		".evolve/": true, ".evolve/runs/cycle-1/scout-report.md": true, "lane/.evolve/": true, "a/b/.evolve/state.json": true,
		".commit-gate/": true, ".commit-gate/attestation.json": true, "lane/.commit-gate/": true, "a/b/.commit-gate/x.json": true,
		".evolve-old/": false, "evolve/": false, "my.evolve/": false, "x.commit-gate/": false, ".commit-gate-old/": false,
		".env": false, ".envrc": false,
	}
	for entry, want := range cases {
		if got := rebuildableIgnored(entry); got != want {
			t.Errorf("rebuildableIgnored(%q) = %v, want %v", entry, got, want)
		}
	}
}

func TestPlanDevWorktrees_ATreeHoldingOnlyPipelineOutputIsArchived(t *testing.T) {
	e := newDevEnv(t)
	e.exclude(".evolve/", ".commit-gate/")
	dir := e.addDev("pipeline")
	writeRepoFile(t, dir, ".commit-gate/attestation.json", "{}\n")
	writeRepoFile(t, dir, ".evolve/runs/cycle-1/scout-report.md", "report\n")
	writeRepoFile(t, dir, "lane/.evolve/state.json", "{}\n")
	writeRepoFile(t, dir, "lane/.commit-gate/attestation.json", "{}\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionArchive {
		t.Fatalf("item = %+v, want archive: every ignored entry is pipeline output", it)
	}
}

func TestPlanDevWorktrees_APipelineOnlyCommitGateTreeIsArchived(t *testing.T) {
	e := newDevEnv(t)
	e.exclude(".commit-gate/")
	dir := e.addDev("gateonly")
	writeRepoFile(t, dir, ".commit-gate/attestation.json", "{}\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionArchive {
		t.Fatalf("item = %+v, want archive: .commit-gate/attestation.json is the only ignored file", it)
	}
}

func TestPlanDevWorktrees_AnEnvFileBesidePipelineOutputKeepsTheTree(t *testing.T) {
	e := newDevEnv(t)
	e.exclude(".evolve/", ".commit-gate/", ".env")
	dir := e.addDev("envbeside")
	writeRepoFile(t, dir, ".commit-gate/attestation.json", "{}\n")
	writeRepoFile(t, dir, ".evolve/runs/cycle-1/scout-report.md", "report\n")
	writeRepoFile(t, dir, ".env", "KEY=1\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !strings.HasPrefix(it.Reason, "holds ignored files: ") || !strings.Contains(it.Reason, ".env") {
		t.Fatalf("item = %+v, want kept for .env", it)
	}
	if strings.Contains(it.Reason, ".commit-gate") || strings.Contains(it.Reason, ".evolve/") {
		t.Errorf("reason %q names pipeline output, want only the held file", it.Reason)
	}
}

func TestPlanDevWorktrees_AnIgnoredFileInsidePipelineOutputDirectoryStaysRebuildable(t *testing.T) {
	e := newDevEnv(t)
	e.exclude(".evolve/", ".env")
	dir := e.addDev("envinside")
	writeRepoFile(t, dir, ".evolve/runs/cycle-1/scout-report.md", "report\n")
	writeRepoFile(t, dir, "sub/.env", "KEY=1\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "sub/.env") {
		t.Fatalf("item = %+v, want kept for sub/.env", it)
	}
}

func TestPlanDevWorktrees_ANestedRepositoryBesidePipelineOutputKeepsTheTree(t *testing.T) {
	e := newDevEnv(t)
	e.exclude(".evolve/", ".commit-gate/")
	dir := e.addDev("nestedbeside")
	writeRepoFile(t, dir, ".commit-gate/attestation.json", "{}\n")
	writeRepoFile(t, dir, ".evolve/runs/cycle-1/scout-report.md", "report\n")
	inner := filepath.Join(dir, "vendor", "lib")
	if err := os.MkdirAll(filepath.Join(inner, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, inner, "inner.txt", "inner work\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "holds a nested git repository") {
		t.Fatalf("item = %+v, want kept for the nested repository", it)
	}
}
