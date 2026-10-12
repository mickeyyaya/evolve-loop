package core

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestReadContinuationFindings_RendersReasonsOrFallsBackLoudly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	reasons := filepath.Join(dir, "reasons.json")
	writeFile(t, reasons, `{"schema_version":1,"phase":"audit","reasons":["export X unnamed","race in Y"]}`)
	noReasons := filepath.Join(dir, "no-reasons.json")
	writeFile(t, noReasons, `{"phase":"audit","reasons":[]}`)
	raw := filepath.Join(dir, "raw.txt")
	writeFile(t, raw, "  plain findings line\n")
	cases := []struct {
		name     string
		path     string
		want     string
		wantWarn bool
	}{
		{"no path names no findings", "", "", false},
		{"a missing artifact warns and serves nothing", filepath.Join(dir, "absent.json"), "", true},
		{"reason JSON renders as lines", reasons, "failed phase: audit\n- export X unnamed\n- race in Y", false},
		{"reason JSON without reasons falls back to the raw body", noReasons, `{"phase":"audit","reasons":[]}`, false},
		{"plain text is trimmed", raw, "plain findings line", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var sink injectedStderr
			got = readContinuationFindings(tc.path, &sink)
			log := sink.String()
			if got != tc.want {
				t.Fatalf("findings = %q, want %q", got, tc.want)
			}
			if warned := strings.Contains(log, "WARN continuation: findings artifact "+tc.path+" unreadable"); warned != tc.wantWarn {
				t.Fatalf("warned = %v, want %v (stderr %q)", warned, tc.wantWarn, log)
			}
		})
	}
}

func FuzzReadContinuationFindings_IsBoundedAndRendersEveryReason(f *testing.F) {
	f.Add([]byte(`{"phase":"build","reasons":["a","b"]}`))
	f.Add([]byte(`{"phase":"","reasons":[""]}`))
	f.Add([]byte("not json — ünïcødé\n"))
	f.Add([]byte(strings.Repeat("x\n", maxFindingsBytes)))
	f.Add([]byte(`{"reasons":"not a list"}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		path := filepath.Join(t.TempDir(), "findings.json")
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		got := readContinuationFindings(path, io.Discard)
		if len(got) > maxFindingsBytes+len("\n…[truncated  bytes]")+20 {
			t.Fatalf("findings of %d bytes exceed the budget", len(got))
		}
		if utf8.Valid(body) && strings.Count(string(body), "\n") > 0 && !utf8.ValidString(got) {
			t.Fatalf("a line-aligned cut split a rune: %q", got)
		}
		var a auditFailReason
		if json.Unmarshal(body, &a) == nil && len(a.Reasons) > 0 {
			if !strings.HasPrefix(got, "failed phase: "+a.Phase+"\n- ") {
				t.Fatalf("reason JSON rendered as %q", got)
			}
			return
		}
		if trimmed := strings.TrimSpace(string(body)); len(trimmed) <= maxFindingsBytes && got != trimmed {
			t.Fatalf("raw body served as %q, want %q", got, trimmed)
		}
	})
}

func TestPrepareAdoptedTree_AnArchiveFailureStopsAdoptionWithItsStage(t *testing.T) {
	t.Parallel()

	_, wt := initContinuationRepo(t, 61)
	base := gitOut(t, wt, "rev-parse", "HEAD")
	cr := &cycleRun{ctx: context.Background(), cycle: 62}

	err := cr.prepareAdoptedTree(wt, "0000000000000000000000000000000000000000")
	if err == nil || !strings.HasPrefix(err.Error(), "continuation unpublished explanation archive: ") {
		t.Fatalf("an unresolvable base = %v, want the explanation-archive stage named", err)
	}

	predicate := filepath.Join(wt, "go", "acs", "cycle61", "predicates_test.go")
	writeFile(t, predicate, "//go:build acs\n\npackage cycle61\n")
	gitOut(t, wt, "add", "go/acs")
	gitOut(t, wt, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "ancestor predicate")
	if err := cr.prepareAdoptedTree(wt, base); err != nil {
		t.Fatalf("first adoption must archive the ancestor package: %v", err)
	}
	archived, _ := filepath.Glob(filepath.Join(wt, "docs", "private", "research", "archived-*", "superseded-predicate-packages", "cycle61", "predicates_test.go"))
	if len(archived) != 1 {
		t.Fatalf("archived ancestor package = %v, want one", archived)
	}
	writeFile(t, predicate, "//go:build acs\n\npackage cycle61\n")

	err = cr.prepareAdoptedTree(wt, base)

	if err == nil || !strings.HasPrefix(err.Error(), "continuation superseded predicate archive: ") || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a second archive of the same package = %v, want the predicate-archive stage refusing to clobber", err)
	}
	if _, statErr := os.Stat(predicate); statErr != nil {
		t.Fatalf("a refused archive must leave the package in place: %v", statErr)
	}
}

type createOnlyProvisionerE4 struct{ created int }

func (p *createOnlyProvisionerE4) Create(string, int) (string, error) { p.created++; return "", nil }
func (p *createOnlyProvisionerE4) Cleanup(string, string) error       { return nil }

type plantingProvisionerE4 struct {
	gitWorktree
	plant func(wt string) error
}

func (p plantingProvisionerE4) CreateFrom(root string, cycle int, ref string) (string, error) {
	wt, err := p.gitWorktree.CreateFrom(root, cycle, ref)
	if err == nil && p.plant != nil {
		err = p.plant(wt)
	}
	return wt, err
}

func adoptionRunE4(t *testing.T, root string, m continuation.Continuation, wp WorktreeProvisioner, st *fakeStorage, ws string) *cycleRun {
	t.Helper()
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithContinuationResolver(func(string, int, []string) *continuation.Continuation { c := m; return &c }),
		WithWorktreeProvisioner(wp))
	return &cycleRun{
		o: o, ctx: context.Background(), cycle: 72, req: CycleRequest{ProjectRoot: root},
		cs:      CycleState{CycleID: 72, WorkspacePath: ws, ActiveWorktree: "fresh-worktree"},
		ctxSnap: map[string]string{},
	}
}

func TestAdoptContinuationAfterTriage_AProvisionerThatCannotSeedKeepsTheFreshWorktree(t *testing.T) {
	t.Parallel()

	root, wt := initContinuationRepo(t, 71)
	m := stampedContinuation(t, root, wt, 71)
	ws := t.TempDir()
	cr := adoptionRunE4(t, root, m, &createOnlyProvisionerE4{}, &fakeStorage{}, ws)

	if err := cr.adoptContinuationAfterTriage(); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if cr.cs.ActiveWorktree != "fresh-worktree" || cr.cs.WorktreeBaseSHA != "" {
		t.Fatalf("cycle state = %+v, want the fresh worktree untouched", cr.cs)
	}
	if _, ok, _ := continuation.ReadManifest(ws); ok {
		t.Fatal("no manifest may be copied when nothing was adopted")
	}
}

func TestAdoptContinuationAfterTriage_EachPostSeedFailureIsReturnedWithTheSeededIdentityKept(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		plant   func(wt string) error
		storage *fakeStorage
		wsFile  bool
		want    string
	}{
		{"the adopted tree cannot be prepared", func(wt string) error {
			return os.WriteFile(filepath.Join(wt, "docs"), []byte("a file where the docs dir belongs"), 0o644)
		}, &fakeStorage{}, false, "continuation unpublished explanation archive: "},
		{"the cycle state cannot persist", nil, &fakeStorage{failOnWriteCS: true}, false, "continuation cycle-state persist after adoption: write CS forced fail"},
		{"the workspace manifest cannot be written", nil, &fakeStorage{}, true, "continuation workspace manifest copy: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, wt := initContinuationRepo(t, 71)
			m := stampedContinuation(t, root, wt, 71)
			ws := t.TempDir()
			if tc.wsFile {
				ws = filepath.Join(ws, "workspace-is-a-file")
				writeFile(t, ws, "x")
			}
			cr := adoptionRunE4(t, root, m, plantingProvisionerE4{plant: tc.plant}, tc.storage, ws)

			err := cr.adoptContinuationAfterTriage()

			if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
				t.Fatalf("adopt = %v, want prefix %q", err, tc.want)
			}
			if cr.cs.ActiveWorktree == "fresh-worktree" || cr.cs.ActiveWorktree == "" {
				t.Fatalf("the seeded worktree identity must be kept for closeout, got %q", cr.cs.ActiveWorktree)
			}
			if got := gitOut(t, cr.cs.ActiveWorktree, "rev-parse", "HEAD"); got == "" {
				t.Fatal("the kept worktree must be a real checkout")
			}
			if cr.ctxSnap["continuation_findings"] != "" {
				t.Fatal("a failed adoption must not serve findings")
			}
		})
	}
}
