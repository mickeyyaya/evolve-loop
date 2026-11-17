package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedset"
)

const (
	shadowRedTestGate = "build:\n\t@true\n" +
		"test:\n\t@printf -- '--- FAIL: TestB (0.00s)\\nFAIL\\nFAIL\\texample.com/m/b\\t0.1s\\n'; exit 1\n" +
		"test-acs-durable:\n\t@true\n" +
		"apicover-enforce:\n\t@true\n"
	shadowGreenGates = "build:\n\t@true\ntest:\n\t@true\ntest-acs-durable:\n\t@true\napicover-enforce:\n\t@true\n"
)

func shadowRebasedLane(t *testing.T, makefile string) (repo *gittest.Repo, auditedBase string) {
	t.Helper()
	repo = gittest.Fixture(t)
	for rel, body := range map[string]string{
		"go/go.mod":   "module example.com/m\n\ngo 1.23\n",
		"go/Makefile": makefile,
		"go/a/a.go":   "package a\n",
		"go/b/b.go":   "package b\n\nimport _ \"example.com/m/a\"\n",
		"go/c/c.go":   "package c\n",
	} {
		writeScopeFile(t, repo.Dir, rel, body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	auditedBase = repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "-b", "lane")
	commitScopeChange(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	repo.Git("checkout", "-q", "main")
	commitScopeChange(t, repo, "go/c/c.go", "package c\n\nfunc C() {}\n")
	repo.Git("checkout", "-q", "lane")
	repo.Git("merge", "-q", "--no-edit", "main")
	return repo, auditedBase
}

func shadowRecords(t *testing.T, plane string) []scopedset.Record {
	t.Helper()
	f, err := os.Open(filepath.Join(plane, ".evolve", scopedset.ArtifactName))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []scopedset.Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r scopedset.Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("shadow line %q is not one record: %v", sc.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func TestComposedGatesTo_ShadowRecordsAnEscapeForARedOutsideTheScopedSet(t *testing.T) {
	t.Parallel()
	repo, base := shadowRebasedLane(t, shadowRedTestGate)
	plane := t.TempDir()
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: plane, Cycle: 7, AuditedBase: base})

	results := composedGatesTo(&bytes.Buffer{})(ctx, repo.Dir)

	wantStatuses := map[string]string{"compile": "pass", "test": "fail", "acs": "pass", "apicover": "pass"}
	if got := ciparity.GateStatuses(results); !reflect.DeepEqual(got, wantStatuses) {
		t.Fatalf("gate statuses = %v, want %v: the shadow must not change the full re-check", got, wantStatuses)
	}
	recs := shadowRecords(t, plane)
	if len(recs) != 1 {
		t.Fatalf("shadow records = %+v, want exactly one for this re-check", recs)
	}
	r := recs[0]
	if r.Stage != "shadow" || r.Cycle != 7 || r.AuditedBase != base || r.All {
		t.Errorf("record header = stage %q cycle %d base %q all %v, want shadow, 7, %q, false", r.Stage, r.Cycle, r.AuditedBase, r.All, base)
	}
	if !reflect.DeepEqual(r.Lane, []string{"go/a/a.go"}) || !reflect.DeepEqual(r.Main, []string{"go/c/c.go"}) {
		t.Errorf("record names lane %v and main %v, want [go/a/a.go] and [go/c/c.go]", r.Lane, r.Main)
	}
	if len(r.Packages) != 0 {
		t.Errorf("the reverse closures of a and c do not meet, so the scoped set is empty, got %v", r.Packages)
	}
	if want := []scopedset.Escape{{Gate: "test", Package: "example.com/m/b", Test: "TestB"}}; !reflect.DeepEqual(r.Escapes, want) {
		t.Errorf("escapes = %+v, want %+v", r.Escapes, want)
	}
}

func TestComposedGatesTo_ShadowRecordsAGreenRecheckWithNoEscape(t *testing.T) {
	t.Parallel()
	repo, base := shadowRebasedLane(t, shadowGreenGates)
	plane := t.TempDir()
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: plane, Cycle: 8, AuditedBase: base})

	results := composedGatesTo(&bytes.Buffer{})(ctx, repo.Dir)

	for gate, outcome := range results {
		if outcome.Status != "pass" {
			t.Errorf("gate %s = %+v, want pass", gate, outcome)
		}
	}
	recs := shadowRecords(t, plane)
	if len(recs) != 1 || recs[0].Cycle != 8 || len(recs[0].Escapes) != 0 {
		t.Errorf("records = %+v, want one record of cycle 8 with no escape: a quiet wave still counts", recs)
	}
}

func TestComposedGatesTo_AnAuditedBaseThatCannotBeReadSelectsTheFullSet(t *testing.T) {
	t.Parallel()
	repo, _ := shadowRebasedLane(t, shadowRedTestGate)
	plane := t.TempDir()
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: plane, Cycle: 9, AuditedBase: "no-such-revision"})

	composedGatesTo(&bytes.Buffer{})(ctx, repo.Dir)

	recs := shadowRecords(t, plane)
	if len(recs) != 1 || !recs[0].All || recs[0].Reason == "" || len(recs[0].Escapes) != 0 {
		t.Errorf("records = %+v, want one record that selects All with a reason and no escape", recs)
	}
}

func TestComposedGatesTo_WithoutARecheckSiteNoShadowRecordIsWritten(t *testing.T) {
	t.Parallel()
	repo, _ := shadowRebasedLane(t, shadowRedTestGate)

	results := composedGatesTo(&bytes.Buffer{})(context.Background(), repo.Dir)

	if results["test"].Status != "fail" || results["compile"].Status != "pass" {
		t.Errorf("results = %+v, want the same full re-check as without a shadow", results)
	}
	if got := shadowRecords(t, repo.Dir); got != nil {
		t.Errorf("a caller that names no site left records %+v in the worktree", got)
	}
}

func TestComposedGatesTo_AShadowRecordThatCannotBeWrittenLeavesTheGatesUntouched(t *testing.T) {
	t.Parallel()
	repo, base := shadowRebasedLane(t, shadowRedTestGate)
	blocker := filepath.Join(t.TempDir(), "plane-is-a-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: blocker, Cycle: 7, AuditedBase: base})

	results := composedGatesTo(&log)(ctx, repo.Dir)

	wantStatuses := map[string]string{"compile": "pass", "test": "fail", "acs": "pass", "apicover": "pass"}
	if got := ciparity.GateStatuses(results); !reflect.DeepEqual(got, wantStatuses) {
		t.Errorf("gate statuses = %v, want %v: a broken evidence sink must not change a gate", got, wantStatuses)
	}
}

func TestComposedGatesTo_ShadowTreatsAnEmptyAuditedBaseAndAnUnnamedRedAsEscapes(t *testing.T) {
	t.Parallel()
	repo, _ := shadowRebasedLane(t, "build:\n\t@exit 1\ntest:\n\t@true\ntest-acs-durable:\n\t@true\napicover-enforce:\n\t@true\n")
	plane := t.TempDir()
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: plane, Cycle: 10})

	composedGatesTo(&bytes.Buffer{})(ctx, repo.Dir)

	recs := shadowRecords(t, plane)
	if len(recs) != 1 || !recs[0].All || recs[0].Reason == "" {
		t.Errorf("records = %+v, want one record that selects All because main cannot be read", recs)
	}
}

func TestRecordScopedShadow_ANamedRedOfAnUnnamedGateIsRecordedAsAnEscape(t *testing.T) {
	t.Parallel()
	repo, base := shadowRebasedLane(t, "build:\n\t@exit 1\ntest:\n\t@true\ntest-acs-durable:\n\t@true\napicover-enforce:\n\t@true\n")
	plane := t.TempDir()
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: plane, Cycle: 11, AuditedBase: base})

	recordScopedShadow(ctx, &bytes.Buffer{}, repo.Dir, map[string]ciparity.GateOutcome{"compile": {Status: "fail"}, "test": {Status: "pass"}})

	recs := shadowRecords(t, plane)
	if len(recs) != 1 || !reflect.DeepEqual(recs[0].Escapes, []scopedset.Escape{{Gate: "compile"}}) {
		t.Errorf("records = %+v, want one escape naming only the compile gate", recs)
	}
}

func TestRecordScopedShadow_AWorktreeThatIsNoRepositorySelectsTheFullSet(t *testing.T) {
	t.Parallel()
	plane := t.TempDir()
	ctx := core.WithRecheckSite(context.Background(), core.RecheckSite{ProjectRoot: plane, Cycle: 12, AuditedBase: "x"})

	recordScopedShadow(ctx, &bytes.Buffer{}, t.TempDir(), nil)

	recs := shadowRecords(t, plane)
	if len(recs) != 1 || !recs[0].All || recs[0].Reason == "" {
		t.Errorf("records = %+v, want one record that selects All with a reason", recs)
	}
}

func TestSortedPaths_DropsBlanksAndDuplicatesAndSorts(t *testing.T) {
	t.Parallel()
	got := sortedPaths([]string{"b", "", "a", "b"})
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sortedPaths = %v, want %v", got, want)
	}
}
