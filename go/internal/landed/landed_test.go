package landed_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/landed"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func lines(changed map[int]string) string {
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		line := fmt.Sprintf("line %d", i)
		if c, ok := changed[i]; ok {
			line = c
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

type tree struct {
	t    *testing.T
	repo *gittest.Repo
	base string
}

func newTree(t *testing.T, files map[string]string) tree {
	t.Helper()
	r := gittest.Fixture(t)
	tr := tree{t: t, repo: r}
	tr.writeAll(files)
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "base")
	tr.base = r.Git("rev-parse", "HEAD")
	r.Git("update-ref", plane.OriginMainRef, "HEAD")
	return tr
}

func (tr tree) path(name string) string { return filepath.Join(tr.repo.Dir, filepath.FromSlash(name)) }

func (tr tree) write(name, content string) {
	tr.t.Helper()
	if err := os.MkdirAll(filepath.Dir(tr.path(name)), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(tr.path(name), []byte(content), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) writeAll(files map[string]string) {
	tr.t.Helper()
	for name, content := range files {
		tr.write(name, content)
	}
}

func (tr tree) landOnMain(edit func()) {
	tr.t.Helper()
	tr.repo.Git("switch", "-q", "-c", "mainline", plane.OriginMainRef)
	edit()
	tr.repo.Git("add", "-A")
	tr.repo.Git("commit", "-q", "--allow-empty", "-m", "main moves on")
	tr.repo.Git("update-ref", plane.OriginMainRef, "HEAD")
	tr.repo.Git("switch", "-q", "main")
	tr.repo.Git("branch", "-q", "-D", "mainline")
}

func (tr tree) changes() landed.Verdict {
	tr.t.Helper()
	v, err := landed.Changes(context.Background(), gitexec.Default(tr.repo.Dir), tr.base)
	if err != nil {
		tr.t.Fatalf("Changes: %v", err)
	}
	return v
}

func TestChanges_NoChangeIsLanded(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"big.txt": lines(nil)})

	if v := tr.changes(); !v.Landed {
		t.Errorf("an unchanged tree = %+v, want landed", v)
	}
}

func TestChanges_AChangeMainCarriesIsLandedEvenAfterMainEditedTheFileElsewhere(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"big.txt": lines(nil)})
	tr.landOnMain(func() { tr.write("big.txt", lines(map[int]string{3: "lane edit", 18: "later main edit"})) })
	tr.write("big.txt", lines(map[int]string{3: "lane edit"}))

	if v := tr.changes(); !v.Landed {
		t.Errorf("a landed edit beside a later main edit = %+v, want landed", v)
	}
}

func TestChanges_AnEditWhoseTextExistsInAnIdenticalBlockElsewhereIsNotLanded(t *testing.T) {
	t.Parallel()
	block := func(mid string) string {
		return "ctx one\nctx two\nctx three\n" + mid + "\nctx four\nctx five\nctx six\n"
	}
	file := func(first, second string) string {
		return "start\n" + block(first) + lines(nil)[:40] + block(second) + "end\n"
	}
	tr := newTree(t, map[string]string{"repeated.txt": file("OLD", "NEW")})
	tr.write("repeated.txt", file("NEW", "NEW"))

	if v := tr.changes(); v.Landed || !strings.Contains(v.Reason, "repeated.txt") {
		t.Errorf("an edit main lacks at its own place = %+v, want not landed naming repeated.txt", v)
	}
}

func TestChanges_AnExtraUnlandedHunkIsNotLanded(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"big.txt": lines(nil)})
	tr.landOnMain(func() { tr.write("big.txt", lines(map[int]string{2: "landed"})) })
	tr.write("big.txt", lines(map[int]string{2: "landed", 18: "unlanded"}))

	if v := tr.changes(); v.Landed || !strings.Contains(v.Reason, "big.txt") {
		t.Errorf("an extra hunk = %+v, want not landed naming big.txt", v)
	}
}

func TestChanges_ADeletionIsLandedOnlyWhenMainDeletedTheFileToo(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"big.txt": lines(nil), "gone.txt": "bye\n"})
	if err := os.Remove(tr.path("gone.txt")); err != nil {
		t.Fatal(err)
	}
	if v := tr.changes(); v.Landed || !strings.Contains(v.Reason, "gone.txt") {
		t.Errorf("a deletion main lacks = %+v, want not landed naming gone.txt", v)
	}

	tr.landOnMain(func() { tr.repo.Git("rm", "-q", "gone.txt") })

	if v := tr.changes(); !v.Landed {
		t.Errorf("a deletion main also made = %+v, want landed", v)
	}
}

func TestChanges_AModeChangeMainLacksIsNotLanded(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"tool.sh": "echo tool\n"})
	if err := os.Chmod(tr.path("tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if v := tr.changes(); v.Landed || !strings.Contains(v.Reason, "tool.sh") {
		t.Errorf("a mode change main lacks = %+v, want not landed naming tool.sh", v)
	}

	tr.landOnMain(func() {
		if err := os.Chmod(tr.path("tool.sh"), 0o755); err != nil {
			t.Fatal(err)
		}
	})

	if v := tr.changes(); !v.Landed {
		t.Errorf("a mode change main also made = %+v, want landed", v)
	}
}

func TestChanges_ABinaryOrSymlinkChangeIsLandedOnlyWhenEqualToMain(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"blob.bin": "a\x00b\n", "target.txt": "x\n", "other.txt": "y\n"})
	if err := os.Symlink("target.txt", tr.path("link")); err != nil {
		t.Fatal(err)
	}
	tr.repo.Git("add", "link")
	tr.repo.Git("commit", "-q", "-m", "a link")
	tr.base = tr.repo.Git("rev-parse", "HEAD")
	tr.repo.Git("update-ref", plane.OriginMainRef, "HEAD")
	tr.landOnMain(func() { tr.write("blob.bin", "a\x00c\nmore\n") })
	tr.write("blob.bin", "a\x00c\n")
	if v := tr.changes(); v.Landed || !strings.Contains(v.Reason, "blob.bin differs") {
		t.Errorf("a binary change that differs from main = %+v, want not landed naming blob.bin", v)
	}

	tr.write("blob.bin", "a\x00c\nmore\n")
	if err := os.Remove(tr.path("link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other.txt", tr.path("link")); err != nil {
		t.Fatal(err)
	}
	if v := tr.changes(); v.Landed || !strings.Contains(v.Reason, "link differs") {
		t.Errorf("a retargeted symlink main lacks = %+v, want not landed naming link", v)
	}
}

func TestListUntracked_HashesEveryUntrackedFileInNameOrderAndSkipsIgnored(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{".gitignore": "*.log\n"})
	tr.writeAll(map[string]string{"b.txt": "bee\n", "dir/a.txt": "ay\n", "build.log": "ignored\n"})

	files, err := landed.ListUntracked(context.Background(), gitexec.Default(tr.repo.Dir))

	if err != nil {
		t.Fatal(err)
	}
	want := []landed.UntrackedFile{
		{Name: "b.txt", Hash: tr.repo.Git("hash-object", "b.txt")},
		{Name: "dir/a.txt", Hash: tr.repo.Git("hash-object", "dir/a.txt")},
	}
	if fmt.Sprint(files) != fmt.Sprint(want) {
		t.Errorf("ListUntracked = %v, want %v", files, want)
	}
}

func TestUntracked_AFileMissingFromOrDifferentInMainIsNotLanded(t *testing.T) {
	t.Parallel()
	tr := newTree(t, map[string]string{"seed.txt": "seed\n"})
	tr.landOnMain(func() { tr.writeAll(map[string]string{"same.txt": "same\n", "other.txt": "main's\n"}) })
	tr.writeAll(map[string]string{"same.txt": "same\n", "other.txt": "lane's\n", "new.txt": "new\n"})
	git := gitexec.Default(tr.repo.Dir)
	all, err := landed.ListUntracked(context.Background(), git)
	if err != nil {
		t.Fatal(err)
	}
	pick := func(name string) []landed.UntrackedFile {
		for _, f := range all {
			if f.Name == name {
				return []landed.UntrackedFile{f}
			}
		}
		t.Fatalf("%s is not listed as untracked: %v", name, all)
		return nil
	}

	for name, want := range map[string]string{"same.txt": "", "other.txt": "untracked other.txt differs from origin/main", "new.txt": "untracked new.txt is not in origin/main"} {
		v, err := landed.Untracked(context.Background(), git, pick(name))
		if err != nil {
			t.Fatal(err)
		}
		if v.Landed != (want == "") || v.Reason != want {
			t.Errorf("%s: Untracked = %+v, want landed=%v reason %q", name, v, want == "", want)
		}
	}
	if v, err := landed.Untracked(context.Background(), git, nil); err != nil || !v.Landed {
		t.Errorf("no untracked files = %+v (err=%v), want landed", v, err)
	}
}

func TestParseCatFileBatch_ReadsBlobsAndRefusesAMalformedAnswer(t *testing.T) {
	t.Parallel()
	specs := []string{"base:a.txt", "main:b.txt", "main:c.txt"}
	blobs, err := landed.ParseCatFileBatch(specs, "0123 blob 3\nabc\nmain:b.txt missing\n0456 tree 0\n\n")
	if err != nil {
		t.Fatalf("well-formed answer: %v", err)
	}
	if b := blobs["base:a.txt"]; !b.Present || string(b.Data) != "abc" {
		t.Errorf("blob = %+v, want present abc", b)
	}
	if blobs["main:b.txt"].Present || blobs["main:c.txt"].Present {
		t.Errorf("a missing path and a tree must not count as a present blob: %+v", blobs)
	}
	for _, bad := range []string{"", "\n", "0123 blob\nabc\n", "0123 blob x\n", "0123 blob 9\nabc\n"} {
		if _, err := landed.ParseCatFileBatch(specs[:1], bad); err == nil {
			t.Errorf("answer %q parsed without an error", bad)
		}
	}
}
