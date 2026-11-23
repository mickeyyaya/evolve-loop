package gc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const archiveBranchName = "cycle-cd3ae73e-1677"

type archived struct {
	repo *gittest.Repo
	o    WorktreeOptions
	tip  string
	dir  string
	wt   string
}

func newArchived(t *testing.T) *archived {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("f.txt", "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := filepath.Join(repo.Dir, ".evolve", "worktrees")
	wt := filepath.Join(base, archiveBranchName)
	repo.Git("worktree", "add", "-q", wt, "-b", archiveBranchName)
	repo.Write(filepath.Join(".evolve", "worktrees", archiveBranchName, "f.txt"), "base\nlane\n")
	repo.Git("-C", wt, "commit", "-q", "-am", "lane")
	repo.Write(filepath.Join(".evolve", "worktrees", archiveBranchName, "f.txt"), "base\nlane\nmore\n")
	repo.Write(filepath.Join(".evolve", "worktrees", archiveBranchName, "sub", "new.txt"), "untracked\n")
	now := time.Now()
	closeOutCycle(t, repo.Dir, 1677, now.Add(-30*time.Hour))
	o := WorktreeOptions{ProjectRoot: repo.Dir, WorktreeBase: base, EvolveDir: filepath.Join(repo.Dir, ".evolve"),
		Policy: WorktreesPolicy{SalvageAfterHours: 24}, Now: func() time.Time { return now }, Exec: sysexec.DefaultRunner}
	a := &archived{repo: repo, o: o, tip: repo.Git("rev-parse", archiveBranchName), wt: wt}
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyWorktrees(o, m); err != nil {
		t.Fatal(err)
	}
	a.dir = filepath.Join(OperatorSalvageDir(o.EvolveDir), archiveBranchName)
	return a
}

func (a *archived) rewrite(t *testing.T, edit func(*leafManifest)) {
	t.Helper()
	var m leafManifest
	body, err := os.ReadFile(filepath.Join(a.dir, manifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	edit(&m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	a.writeManifest(t, out)
}

func (a *archived) writeManifest(t *testing.T, body []byte) {
	t.Helper()
	writeFile(t, filepath.Join(a.dir, manifestFile), string(body))
	writeFile(t, filepath.Join(a.dir, manifestHashFile), sha256Hex(body)+"\n")
}

func (a *archived) restore(t *testing.T) (RestoreResult, error) {
	t.Helper()
	return RestoreArchive(a.o, 1677, filepath.Join(t.TempDir(), "restored"))
}

func (a *archived) requireRefused(t *testing.T) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "restored")
	_, err := RestoreArchive(a.o, 1677, dest)
	if err == nil {
		t.Fatal("RestoreArchive succeeded, want a refusal")
	}
	if _, serr := os.Stat(dest); serr == nil {
		t.Errorf("a refused restore created %s", dest)
	}
}

func faultOn(prefixes ...string) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		joined := strings.Join(args, " ")
		for _, p := range prefixes {
			if strings.HasPrefix(joined, p) {
				return 1, errors.New("injected " + p)
			}
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func tgz(t *testing.T, entries ...tar.Header) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		h := h
		h.Size = int64(len("body"))
		if h.Typeflag != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("body")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "u.tgz")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRestoreArchive_RoundTripsTheBranchTheUncommittedPatchAndTheUntrackedFiles(t *testing.T) {
	a := newArchived(t)
	dest := filepath.Join(t.TempDir(), "restored")

	res, err := RestoreArchive(a.o, 1677, dest)

	if err != nil {
		t.Fatal(err)
	}
	if res != (RestoreResult{Leaf: archiveBranchName, Branch: archiveBranchName, Head: a.tip, Worktree: dest}) {
		t.Errorf("result %+v", res)
	}
	if got := readFile(t, filepath.Join(dest, "f.txt")); got != "base\nlane\nmore\n" {
		t.Errorf("f.txt = %q", got)
	}
	if got := readFile(t, filepath.Join(dest, "sub", "new.txt")); got != "untracked\n" {
		t.Errorf("sub/new.txt = %q", got)
	}
}

func TestRestoreArchive_DefaultsTheWorktreeToTheLeafUnderTheWorktreeBase(t *testing.T) {
	a := newArchived(t)

	res, err := RestoreArchive(a.o, 1677, "")

	if err != nil || res.Worktree != a.wt {
		t.Fatalf("RestoreArchive = %+v, %v; want the worktree %s", res, err, a.wt)
	}
}

func TestRestoreArchive_RestoresFromTheObjectStoreWhenOnlyTheArchiveRefIsGone(t *testing.T) {
	a := newArchived(t)
	a.repo.Git("update-ref", "-d", archiveRef(archiveBranchName))

	if _, err := a.restore(t); err != nil {
		t.Fatal(err)
	}
	if got := a.repo.Git("rev-parse", archiveRef(archiveBranchName)); got != a.tip {
		t.Errorf("archive ref = %s, want it recreated at %s", got, a.tip)
	}
}

func TestRestoreArchive_RestoresFromTheBundleWhenTheObjectsAreGone(t *testing.T) {
	a := newArchived(t)
	a.repo.Git("update-ref", "-d", archiveRef(archiveBranchName))
	a.repo.Git("reflog", "expire", "--expire=now", "--all")
	a.repo.Git("gc", "-q", "--prune=now")

	if _, err := a.restore(t); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreArchive_RestoresALeafWithNeitherPatchNorUntrackedFiles(t *testing.T) {
	a := newArchived(t)
	a.rewrite(t, func(m *leafManifest) {
		delete(m.Files, salvagePatchFile)
		delete(m.Files, salvageUntrackedFile)
	})

	res, err := a.restore(t)

	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(res.Worktree, "f.txt")); got != "base\nlane\n" {
		t.Errorf("f.txt = %q, want the committed content only", got)
	}
}

func TestRestoreArchive_RefusesAndCreatesNothing(t *testing.T) {
	t.Run("a branch that exists", func(t *testing.T) {
		a := newArchived(t)
		a.repo.Git("branch", archiveBranchName)
		a.requireRefused(t)
	})
	t.Run("a worktree path that exists", func(t *testing.T) {
		a := newArchived(t)
		dest := t.TempDir()
		if _, err := RestoreArchive(a.o, 1677, dest); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a cycle with no leaf", func(t *testing.T) {
		a := newArchived(t)
		if _, err := RestoreArchive(a.o, 9, filepath.Join(t.TempDir(), "r")); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a cycle with two leaves", func(t *testing.T) {
		a := newArchived(t)
		if err := os.MkdirAll(filepath.Join(OperatorSalvageDir(a.o.EvolveDir), "cycle-other-1677"), 0o755); err != nil {
			t.Fatal(err)
		}
		a.requireRefused(t)
	})
	t.Run("a salvage root that is a file", func(t *testing.T) {
		a := newArchived(t)
		o := a.o
		o.EvolveDir = filepath.Join(t.TempDir(), "ev")
		writeFile(t, OperatorSalvageDir(o.EvolveDir), "x")
		if _, err := RestoreArchive(o, 1677, filepath.Join(t.TempDir(), "r")); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("no Exec", func(t *testing.T) {
		a := newArchived(t)
		o := a.o
		o.Exec = nil
		if _, err := RestoreArchive(o, 1677, "x"); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a ship lock that cannot be taken", func(t *testing.T) {
		a := newArchived(t)
		o := a.o
		o.ProjectRoot = t.TempDir()
		writeFile(t, filepath.Join(o.ProjectRoot, ".evolve"), "x")
		if _, err := RestoreArchive(o, 1677, "x"); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRestoreArchive_RefusesALeafThatFailsItsVerification(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, a *archived)
	}{
		{"no manifest", func(t *testing.T, a *archived) { os.Remove(filepath.Join(a.dir, manifestFile)) }},
		{"no manifest hash", func(t *testing.T, a *archived) { os.Remove(filepath.Join(a.dir, manifestHashFile)) }},
		{"manifest hash differs", func(t *testing.T, a *archived) {
			writeFile(t, filepath.Join(a.dir, manifestHashFile), "00\n")
		}},
		{"manifest is not json", func(t *testing.T, a *archived) { a.writeManifest(t, []byte("{")) }},
		{"manifest head is not a sha", func(t *testing.T, a *archived) { a.rewrite(t, func(m *leafManifest) { m.Head = "zz" }) }},
		{"manifest ref is foreign", func(t *testing.T, a *archived) { a.rewrite(t, func(m *leafManifest) { m.Ref = "refs/heads/x" }) }},
		{"manifest lists a path", func(t *testing.T, a *archived) {
			a.rewrite(t, func(m *leafManifest) { m.Files["../x"] = "00" })
		}},
		{"manifest lists a missing file", func(t *testing.T, a *archived) {
			a.rewrite(t, func(m *leafManifest) { m.Files["gone"] = "00" })
		}},
		{"a file differs", func(t *testing.T, a *archived) {
			writeFile(t, filepath.Join(a.dir, salvagePatchFile), "tampered")
		}},
		{"ref points elsewhere", func(t *testing.T, a *archived) {
			a.repo.Git("update-ref", archiveRef(archiveBranchName), "HEAD")
		}},
		{"ref and commit gone and no bundle", func(t *testing.T, a *archived) {
			a.repo.Git("update-ref", "-d", archiveRef(archiveBranchName))
			a.rewrite(t, func(m *leafManifest) { m.Head = strings.Repeat("a", 40); delete(m.Files, salvageBundleFile) })
		}},
		{"the bundle gives another tip", func(t *testing.T, a *archived) {
			a.repo.Git("update-ref", "-d", archiveRef(archiveBranchName))
			a.rewrite(t, func(m *leafManifest) { m.Head = strings.Repeat("a", 40) })
		}},
		{"the bundle does not fetch", func(t *testing.T, a *archived) {
			a.repo.Git("update-ref", "-d", archiveRef(archiveBranchName))
			a.repo.Git("reflog", "expire", "--expire=now", "--all")
			a.repo.Git("gc", "-q", "--prune=now")
			writeFile(t, filepath.Join(a.dir, salvageBundleFile), "junk")
			a.rewrite(t, func(m *leafManifest) { m.Files[salvageBundleFile] = sha256Hex([]byte("junk")) })
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newArchived(t)
			c.edit(t, a)
			a.requireRefused(t)
		})
	}
}

func TestRestoreArchive_RollsBackWhenAStepAfterTheWorktreeFails(t *testing.T) {
	t.Run("a patch that does not apply", func(t *testing.T) {
		a := newArchived(t)
		writeFile(t, filepath.Join(a.dir, salvagePatchFile), "garbage\n")
		a.rewrite(t, func(m *leafManifest) { m.Files[salvagePatchFile] = sha256Hex([]byte("garbage\n")) })
		dest := filepath.Join(t.TempDir(), "restored")
		if _, err := RestoreArchive(a.o, 1677, dest); err == nil {
			t.Fatal("want an error")
		}
		if _, err := os.Stat(dest); err == nil {
			t.Errorf("the failed restore left %s", dest)
		}
		if out := a.repo.Git("branch", "--list", archiveBranchName); out != "" {
			t.Errorf("the failed restore left branch %q", out)
		}
	})
	t.Run("an archive that does not extract", func(t *testing.T) {
		a := newArchived(t)
		writeFile(t, filepath.Join(a.dir, salvageUntrackedFile), "junk")
		a.rewrite(t, func(m *leafManifest) { m.Files[salvageUntrackedFile] = sha256Hex([]byte("junk")) })
		a.requireRefused(t)
	})
	t.Run("a rollback that also fails reports both", func(t *testing.T) {
		a := newArchived(t)
		writeFile(t, filepath.Join(a.dir, salvagePatchFile), "garbage\n")
		a.rewrite(t, func(m *leafManifest) { m.Files[salvagePatchFile] = sha256Hex([]byte("garbage\n")) })
		a.o.Exec = faultOn("worktree remove", "update-ref -d")
		_, err := a.restore(t)
		if err == nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a worktree that cannot be added", func(t *testing.T) {
		a := newArchived(t)
		blocker := filepath.Join(t.TempDir(), "file")
		writeFile(t, blocker, "x")
		if _, err := RestoreArchive(a.o, 1677, filepath.Join(blocker, "sub")); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestApplyWorktrees_ArchivesOnlyWithAScannedTipAndKeepsAMovedBranch(t *testing.T) {
	t.Run("no scanned tip keeps the branch", func(t *testing.T) {
		a := newArchived(t)
		if err := a.o.archiveBranch("leaf", WorktreeItem{Branch: "b"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a ref that cannot be written", func(t *testing.T) {
		a := newArchived(t)
		err := a.o.archiveBranch("leaf", WorktreeItem{Branch: "b", Head: "zz"})
		if !errors.Is(err, ErrBranchKept) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a branch that moved", func(t *testing.T) {
		a := newArchived(t)
		a.repo.Git("branch", "moved")
		err := a.o.archiveBranch("leaf", WorktreeItem{Branch: "moved", Head: a.tip})
		if !errors.Is(err, ErrBranchKept) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestSalvageRemoveWorktrees_ReportsAnArchiveFailureAndARefusedMovedBranch(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("f.txt", "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := filepath.Join(repo.Dir, ".evolve", "worktrees")
	wt := filepath.Join(base, archiveBranchName)
	repo.Git("worktree", "add", "-q", wt, "-b", archiveBranchName)
	tip := repo.Git("rev-parse", archiveBranchName)
	o := WorktreeOptions{ProjectRoot: repo.Dir, WorktreeBase: base, EvolveDir: filepath.Join(repo.Dir, ".evolve"), PidAlive: func(int) bool { return false }, Exec: faultOn("update-ref refs/archive")}

	_, errs := o.salvageRemoveWorktrees([]WorktreeItem{{Path: wt, Branch: archiveBranchName, Head: tip, Action: WorktreeActionSalvageRemove}}, nil)

	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}
	_, errs = o.salvageRemoveWorktrees([]WorktreeItem{{Path: wt, Branch: archiveBranchName, Head: strings.Repeat("b", 40), Action: WorktreeActionSalvageRemove}}, nil)
	if len(errs) != 1 || !errors.Is(errs[0], ErrTreeKept) {
		t.Fatalf("errs = %v", errs)
	}
}

func TestWriteLeafManifest_ReportsEachFailure(t *testing.T) {
	t.Run("no HEAD", func(t *testing.T) {
		if err := writeLeafManifest(t.TempDir()); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a leaf file that is a directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, salvageHeadFile), strings.Repeat("a", 40)+" b\n")
		if err := os.Mkdir(filepath.Join(dir, salvagePatchFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeLeafManifest(dir); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a manifest path that is a directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, salvageHeadFile), strings.Repeat("a", 40)+" b\n")
		if err := os.Mkdir(filepath.Join(dir, manifestFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeLeafManifest(dir); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a hash path that is a directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, salvageHeadFile), strings.Repeat("a", 40)+" b\n")
		if err := os.Mkdir(filepath.Join(dir, manifestHashFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeLeafManifest(dir); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExtractUntracked_RestoresEntriesAndRefusesUnsafeOnes(t *testing.T) {
	t.Run("files, directories and links", func(t *testing.T) {
		root := t.TempDir()
		arc := tgz(t,
			tar.Header{Name: "d/", Typeflag: tar.TypeDir, Mode: 0o755},
			tar.Header{Name: "d/f.txt", Typeflag: tar.TypeReg, Mode: 0o644},
			tar.Header{Name: "e/l", Typeflag: tar.TypeSymlink, Linkname: "../d/f.txt"},
		)
		if err := extractUntracked(arc, root); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, filepath.Join(root, "d", "f.txt")); got != "body" {
			t.Errorf("d/f.txt = %q", got)
		}
		if target, err := os.Readlink(filepath.Join(root, "e", "l")); err != nil || target != "../d/f.txt" {
			t.Errorf("link = %q, %v", target, err)
		}
	})
	refused := map[string]tar.Header{
		"a path that leaves the worktree":      {Name: "../x", Typeflag: tar.TypeReg, Mode: 0o644},
		"an entry type that is not restorable": {Name: "h", Typeflag: tar.TypeFifo, Mode: 0o644},
	}
	for name, h := range refused {
		t.Run(name, func(t *testing.T) {
			if err := extractUntracked(tgz(t, h), t.TempDir()); err == nil {
				t.Fatal("want an error")
			}
		})
	}
	t.Run("a parent that is a symlink", func(t *testing.T) {
		root := t.TempDir()
		arc := tgz(t,
			tar.Header{Name: "l", Typeflag: tar.TypeSymlink, Linkname: os.TempDir()},
			tar.Header{Name: "l/x", Typeflag: tar.TypeReg, Mode: 0o644},
		)
		if err := extractUntracked(arc, root); err == nil {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a file that exists", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "f"), "old")
		if err := extractUntracked(tgz(t, tar.Header{Name: "f", Typeflag: tar.TypeReg, Mode: 0o644}), root); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("a parent that cannot be made", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "p"), "file")
		for _, h := range []tar.Header{
			{Name: "p/x", Typeflag: tar.TypeReg, Mode: 0o644},
			{Name: "p/l", Typeflag: tar.TypeSymlink, Linkname: "t"},
			{Name: "p/d/", Typeflag: tar.TypeDir, Mode: 0o755},
		} {
			if err := extractUntracked(tgz(t, h), root); err == nil {
				t.Errorf("%s: want an error", h.Name)
			}
		}
	})
	t.Run("an archive that is missing, not gzip or truncated", func(t *testing.T) {
		if err := extractUntracked(filepath.Join(t.TempDir(), "none"), t.TempDir()); err == nil {
			t.Error("missing: want an error")
		}
		junk := filepath.Join(t.TempDir(), "junk")
		writeFile(t, junk, "junk")
		if err := extractUntracked(junk, t.TempDir()); err == nil {
			t.Error("junk: want an error")
		}
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write([]byte("not a tar header, but long enough to be read as one: " + strings.Repeat("x", 600)))
		gz.Close()
		bad := filepath.Join(t.TempDir(), "bad.tgz")
		if err := os.WriteFile(bad, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := extractUntracked(bad, t.TempDir()); err == nil {
			t.Error("bad tar: want an error")
		}
	})
}

func TestRequirePlainParents_ReportsAnUnrelatableAndAnUnreadableParent(t *testing.T) {
	if err := requirePlainParents("relative", "/absolute"); err == nil {
		t.Error("a relative root against an absolute dir: want an error")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a"), "file")
	if err := requirePlainParents(root, filepath.Join(root, "a", "b")); err == nil {
		t.Error("a parent below a regular file: want an error")
	}
	if err := requirePlainParents(root, root); err != nil {
		t.Errorf("the root itself: %v", err)
	}
}
