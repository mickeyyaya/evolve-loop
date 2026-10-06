package gitexec_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func commitOne(t *testing.T, r *gittest.Repo, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", name)
	r.Git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "add "+name)
	return r.Git("rev-parse", "HEAD")
}

func TestClassifyHeadPlacesEveryHead(t *testing.T) {
	r := gittest.Fixture(t)
	base := commitOne(t, r, "a.txt")
	r.Git("update-ref", "refs/remotes/origin/main", base)
	g := gitexec.Default(r.Dir)
	ctx := context.Background()

	r.Git("checkout", "-q", "-b", "scratch", base)
	orphan := commitOne(t, r, "b.txt")
	r.Git("checkout", "-q", "main")

	cases := []struct {
		name string
		sha  string
		want gitexec.HeadPlace
	}{
		{"ancestor of origin/main", base, gitexec.HeadInMain},
		{"only a local branch", orphan, gitexec.HeadOnlyInBundle},
		{"object absent from repo", "0123456789012345678901234567890123456789", gitexec.HeadOnlyInBundle},
	}
	for _, c := range cases {
		got, err := g.ClassifyHead(ctx, c.sha)
		if err != nil || got != c.want {
			t.Errorf("%s: ClassifyHead = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
	r.Git("update-ref", "refs/remotes/origin/feat", orphan)
	if got, err := g.ClassifyHead(ctx, orphan); err != nil || got != gitexec.HeadOnOtherRemoteBranch {
		t.Errorf("remote branch: ClassifyHead = %v, %v", got, err)
	}
	for place, want := range map[gitexec.HeadPlace]string{
		gitexec.HeadInMain: "IN_MAIN", gitexec.HeadOnOtherRemoteBranch: "OTHER_BRANCH", gitexec.HeadOnlyInBundle: "ONLY_IN_BACKUP",
	} {
		if place.String() != want {
			t.Errorf("%d.String() = %q, want %q", place, place.String(), want)
		}
	}
}

func TestBundleHeadsListsRefsAndRejectsCorruptBundles(t *testing.T) {
	r := gittest.Fixture(t)
	base := commitOne(t, r, "a.txt")
	dir := t.TempDir()
	r.Git("bundle", "create", filepath.Join(dir, "m.bundle"), "main")
	g := gitexec.Default(r.Dir)
	heads, err := g.BundleHeads(context.Background(), filepath.Join(dir, "m.bundle"))
	if err != nil || len(heads) != 1 || heads[0] != (gitexec.BundleHead{SHA: base, Ref: "refs/heads/main"}) {
		t.Fatalf("BundleHeads = %v, %v", heads, err)
	}
	bad := filepath.Join(dir, "bad.bundle")
	if err := os.WriteFile(bad, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := g.BundleHeads(context.Background(), bad); err == nil {
		t.Error("corrupt bundle must error")
	}
}

func TestPatchAppliedDistinguishesLandedUnlandedAndMissing(t *testing.T) {
	r := gittest.Fixture(t)
	base := commitOne(t, r, "a.txt")
	r.Git("update-ref", "refs/remotes/origin/main", base)
	g := gitexec.Default(r.Dir)
	ctx := context.Background()
	patch := filepath.Join(t.TempDir(), "p.patch")
	text := "diff --git a/z.txt b/z.txt\nnew file mode 100644\n--- /dev/null\n+++ b/z.txt\n@@ -0,0 +1 @@\n+z.txt\n"
	if err := os.WriteFile(patch, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if applied, err := g.PatchApplied(ctx, patch); err != nil || applied {
		t.Errorf("before landing = %v, %v", applied, err)
	}
	landed := commitOne(t, r, "z.txt")
	if applied, err := g.PatchApplied(ctx, patch); err != nil || applied {
		t.Errorf("committed locally but not on origin/main = %v, %v", applied, err)
	}
	r.Git("update-ref", "refs/remotes/origin/main", landed)
	if applied, err := g.PatchApplied(ctx, patch); err != nil || !applied {
		t.Errorf("after landing on origin/main = %v, %v", applied, err)
	}
	if _, err := g.PatchApplied(ctx, filepath.Join(t.TempDir(), "absent.patch")); err == nil {
		t.Error("missing patch file must be an error, not a verdict")
	}
}
