package dossier

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func validWriteDossier() *Dossier {
	return &Dossier{Cycle: 7, Goal: "backfill", FinalVerdict: VerdictPass,
		Phases: []PhaseRecord{{Name: "build", Verdict: VerdictPass}}}
}

func TestWrite_EachFailingStepReturnsItsWrappedError(t *testing.T) {
	cases := []struct {
		name     string
		arrange  func(t *testing.T, d *Dossier) (dir string, commit bool)
		want     string
		jsonLeft bool
	}{
		{"unmarshalable signal", func(t *testing.T, d *Dossier) (string, bool) {
			d.Phases[0].Signals = map[string]any{"score": math.Inf(1)}
			return t.TempDir(), false
		}, "dossier: Write: dossier: RenderJSON: json: unsupported value: +Inf", false},
		{"invalid dossier", func(t *testing.T, d *Dossier) (string, bool) {
			d.Goal = "  "
			return t.TempDir(), false
		}, "dossier: Write: dossier: RenderJSON: dossier: cycle 7: goal is empty", false},
		{"dir is a file", func(t *testing.T, _ *Dossier) (string, bool) {
			file := filepath.Join(t.TempDir(), "plain")
			if err := os.WriteFile(file, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			return file, false
		}, "dossier: Write JSON:", false},
		{"markdown target is a directory", func(t *testing.T, _ *Dossier) (string, bool) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "cycle-7.md", "occupied"), 0o755); err != nil {
				t.Fatal(err)
			}
			return dir, false
		}, "dossier: Write markdown:", true},
		{"commit outside a repo", func(t *testing.T, _ *Dossier) (string, bool) {
			return t.TempDir(), true
		}, "dossier: Write commit:", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validWriteDossier()
			dir, commit := tc.arrange(t, d)

			err := Write(d, dir, commit)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			_, statErr := os.Stat(filepath.Join(dir, "cycle-7.json"))
			if (statErr == nil) != tc.jsonLeft {
				t.Errorf("cycle-7.json present = %v, want %v", statErr == nil, tc.jsonLeft)
			}
		})
	}
}

func TestWrite_MarkdownRenderFailureStopsBeforeTheMarkdownFile(t *testing.T) {
	orig := renderMarkdown
	renderMarkdown = func(*Dossier) ([]byte, error) { return nil, errors.New("template boom") }
	t.Cleanup(func() { renderMarkdown = orig })
	dir := t.TempDir()

	err := Write(validWriteDossier(), dir, false)

	if err == nil || err.Error() != "dossier: Write: template boom" {
		t.Fatalf("err = %v, want dossier: Write: template boom", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "cycle-7.md")); !os.IsNotExist(statErr) {
		t.Errorf("cycle-7.md stat = %v, want it never written", statErr)
	}
}

func TestSweepOrphans_LeavesATrackedButModifiedPairAlone(t *testing.T) {
	r := gittest.Fixture(t)
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-5.json", `{"cycle":5}`)
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-5.md", "# five")
	r.Git("add", "--", "knowledge-base")
	r.Git("commit", "-q", "-m", "seed")
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-5.json", `{"cycle":5,"edited":true}`)
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-5.md", "# five edited")
	before := r.Git("rev-parse", "HEAD")
	var log bytes.Buffer

	res, err := SweepOrphans(gitexec.Default(r.Dir), &log)

	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if len(res.Recommitted) != 0 || len(res.Skipped) != 0 || len(res.Failed) != 0 {
		t.Errorf("result = %+v, want a tracked pair neither recommitted, skipped nor failed", res)
	}
	if after := r.Git("rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved %s -> %s, want no commit for a tracked pair", before, after)
	}
	if log.Len() != 0 {
		t.Errorf("log = %q, want silence", log.String())
	}
}

func TestSweepOrphans_HalfTrackedPairIsNotRecommitted(t *testing.T) {
	r := gittest.Fixture(t)
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-6.md", "# six")
	r.Git("add", "--", "knowledge-base")
	r.Git("commit", "-q", "-m", "seed md only")
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-6.json", `{"cycle":6}`)
	writePairFile(t, r.Dir, "knowledge-base/cycles/cycle-6.md", "# six edited")

	res, err := SweepOrphans(gitexec.Default(r.Dir), &bytes.Buffer{})

	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if len(res.Recommitted) != 0 || len(res.Skipped) != 0 {
		t.Errorf("result = %+v, want the half-tracked pair left alone", res)
	}
}

func TestAnyTracked_EmptyNamesNeverAskGit(t *testing.T) {
	r := gittest.Fixture(t)
	writePairFile(t, r.Dir, "tracked.txt", "x")
	r.Git("add", "tracked.txt")
	r.Git("commit", "-q", "-m", "seed")
	g := gitexec.Default(r.Dir)
	ctx := context.Background()

	cases := []struct {
		name  string
		files []string
		want  bool
	}{
		{"no names", nil, false},
		{"only empty names", []string{"", ""}, false},
		{"untracked name", []string{"", "absent.txt"}, false},
		{"tracked name beside an empty one", []string{"tracked.txt", ""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := anyTracked(ctx, g, tc.files...); got != tc.want {
				t.Errorf("anyTracked(%q) = %v, want %v", tc.files, got, tc.want)
			}
		})
	}
}

func TestGroupOrphanPairs_KeepsOnlyDirectChildrenOfTheDossierDir(t *testing.T) {
	dirty := []string{
		"knowledge-base/cycles/cycle-3.json",
		"knowledge-base/cycles/cycle-3.md",
		"knowledge-base/cycles/cycle-4.md",
		"knowledge-base/cycles/nested/cycle-8.json",
		"knowledge-base/cycle-9.json",
		"other/knowledge-base/cycles/cycle-10.json",
		"knowledge-base/cycles/cycle-11.txt",
		"knowledge-base/cycles/README.md",
		"knowledge-base/cycles/cycle-x.json",
		"knowledge-base/cycles/cycle-99999999999999999999.json",
		"knowledge-base/cycles/cycle-é.md",
		"",
	}

	got := groupOrphanPairs(dirty)

	want := map[int]*orphanPairPaths{
		3: {json: "knowledge-base/cycles/cycle-3.json", md: "knowledge-base/cycles/cycle-3.md"},
		4: {md: "knowledge-base/cycles/cycle-4.md"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groupOrphanPairs = %v, want %v", describePairs(got), describePairs(want))
	}
}

func TestGroupOrphanPairs_IsOrderIndependent(t *testing.T) {
	a := []string{"knowledge-base/cycles/cycle-1.md", "knowledge-base/cycles/cycle-1.json", "knowledge-base/cycles/cycle-2.json"}
	b := []string{a[2], a[1], a[0]}

	if !reflect.DeepEqual(groupOrphanPairs(a), groupOrphanPairs(b)) {
		t.Errorf("grouping depends on input order: %v vs %v", describePairs(groupOrphanPairs(a)), describePairs(groupOrphanPairs(b)))
	}
}

func FuzzGroupOrphanPairs_EveryPairLivesInTheDossierDir(f *testing.F) {
	f.Add("knowledge-base/cycles/cycle-3.json")
	f.Add("knowledge-base/cycles/cycle-3.md")
	f.Add("knowledge-base/cycles/sub/cycle-1.md")
	f.Add("cycle-0.json")
	f.Fuzz(func(t *testing.T, p string) {
		for n, pp := range groupOrphanPairs([]string{p}) {
			for _, got := range []string{pp.json, pp.md} {
				if got != "" && filepath.Dir(got) != dossierDir {
					t.Fatalf("cycle %d pair path %q is outside %s", n, got, dossierDir)
				}
			}
			if pp.json != "" && !strings.HasSuffix(pp.json, ".json") || pp.md != "" && !strings.HasSuffix(pp.md, ".md") {
				t.Fatalf("cycle %d pair %+v has a wrong extension", n, *pp)
			}
		}
	})
}

func describePairs(m map[int]*orphanPairPaths) map[int]orphanPairPaths {
	out := map[int]orphanPairPaths{}
	for n, pp := range m {
		out[n] = *pp
	}
	return out
}
