//go:build acs

// Package cycle2645 encodes the acceptance of the salvage retention: evolve gc --only salvage turns a full bundle into a thin bundle and evicts only landed leaves over the gc.salvage_max_mb cap.
package cycle2645

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	paddingBytes = 2 << 20
	bulkBytes    = 3 << 20
	thinLimit    = 1 << 20
	capPolicy    = `{"gc":{"salvage_max_mb":1}}`
	wideCap      = `{"gc":{"salvage_max_mb":3}}`
	bundleName   = "refs.bundle"
	day          = 24 * time.Hour
)

var (
	evolveBin string
	buildErr  string
)

func TestMain(m *testing.M) {
	os.Exit(runWithBuiltBinary(m))
}

func runWithBuiltBinary(m *testing.M) int {
	out, err := os.MkdirTemp("", "acs-cycle2645-bin-")
	if err != nil {
		buildErr = err.Error()
		return m.Run()
	}
	defer os.RemoveAll(out)
	mod, err := acsassert.ModuleDir(".")
	if err != nil {
		buildErr = err.Error()
		return m.Run()
	}
	bin := filepath.Join(out, "evolve-under-test")
	build := sysexec.Command(context.Background(), "go", "build", "-o", bin, ".")
	build.Dir = filepath.Join(mod, "cmd", "evolve")
	if log, berr := build.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build: %v\n%s", berr, log)
		return m.Run()
	}
	evolveBin = bin
	return m.Run()
}

type hub struct {
	*gittest.Repo
	olderTip string
	newerTip string
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func newHub(t *testing.T, policyJSON string) *hub {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".git/info/exclude", ".evolve/\n")
	repo.Write("README.md", "hub\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write("bulk.bin", string(randomBytes(t, bulkBytes)))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "bulk history")
	older := repo.Git("rev-parse", "HEAD")
	repo.Write("second.txt", "second\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "second")
	repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	repo.Write(".evolve/policy.json", policyJSON)
	return &hub{Repo: repo, olderTip: older, newerTip: repo.Git("rev-parse", "HEAD")}
}

func (h *hub) leafDir(name string) string {
	return filepath.Join(h.Dir, ".evolve", "operator-salvage", name)
}

type leafSpec struct {
	name   string
	tip    string
	branch string
	files  map[string][]byte
	age    time.Duration
}

func (h *hub) writeLeaf(t *testing.T, s leafSpec) {
	t.Helper()
	dir := h.leafDir(s.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"HEAD": []byte(s.tip + " " + s.branch + "\n")}
	for name, body := range s.files {
		files[name] = body
	}
	written := time.Now()
	stamp := written.Add(-s.age)
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(dir, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func (h *hub) landedLeaf(t *testing.T, name, tip string, age time.Duration, extra map[string][]byte) {
	t.Helper()
	files := map[string][]byte{bundleName: randomBytes(t, paddingBytes)}
	for k, v := range extra {
		files[k] = v
	}
	h.writeLeaf(t, leafSpec{name: name, tip: tip, branch: "lane/landed", files: files, age: age})
}

func (h *hub) laneCommit(t *testing.T, branch, file string, body []byte) string {
	t.Helper()
	h.Git("checkout", "-q", "-b", branch, "main")
	h.Write(file, string(body))
	h.Git("add", "-A")
	h.Git("commit", "-q", "-m", "lane "+branch)
	tip := h.Git("rev-parse", "HEAD")
	h.Git("checkout", "-q", "main")
	return tip
}

func (h *hub) bundleBytes(t *testing.T, revs ...string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "made.bundle")
	h.Git(append([]string{"bundle", "create", out}, revs...)...)
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (h *hub) dropCommits(t *testing.T, branch, tip string) {
	t.Helper()
	h.Git("branch", "-D", branch)
	h.Git("reflog", "expire", "--expire=now", "--all")
	h.Git("gc", "-q", "--prune=now")
	if _, _, code, _ := acsassert.SubprocessOutputIn(h.Dir, "git", "cat-file", "-e", tip+"^{commit}"); code == 0 {
		t.Fatalf("fixture: commit %s is still in the hub repository after the prune", tip)
	}
}

type salvageRun struct {
	stdout, stderr string
	code           int
}

func (h *hub) salvage(t *testing.T, extra ...string) salvageRun {
	t.Helper()
	if evolveBin == "" {
		t.Fatalf("RED: the evolve binary did not build:\n%s", buildErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	args := append([]string{"gc", "--only", "salvage", "--project-root", h.Dir}, extra...)
	cmd := sysexec.Command(ctx, evolveBin, args...)
	cmd.Dir = h.Dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_TERMINAL_PROMPT=0")
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	res := salvageRun{}
	if err := cmd.Run(); err != nil {
		res.code = -1
		if cmd.ProcessState != nil {
			res.code = cmd.ProcessState.ExitCode()
		}
	}
	res.stdout, res.stderr = out.String(), errOut.String()
	return res
}

func (r salvageRun) tail() string {
	return "stdout: " + acsassert.OutputTail(r.stdout, 8) + "\nstderr: " + acsassert.OutputTail(r.stderr, 8)
}

func (h *hub) requireLeaf(t *testing.T, name, why string) {
	t.Helper()
	if !acsassert.PathExists(h.leafDir(name)) {
		t.Errorf("leaf %s was evicted: %s", name, why)
	}
}

func (h *hub) requireEvicted(t *testing.T, name, why string) {
	t.Helper()
	if acsassert.PathExists(h.leafDir(name)) {
		t.Errorf("leaf %s stays: %s", name, why)
	}
}

func untrackedArchive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("work that git does not track\n")
	if err := tw.WriteHeader(&tar.Header{Name: "notes.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestC2645_001_AFullBundleBecomesAThinBundleThatVerifies(t *testing.T) {
	h := newHub(t, `{}`)
	const leaf = "cycle-aaaa0001-101"
	tip := h.laneCommit(t, "lane/legacy", "lane.txt", []byte("lane work\n"))
	full := h.bundleBytes(t, "refs/heads/lane/legacy")
	h.writeLeaf(t, leafSpec{name: leaf, tip: tip, branch: "lane/legacy", files: map[string][]byte{bundleName: full}, age: 3 * day})
	bundle := filepath.Join(h.leafDir(leaf), bundleName)
	headsBefore := h.Git("bundle", "list-heads", bundle)
	h.dropCommits(t, "lane/legacy", tip)
	refsBefore := h.Git("for-each-ref", "--format=%(refname) %(objectname)")

	res := h.salvage(t)

	if res.code != 0 {
		t.Fatalf("evolve gc --only salvage exit %d\n%s", res.code, res.tail())
	}
	info, err := os.Stat(bundle)
	if err != nil {
		t.Fatalf("the leaf lost its bundle: %v", err)
	}
	if info.Size() >= thinLimit {
		t.Errorf("RED: the bundle is %d bytes, want under %d: a full bundle whose objects left the repository must still become thin\n%s", info.Size(), thinLimit, res.tail())
	}
	h.Git("bundle", "verify", bundle)
	if got := h.Git("bundle", "list-heads", bundle); got != headsBefore {
		t.Errorf("heads = %q, want %q", got, headsBefore)
	}
	if got := h.Git("for-each-ref", "--format=%(refname) %(objectname)"); got != refsBefore {
		t.Errorf("a temporary ref outlived the migration:\nbefore %s\nafter  %s", refsBefore, got)
	}
}

func TestC2645_002_AFullBundleThatCannotBeMigratedStaysAsItWas(t *testing.T) {
	h := newHub(t, `{}`)
	const leaf = "cycle-aaaa0002-102"
	tip := h.laneCommit(t, "lane/broken", "lane.txt", []byte("lane work\n"))
	full := h.bundleBytes(t, "refs/heads/lane/broken")
	truncated := full[:len(full)/2]
	h.writeLeaf(t, leafSpec{name: leaf, tip: tip, branch: "lane/broken", files: map[string][]byte{bundleName: truncated}, age: 3 * day})
	h.dropCommits(t, "lane/broken", tip)
	refsBefore := h.Git("for-each-ref", "--format=%(refname) %(objectname)")

	h.salvage(t)

	kept, err := os.ReadFile(filepath.Join(h.leafDir(leaf), bundleName))
	if err != nil {
		t.Fatalf("the leaf lost its bundle: %v", err)
	}
	if !bytes.Equal(kept, truncated) {
		t.Errorf("the bundle changed although no thin bundle could be verified: %d bytes, was %d", len(kept), len(truncated))
	}
	leftovers, _ := filepath.Glob(filepath.Join(h.leafDir(leaf), bundleName+".*"))
	if len(leftovers) != 0 {
		t.Errorf("a failed migration left %v", leftovers)
	}
	if got := h.Git("for-each-ref", "--format=%(refname) %(objectname)"); got != refsBefore {
		t.Errorf("a temporary ref outlived the failed migration:\nbefore %s\nafter  %s", refsBefore, got)
	}
}

func TestC2645_003_ALeafWithUntrackedFilesStaysWhileItsLandedSiblingGoes(t *testing.T) {
	h := newHub(t, capPolicy)
	const withUntracked, plain = "cycle-aaaa0003-201", "cycle-aaaa0003-202"
	h.landedLeaf(t, withUntracked, h.olderTip, 25*day, map[string][]byte{"untracked.tgz": untrackedArchive(t)})
	h.landedLeaf(t, plain, h.newerTip, 20*day, nil)

	res := h.salvage(t)

	if res.code != 0 {
		t.Fatalf("evolve gc --only salvage exit %d\n%s", res.code, res.tail())
	}
	h.requireEvicted(t, plain, "RED: the landed leaf with no patch and no untracked files is over the cap and must go")
	h.requireLeaf(t, withUntracked, "it holds untracked files that no ref holds")
	if !acsassert.PathExists(filepath.Join(h.leafDir(withUntracked), "untracked.tgz")) {
		t.Errorf("the untracked archive of %s is gone", withUntracked)
	}
}

func TestC2645_004_ALeafWithAnUncommittedPatchStaysWhileItsLandedSiblingGoes(t *testing.T) {
	h := newHub(t, capPolicy)
	const withPatch, plain = "cycle-aaaa0004-211", "cycle-aaaa0004-212"
	patch := []byte("diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1 +1 @@\n-old\n+new\n")
	h.landedLeaf(t, withPatch, h.olderTip, 25*day, map[string][]byte{"uncommitted.patch": patch})
	h.landedLeaf(t, plain, h.newerTip, 20*day, nil)

	res := h.salvage(t)

	if res.code != 0 {
		t.Fatalf("evolve gc --only salvage exit %d\n%s", res.code, res.tail())
	}
	h.requireEvicted(t, plain, "RED: the landed leaf with no patch and no untracked files is over the cap and must go")
	h.requireLeaf(t, withPatch, "it holds an uncommitted patch that no ref holds")
}

func TestC2645_005_OverTheCapOnlyTheOldestLandedLeafGoesAndAnUnlandedLeafStays(t *testing.T) {
	h := newHub(t, wideCap)
	const oldest, newer, unlanded = "cycle-aaaa0005-221", "cycle-aaaa0005-222", "cycle-aaaa0005-223"
	h.landedLeaf(t, oldest, h.olderTip, 20*day, nil)
	h.landedLeaf(t, newer, h.newerTip, 10*day, nil)
	tip := h.laneCommit(t, "lane/unlanded", "lane.txt", []byte("unlanded work\n"))
	thin := h.bundleBytes(t, "refs/heads/lane/unlanded", "^refs/remotes/origin/main")
	h.writeLeaf(t, leafSpec{name: unlanded, tip: tip, branch: "lane/unlanded", files: map[string][]byte{bundleName: thin}, age: 5 * day})

	res := h.salvage(t)

	if res.code != 0 {
		t.Fatalf("evolve gc --only salvage exit %d\n%s", res.code, res.tail())
	}
	h.requireEvicted(t, oldest, "RED: the leaf stays although the leaves are over the cap and it is the oldest landed one")
	h.requireLeaf(t, newer, "the leaves are back under the cap once the oldest landed leaf is gone")
	h.requireLeaf(t, unlanded, "its tip is not in origin/main and it is younger than the TTL")
}

func TestC2645_006_AnUnlandedLeafYoungerThanTheTTLStaysEvenWhenItAloneKeepsTheStoreOverTheCap(t *testing.T) {
	h := newHub(t, capPolicy)
	const first, second, unlanded = "cycle-aaaa0006-231", "cycle-aaaa0006-232", "cycle-aaaa0006-233"
	h.landedLeaf(t, first, h.olderTip, 20*day, nil)
	h.landedLeaf(t, second, h.newerTip, 15*day, nil)
	tip := h.laneCommit(t, "lane/big", "big.bin", randomBytes(t, paddingBytes))
	thin := h.bundleBytes(t, "refs/heads/lane/big", "^refs/remotes/origin/main")
	h.writeLeaf(t, leafSpec{name: unlanded, tip: tip, branch: "lane/big", files: map[string][]byte{bundleName: thin}, age: 5 * day})

	res := h.salvage(t)

	if res.code != 0 {
		t.Fatalf("evolve gc --only salvage exit %d\n%s", res.code, res.tail())
	}
	h.requireEvicted(t, first, "RED: a landed leaf over the cap must go")
	h.requireEvicted(t, second, "RED: a landed leaf over the cap must go")
	h.requireLeaf(t, unlanded, "no cap may evict a leaf whose tip is not in origin/main and which is younger than the TTL")
	kept, err := os.ReadFile(filepath.Join(h.leafDir(unlanded), bundleName))
	if err != nil || !bytes.Equal(kept, thin) {
		t.Errorf("the bundle of the unlanded leaf changed: %v", err)
	}
}

func TestC2645_007_ALandedLeafBoundToAContinuationStaysWhileAnUnboundOneGoes(t *testing.T) {
	h := newHub(t, capPolicy)
	const bound, free = "cycle-aaaa0007-301", "cycle-aaaa0007-302"
	h.landedLeaf(t, bound, h.olderTip, 25*day, nil)
	h.landedLeaf(t, free, h.newerTip, 20*day, nil)
	if err := continuation.WriteRegistryEntry(h.Dir, "item-1", continuation.Continuation{Branch: bound, Cycle: 301, Worktree: filepath.Join(h.Dir, ".evolve", "worktrees", bound)}); err != nil {
		t.Fatal(err)
	}

	res := h.salvage(t)

	if res.code != 0 {
		t.Fatalf("evolve gc --only salvage exit %d\n%s", res.code, res.tail())
	}
	h.requireEvicted(t, free, "RED: the landed leaf with no continuation is over the cap and must go")
	h.requireLeaf(t, bound, "a continuation binds its cycle")
}

func TestC2645_008_WithNoCapEveryLeafStays(t *testing.T) {
	h := newHub(t, `{}`)
	const first, second = "cycle-aaaa0008-311", "cycle-aaaa0008-312"
	h.landedLeaf(t, first, h.olderTip, 25*day, nil)
	h.landedLeaf(t, second, h.newerTip, 20*day, nil)

	h.salvage(t)

	h.requireLeaf(t, first, "no gc.salvage_max_mb is set")
	h.requireLeaf(t, second, "no gc.salvage_max_mb is set")
}
