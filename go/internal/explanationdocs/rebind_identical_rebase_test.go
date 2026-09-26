package explanationdocs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// sealRequired seals an approved REQUIRED Build whose lane change is pending on the base, the shape a
// cycle worktree has after a clean rebase.
func (f fixture) sealRequired(t *testing.T) {
	t.Helper()
	f.activate(t)
	f.prepareRequired(t)
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
}

// commitPeer moves HEAD to a commit that adds a peer lane's files on top of HEAD without touching
// the worktree, as a peer's landing does to this lane's base.
func (f fixture) commitPeer(t *testing.T, files map[string]string) string {
	t.Helper()
	commit := f.commitPeerOnly(t, files)
	for path, body := range files {
		f.write(t, path, body)
	}
	return commit
}

// commitPeerOnly lands a peer commit without materializing it, for a peer that edits a path the lane
// also changed: the lane's pending bytes stay in the worktree.
func (f fixture) commitPeerOnly(t *testing.T, files map[string]string) string {
	t.Helper()
	env := append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(t.TempDir(), "index"))
	run := func(stdin string, args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", f.worktree}, args...)...)
		cmd.Env = env
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("", "read-tree", "HEAD")
	for path, body := range files {
		blob := run(body, "hash-object", "-w", "--stdin")
		run("", "update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
	}
	commit := run("", "commit-tree", run("", "write-tree"), "-p", "HEAD", "-m", "peer")
	f.git(t, "reset", "-q", "--mixed", commit)
	return commit
}

func (f fixture) hostState(t *testing.T) string {
	t.Helper()
	var state strings.Builder
	for _, path := range []string{activationPath(f.root, f.cycle), resultSnapshotPath(f.root, f.cycle), filepath.Join(f.workspace, manifestFilename)} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		state.Write(body)
	}
	return state.String()
}

func (f fixture) rebind(t *testing.T, newBase string) bool {
	t.Helper()
	rebound, err := RebindIdenticalRebase(context.Background(), f.binding(), newBase, func() error { return nil })
	if err != nil {
		t.Fatalf("RebindIdenticalRebase: %v", err)
	}
	return rebound
}

func (f fixture) requireDeclinedWithoutWrites(t *testing.T, newBase string) {
	t.Helper()
	before := f.hostState(t)
	if f.rebind(t, newBase) {
		t.Fatal("rebound a rebase whose change is not provably identical")
	}
	if f.hostState(t) != before {
		t.Fatal("a declined rebind wrote host state")
	}
}

func TestRebindIdenticalRebase_DisjointCleanRebaseRebindsWithoutTouchingBuilderFiles(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	document := filepath.Join(f.worktree, filepath.FromSlash(cycleDocumentPath(f.cycle, f.runID)))
	docBefore, _ := os.ReadFile(document)
	reportBefore, _ := os.ReadFile(filepath.Join(f.workspace, "build-report.md"))
	authored := f.base

	newBase := f.commitPeer(t, map[string]string{"peer.txt": "a peer lane's change\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("an identical change on a disjoint new base was not rebound")
	}
	rebased := f.binding()
	rebased.BaseSHA = newBase
	view, active, err := Verify(context.Background(), rebased)
	if err != nil || !active {
		t.Fatalf("Verify after rebind = (%v, %v)", active, err)
	}
	if view.BaseSHA != newBase || view.AuthoredBaseSHA != authored {
		t.Errorf("view base=%s authored=%s, want %s and %s", view.BaseSHA, view.AuthoredBaseSHA, newBase, authored)
	}
	docAfter, _ := os.ReadFile(document)
	reportAfter, _ := os.ReadFile(filepath.Join(f.workspace, "build-report.md"))
	if string(docAfter) != string(docBefore) || string(reportAfter) != string(reportBefore) {
		t.Error("the rebind touched a Builder-owned file")
	}
}

func TestRebindIdenticalRebase_PeerTouchedALanePathDeclines(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	f.requireDeclinedWithoutWrites(t, f.commitPeerOnly(t, map[string]string{"config/app.yaml": "enabled: false\nmode: peer\n"}))
}

func TestRebindIdenticalRebase_WhitespaceOnlyDriftDeclines(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	f.write(t, "config/app.yaml", "enabled: true \n")
	f.requireDeclinedWithoutWrites(t, f.commitPeer(t, map[string]string{"peer.txt": "peer\n"}))
}

func TestRebindIdenticalRebase_BinaryContentDriftDeclines(t *testing.T) {
	f := newFixture(t)
	f.write(t, "assets/logo.bin", "\x00\x01\x02")
	f.git(t, "add", "assets/logo.bin")
	f.git(t, "commit", "-q", "-m", "binary at base")
	f.base = f.git(t, "rev-parse", "HEAD")
	f.sealRequired(t)
	f.write(t, "assets/logo.bin", "\x00\x01\x03")
	f.requireDeclinedWithoutWrites(t, f.commitPeer(t, map[string]string{"peer.txt": "peer\n"}))
}

func TestRebindIdenticalRebase_ExtraShipConsumptionPathDeclines(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	f.write(t, ".evolve/inbox/consumed/item.json", "{}\n")
	f.requireDeclinedWithoutWrites(t, f.commitPeer(t, map[string]string{"peer.txt": "peer\n"}))
}

// After one rebind the bound base has moved past the authored one; a candidate that descends from the
// authored base but not the bound one would drop the peer change the rebind already absorbed.
func TestRebindIdenticalRebase_ABaseThatDoesNotDescendFromTheBoundOneDeclines(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	authored := f.base
	bound := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, bound) {
		t.Fatal("first rebind declined")
	}
	f.base = bound
	if err := os.Remove(filepath.Join(f.worktree, "peer.txt")); err != nil {
		t.Fatal(err)
	}
	f.git(t, "reset", "-q", "--mixed", authored)
	f.requireDeclinedWithoutWrites(t, f.commitPeer(t, map[string]string{"other.txt": "other\n"}))
}

func TestRebindIdenticalRebase_GitattributesInPeerDeltaDeclines(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	f.requireDeclinedWithoutWrites(t, f.commitPeer(t, map[string]string{".gitattributes": "*.yaml -diff\n"}))
}

func TestRebindIdenticalRebase_CaseFoldCollisionDeclines(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	f.requireDeclinedWithoutWrites(t, f.commitPeerOnly(t, map[string]string{"Config/App.yaml": "enabled: false\n"}))
}

// A rebound handoff was proven, not re-authored: an unchanged tree needs no refresh, and any later
// change routes back to Build instead of failing the strict re-seal.
func TestRefreshResult_AfterARebindRoutesChangesToBuild(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("rebind declined")
	}
	f.base = newBase
	requiresBuild, err := RefreshResult(context.Background(), f.binding())
	if err != nil || requiresBuild {
		t.Fatalf("unchanged rebound tree: RefreshResult = (%v, %v), want (false, nil)", requiresBuild, err)
	}
	f.write(t, "internal/app_test.go", "package app\n")
	requiresBuild, err = RefreshResult(context.Background(), f.binding())
	if err != nil || !requiresBuild {
		t.Fatalf("changed rebound tree: RefreshResult = (%v, %v), want (true, nil)", requiresBuild, err)
	}
}

// The snapshot moves with a rebind, so a caller that cannot move the cycle state with it must be refused
// before any write, or it would leave a split nothing recovers.
func TestRebindIdenticalRebase_RefusesANilPersistBeforeAnyWrite(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	before := f.hostState(t)
	if rebound, err := RebindIdenticalRebase(context.Background(), f.binding(), newBase, nil); rebound || err == nil {
		t.Fatalf("a nil persist returned (%v, %v)", rebound, err)
	}
	if f.hostState(t) != before {
		t.Fatal("a refused rebind wrote host state")
	}
}

// The workspace handoff is written before the snapshot, the commit point: a failed handoff write leaves
// the snapshot on the old base, a split RecoverRebaseSplit recognises.
func TestRebindIdenticalRebase_AFailedHandoffWriteLeavesTheSnapshotAndARecoverableSplit(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	snapshotBefore, err := os.ReadFile(resultSnapshotPath(f.root, f.cycle))
	if err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes through a 0555 directory, so the handoff write cannot be made to fail this way")
	}
	if err := os.Chmod(f.workspace, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.workspace, 0o755) })
	rebound, err := RebindIdenticalRebase(context.Background(), f.binding(), newBase, func() error { return nil })
	if rebound || !errors.Is(err, ErrRebindIncomplete) {
		t.Fatalf("a failed handoff write returned (%v, %v)", rebound, err)
	}
	if snapshotAfter, _ := os.ReadFile(resultSnapshotPath(f.root, f.cycle)); string(snapshotAfter) != string(snapshotBefore) {
		t.Fatal("the snapshot moved although the handoff write failed")
	}
	if got, recoverable, err := RecoverRebaseSplit(context.Background(), f.binding()); err != nil || !recoverable || got != newBase {
		t.Fatalf("RecoverRebaseSplit = (%s, %v, %v), want a recoverable split to %s", got, recoverable, err, newBase)
	}
}

func TestSealResult_AnUnchangedReboundHandoffIsANoOp(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("rebind declined")
	}
	f.base = newBase
	before := f.hostState(t)
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult on an unchanged rebound handoff: %v", err)
	}
	if f.hostState(t) != before {
		t.Error("SealResult rewrote an unchanged rebound handoff")
	}
}

// After a rebind, a real Build re-authors the document against the bound base; its seal replaces the
// rebound snapshot and clears the authored base.
func TestSealResult_AReauthoredHandoffReplacesARebind(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("rebind declined")
	}
	f.base = newBase
	f.write(t, cycleDocumentPath(f.cycle, f.runID), f.validDocument())
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild on the re-authored document: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	view, err := LoadSnapshot(f.binding())
	if err != nil || view.AuthoredBaseSHA != "" {
		t.Fatalf("after a re-authored seal: authored=%q err=%v, want it cleared", view.AuthoredBaseSHA, err)
	}
}

func TestPeerTouchesLane(t *testing.T) {
	lane := []string{"config/app.yaml", "docs/explain/cycle-42.md"}
	cases := map[string]bool{
		"peer.txt":              false,
		"config/app.yaml":       true,
		"Config/App.yaml":       true,
		".gitattributes":        true,
		"nested/.gitattributes": true,
		"vendor/.gitignore":     true,
		"VENDOR/.GITIGNORE":     true,
	}
	for peer, want := range cases {
		if got := peerTouchesLane([]string{peer}, lane); got != want {
			t.Errorf("peerTouchesLane(%q) = %v, want %v", peer, got, want)
		}
	}
	// APFS treats canonically equivalent names as one file and git reports raw tree bytes, so NFC
	// "café" and NFD "café" name the same worktree file: any non-ASCII name declines.
	if !peerTouchesLane([]string{"café.txt"}, []string{"café.txt"}) {
		t.Error("an NFD peer path was not matched to its NFC lane twin")
	}
	if !peerTouchesLane([]string{"peer.txt"}, []string{"café.txt"}) {
		t.Error("a non-ASCII lane path must decline: its equivalent spellings cannot be proven disjoint")
	}
	// NTFS, exFAT and SMB strip a trailing dot or space and resolve 8.3 short names and stream suffixes,
	// so these name the lane's own file there.
	for _, alias := range []string{"config/app.yaml.", "config/app.yaml ", "config/app.yaml. ", "CONFIG/APP~1.YAM", "config/app.yaml:stream", `config\app.yaml`, "config./app.yaml", "config/./app.yaml", "config/" + strings.Repeat("a", 251)} {
		if !peerTouchesLane([]string{alias}, lane) {
			t.Errorf("peerTouchesLane(%q) = false: an alias of a lane file on some filesystem must decline", alias)
		}
	}
}

func TestRebindIdenticalRebase_SecondRebindKeepsAuthoredBase(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	authored := f.base
	base1 := f.commitPeer(t, map[string]string{"peer1.txt": "one\n"})
	if !f.rebind(t, base1) {
		t.Fatal("first rebind declined")
	}
	f.base = base1
	base2 := f.commitPeer(t, map[string]string{"peer2.txt": "two\n"})
	if !f.rebind(t, base2) {
		t.Fatal("second rebind declined")
	}
	rebased := f.binding()
	rebased.BaseSHA = base2
	view, _, err := Verify(context.Background(), rebased)
	if err != nil || view.AuthoredBaseSHA != authored {
		t.Fatalf("after two rebinds: authored=%v err=%v, want %s", view, err, authored)
	}
}

func TestRebindIdenticalRebase_AFailedPersistLeavesARecoverableSplit(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	rebound, err := RebindIdenticalRebase(context.Background(), f.binding(), newBase, func() error { return errors.New("checkpoint write failed") })
	if rebound || !errors.Is(err, ErrRebindIncomplete) {
		t.Fatalf("a failed persist returned (%v, %v)", rebound, err)
	}
	got, recoverable, err := RecoverRebaseSplit(context.Background(), f.binding())
	if err != nil || !recoverable || got != newBase {
		t.Fatalf("RecoverRebaseSplit = (%s, %v, %v), want a recoverable split to %s", got, recoverable, err, newBase)
	}
}

func TestRebindIdenticalRebase_NotApplicableHandoff(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.write(t, "internal/app_test.go", "package app\n")
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n"+RenderNotApplicableDeclaration("test-only change"))
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("an identical not-applicable handoff was not rebound")
	}
	rebased := f.binding()
	rebased.BaseSHA = newBase
	if _, _, err := Verify(context.Background(), rebased); err != nil {
		t.Fatalf("Verify after rebind: %v", err)
	}
}

func TestVerify_AnAuthoredBaseThatIsNotAnAncestorIsRefused(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("rebind declined")
	}
	f.git(t, "reset", "-q", "--soft", f.base)
	unrelated := f.commitPeer(t, map[string]string{"x.txt": "x\n"})
	f.git(t, "reset", "-q", "--soft", newBase)
	forgeAuthoredBase(t, f, unrelated)
	rebased := f.binding()
	rebased.BaseSHA = newBase
	if _, _, err := Verify(context.Background(), rebased); err == nil || !strings.Contains(err.Error(), "not a disjoint ancestor") {
		t.Fatalf("Verify = %v, want the lineage refusal for an authored base that is not an ancestor", err)
	}
}

// forgeView rewrites the marker, the host snapshot and the workspace handoff consistently, so a test
// can show Verify re-derives lineage rather than trusting what the host state claims.
func forgeView(t *testing.T, f fixture, edit func(marker *activation, snapshot *resultSnapshot)) {
	t.Helper()
	marker, err := readActivation(f.root, f.cycle)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(resultSnapshotPath(f.root, f.cycle))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot resultSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	edit(&marker, &snapshot)
	if err := atomicwrite.JSON(activationPath(f.root, f.cycle), marker); err != nil {
		t.Fatal(err)
	}
	if err := atomicwrite.JSON(resultSnapshotPath(f.root, f.cycle), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(f.binding(), &snapshot.View); err != nil {
		t.Fatal(err)
	}
}

func forgeAuthoredBase(t *testing.T, f fixture, authored string) {
	t.Helper()
	forgeView(t, f, func(_ *activation, snapshot *resultSnapshot) { snapshot.View.AuthoredBaseSHA = authored })
}

func TestVerify_WithoutAnAuthoredBaseTheDocumentMustNameTheBoundBase(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	newBase := f.commitPeer(t, map[string]string{"peer.txt": "peer\n"})
	if !f.rebind(t, newBase) {
		t.Fatal("rebind declined")
	}
	forgeAuthoredBase(t, f, "")
	rebased := f.binding()
	rebased.BaseSHA = newBase
	if _, _, err := Verify(context.Background(), rebased); err == nil {
		t.Fatal("Verify accepted a document naming another base with no authored-base lineage")
	}
}

func TestVerify_AnAuthoredBaseWhosePeerDeltaOverlapsTheChangeIsRefused(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	authored := f.base
	overlapping := f.commitPeerOnly(t, map[string]string{"config/app.yaml": "enabled: false\nmode: peer\n"})
	diff, err := diffSHA256(context.Background(), f.worktree, overlapping)
	if err != nil {
		t.Fatal(err)
	}
	forgeView(t, f, func(marker *activation, snapshot *resultSnapshot) {
		marker.BaseSHA = overlapping
		snapshot.View.BaseSHA, snapshot.View.DiffSHA256, snapshot.View.AuthoredBaseSHA = overlapping, diff, authored
	})
	rebased := f.binding()
	rebased.BaseSHA = overlapping
	if _, _, err := Verify(context.Background(), rebased); err == nil {
		t.Fatal("Verify accepted a lineage whose peer delta overlaps the audited change")
	}
}

func TestSealResult_NeverWritesAnAuthoredBase(t *testing.T) {
	f := newFixture(t)
	f.sealRequired(t)
	view, err := LoadSnapshot(f.binding())
	if err != nil || view.AuthoredBaseSHA != "" {
		t.Fatalf("a freshly sealed snapshot carries authored base %q (err %v)", view.AuthoredBaseSHA, err)
	}
}
