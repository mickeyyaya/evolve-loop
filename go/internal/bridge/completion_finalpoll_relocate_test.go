package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// freezeSourceDir makes dir unwritable so os.Rename out of it fails while
// os.ReadFile of its contents still succeeds — the copy+remove branch's
// precondition, reproduced without a cross-device mount. Restored on cleanup so
// t.TempDir's own teardown can still remove the tree.
func freezeSourceDir(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("runs as root: mode bits do not constrain rename, so the copy+remove branch cannot be forced")
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, fi.Mode().Perm()) })
	if err := os.Rename(filepath.Join(dir, "probe-nonexistent"), filepath.Join(dir, "probe2")); err == nil {
		t.Fatalf("precondition: rename out of %s unexpectedly succeeded", dir)
	}
}

func TestArtifactDetector_FinalPollNeverCopyRemovesAnUnwitnessedFallback(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallbackDir := filepath.Join(ws, "workspace")
	fallback := filepath.Join(fallbackDir, "report.md")

	writeArtifact(t, fallback, "# report\npartial, still being written", fixedMTime)
	freezeSourceDir(t, fallbackDir)

	d := newArtifactDetectorAt(ws, canonical)
	fctx, fcancel := withFinalPoll(context.Background())
	defer fcancel()
	ready, _, _, err := d.poll(fctx)

	if ready {
		t.Fatalf("final poll completed the phase after canonicalizing an artifact it never " +
			"observed settle; on the copy+remove branch that publishes a truncated snapshot")
	}
	if err == nil {
		t.Fatalf("final poll silently declined to canonicalize: the operator gets no signal. " +
			"renameOnlyRelocate must RETURN the rename failure, not swallow it")
	}
	if _, serr := os.Stat(canonical); serr == nil {
		t.Fatalf("a copy of the unwitnessed fallback was published at the canonical path %s — "+
			"the finality path must be rename-only", canonical)
	}
	if !regularFileNonEmpty(fallback) {
		t.Fatalf("the agent's source file at %s was removed by the finality path; every byte "+
			"it wrote after the snapshot is now unrecoverable", fallback)
	}
}

func TestArtifactDetector_FinalPollStillCanonicalizesByRename(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallback := filepath.Join(ws, "workspace", "report.md")
	const body = "# report\nfinished at the buzzer"

	writeArtifact(t, fallback, body, fixedMTime)

	d := newArtifactDetectorAt(ws, canonical)
	fctx, fcancel := withFinalPoll(context.Background())
	defer fcancel()
	ready, _, note, err := d.poll(fctx)
	if err != nil {
		t.Fatalf("final poll: unexpected error: %v", err)
	}
	if !ready {
		t.Fatalf("final poll did not complete a phase whose deliverable is on disk at %s: "+
			"the rename-only guard must not launder finished sessions into ExitArtifactTimeout", fallback)
	}
	got, rerr := os.ReadFile(canonical)
	if rerr != nil {
		t.Fatalf("read canonical %s: %v", canonical, rerr)
	}
	if string(got) != body {
		t.Fatalf("canonical content = %q, want %q (rename must move the whole file)", got, body)
	}
	if _, serr := os.Stat(fallback); serr == nil {
		t.Fatalf("fallback %s survived a successful rename: the artifact now exists twice", fallback)
	}
	if note == "" {
		t.Fatalf("relocation carried no operator note; the wrote-to-the-wrong-place diagnostic is lost")
	}
}

func TestArtifactDetector_SettledFallbackStillUsesTheFullMover(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallbackDir := filepath.Join(ws, "workspace")
	fallback := filepath.Join(fallbackDir, "report.md")
	const body = "# report\ncomplete and settled"

	writeArtifact(t, fallback, body, fixedMTime)
	freezeSourceDir(t, fallbackDir)

	d := newArtifactDetectorAt(ws, canonical)
	at, _ := pollUntilReady(t, d, artifactStableTicks+2, nil)
	if at != artifactStableTicks {
		t.Fatalf("settled fallback completed at poll %d, want %d: the finality guard must not "+
			"suppress the normal window-close relocation", at, artifactStableTicks)
	}
	got, rerr := os.ReadFile(canonical)
	if rerr != nil {
		t.Fatalf("read canonical %s: %v", canonical, rerr)
	}
	if string(got) != body {
		t.Fatalf("canonical content = %q, want %q", got, body)
	}
}

func TestArtifactDetector_FinalPollOnCanonicalArtifactMovesNothing(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	writeArtifact(t, canonical, "# report\non disk", fixedMTime)

	d := newArtifactDetectorAt(ws, canonical)
	fctx, fcancel := withFinalPoll(context.Background())
	defer fcancel()
	ready, _, _, err := d.poll(fctx)
	if err != nil || !ready {
		t.Fatalf("final poll on a canonical artifact: ready=%v err=%v, want ready with no error", ready, err)
	}
}

func TestArtifactDetector_CancelledCtxSharesTheFinalityGuard(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallbackDir := filepath.Join(ws, "workspace")
	fallback := filepath.Join(fallbackDir, "report.md")

	writeArtifact(t, fallback, "# report\npartial", fixedMTime)
	freezeSourceDir(t, fallbackDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := newArtifactDetectorAt(ws, canonical)
	ready, _, _, err := d.poll(ctx)
	if ready || err == nil {
		t.Fatalf("cancelled-context poll: ready=%v err=%v — want the same rename-only refusal "+
			"the isFinalPoll key gets", ready, err)
	}
	if !regularFileNonEmpty(fallback) {
		t.Fatalf("the source at %s was destroyed on the ctx.Err() half of the finality branch", fallback)
	}
}

func TestArtifactLocate_RejectsSymlinkedCandidates(t *testing.T) {
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "credentials.json")
	writeArtifact(t, secret, `{"token":"sk-not-a-deliverable"}`, fixedMTime)

	for _, tc := range []struct{ name, rel string }{
		{"canonical", "report.md"},
		{"workspace fallback", filepath.Join("workspace", "report.md")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			canonical := filepath.Join(ws, "report.md")
			link := filepath.Join(ws, tc.rel)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.Symlink(secret, link); err != nil {
				t.Skipf("symlinks unavailable on this host: %v", err)
			}
			got, found := artifactLocate(&Config{Workspace: ws, Artifact: canonical})
			if found {
				t.Fatalf("artifactLocate promoted symlink %s (→ %s) as the deliverable at %q; "+
					"its target's bytes would be relocated into the canonical path and committed",
					link, secret, got)
			}
		})
	}
}

func TestArtifactLocate_RejectsNonRegularCandidates(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, found := artifactLocate(&Config{Workspace: ws, Artifact: canonical}); found {
		t.Fatalf("artifactLocate accepted a DIRECTORY at the canonical artifact path")
	}
}

func TestRelocateFile_DoesNotWriteThroughAPredictableTempName(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	srcDir := filepath.Join(ws, "workspace")
	src := filepath.Join(srcDir, "report.md")

	victim := filepath.Join(t.TempDir(), "victim.txt")
	const victimBody = "do not overwrite me"
	writeArtifact(t, victim, victimBody, fixedMTime)

	writeArtifact(t, src, "# report\nagent-controlled bytes", fixedMTime)
	planted := filepath.Join(ws, filepath.Base(canonical)+".tmp."+strconv.Itoa(os.Getpid()))
	if err := os.Symlink(victim, planted); err != nil {
		t.Skipf("symlinks unavailable on this host: %v", err)
	}
	freezeSourceDir(t, srcDir)

	if err := relocateFile(src, canonical); err != nil {
		t.Fatalf("relocateFile: %v", err)
	}
	got, rerr := os.ReadFile(victim)
	if rerr != nil {
		t.Fatalf("read victim %s: %v", victim, rerr)
	}
	if string(got) != victimBody {
		t.Fatalf("victim %s was overwritten through the predictable temp name %s: content = %q",
			victim, planted, got)
	}
}

// renameOnlyRelocate's production caller reaches it only indirectly, through
// artifactDetector.poll's finality branch, so this test pins the mover's own
// contract directly.
func TestRenameOnlyRelocate_IsTheDetectorsFinalityMover(t *testing.T) {
	ws := t.TempDir()
	srcDir := filepath.Join(ws, "workspace")
	src := filepath.Join(srcDir, "report.md")
	dst := filepath.Join(ws, "nested", "report.md")
	writeArtifact(t, src, "# report\n", fixedMTime)

	if err := renameOnlyRelocate(src, dst); err != nil {
		t.Fatalf("renameOnlyRelocate on a plain move: %v (it must still create dst's parent)", err)
	}
	if !regularFileNonEmpty(dst) {
		t.Fatalf("renameOnlyRelocate did not produce %s", dst)
	}

	writeArtifact(t, src, "# report\n", fixedMTime)
	freezeSourceDir(t, srcDir)
	dst2 := filepath.Join(ws, "nested", "report2.md")
	if err := renameOnlyRelocate(src, dst2); err == nil {
		t.Fatalf("renameOnlyRelocate degraded to a copy when rename failed — the one thing it exists to refuse")
	}
	if _, serr := os.Stat(dst2); serr == nil {
		t.Fatalf("renameOnlyRelocate published %s despite the rename failing", dst2)
	}
}
