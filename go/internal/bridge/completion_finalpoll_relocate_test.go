package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// completion_finalpoll_relocate_test.go — cycle-1258's closure of the cycle-1256
// audit findings against the artifact cross-poll debounce.
//
// The window that completion_debounce_test.go and
// completion_relocate_stability_test.go pin is correct on honest polls. The
// audit found it bypassed on the ONE path every cancelled or timed-out phase
// takes, and found that path carrying 0 test hits after 716 new test lines
// (D5). Everything here exercises a branch that was previously unentered:
//
//	D1 — artifactDetector.poll's isFinalPoll/ctx.Err short-circuit reached the
//	     mover with stable==0, so relocateFile's copy+remove could snapshot a
//	     still-growing fallback into the canonical path and DELETE the source.
//	     Fix: finality uses renameOnlyRelocate.
//	D3 — artifactLocate qualified candidates with os.Stat, following symlinks, so
//	     a planted link promoted arbitrary readable bytes into the deliverable.
//	     Fix: regularFileNonEmpty (os.Lstat + IsRegular).
//	D4 — relocateFile wrote a predictable "<dst>.tmp.<pid>". Fix: os.CreateTemp.
//
// Test design note on the D1 tests. The harm needs a rename that FAILS while a
// copy would succeed — otherwise the two movers are indistinguishable and the
// test proves nothing. `os.Rename` needs write permission on the SOURCE's
// directory (it unlinks the old entry); reading the file does not. So chmodding
// the fallback's directory to r-x forces exactly relocateFile's copy+remove
// branch without needing a second filesystem, and does it deterministically on
// every platform the loop runs on.

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

// --- D1: the finality short-circuit must not run copy+remove ----------------

// TestArtifactDetector_FinalPollNeverCopyRemovesAnUnwitnessedFallback is the
// core D1 assertion. stable==0 on this path by construction — the detector has
// never seen this file before — so the artifact may still be growing. Rename is
// allowed (it relinks the inode; the agent's fd follows). Copy+remove is not,
// because it publishes a half file AND destroys the original.
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

// TestArtifactDetector_FinalPollStillCanonicalizesByRename is the honest
// counterweight: "never relocate under finality" would pass the test above and
// silently break the cycle-108/141 fallback tolerance for every phase that
// finishes at the buzzer. Rename is safe and must still happen.
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

// TestArtifactDetector_SettledFallbackStillUsesTheFullMover proves the guard is
// scoped to FINALITY and is not a blanket disable of copy+remove. Same frozen
// source directory, but the window is allowed to close honestly: the artifact
// has been observed unchanged across artifactStableTicks ticks, so copying it is
// safe and the cycle-108/141 tolerance must still work on a cross-device source.
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

// TestArtifactDetector_FinalPollOnCanonicalArtifactMovesNothing pins the common
// finality case: the artifact is already where it belongs, so no mover of either
// kind is consulted and the short-circuit's original purpose — not failing a
// session whose deliverable is on disk — is preserved.
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

// TestArtifactDetector_CancelledCtxSharesTheFinalityGuard covers poll's SECOND
// finality key. isFinalPoll and ctx.Err() land on the same branch, so a detector
// whose context died mid-wait must get the same rename-only restriction — a fix
// wired to only one of the two keys leaves the other half of D1 open.
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

// --- D3: artifactLocate must not follow symlinks ----------------------------

// TestArtifactLocate_RejectsSymlinkedCandidates is the confidentiality
// assertion. artifactLocate decides which bytes become the committed
// deliverable; a symlink there turns "the agent wrote its report" into "the
// agent named a file it wants exfiltrated".
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

// TestArtifactLocate_RejectsNonRegularCandidates guards the same chokepoint
// against the other non-regular kinds: a directory named like the artifact is
// not a deliverable either, and os.Stat's size for one is nonzero on most hosts.
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

// --- D4: relocateFile must not write a predictable temp name ----------------

// TestRelocateFile_DoesNotWriteThroughAPredictableTempName plants the exact name
// the old code used, "<dst>.tmp.<pid>", as a symlink to a victim file and forces
// the copy branch. With os.WriteFile on that name the agent-controlled bytes
// followed the link and overwrote the victim; with os.CreateTemp the name is
// unpredictable and O_EXCL, so the victim is untouched.
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

// --- caller proof: the finality mover is reached from the real wait loop -----

// TestRenameOnlyRelocate_IsTheDetectorsFinalityMover is the seam-reachability
// assertion required by the caller-proof floor: renameOnlyRelocate is not a
// helper only tests call. Its production caller is artifactDetector.poll's
// finality branch (completion.go), reached from Engine.LaunchArgs →
// runTmuxREPL's post-cancel final poll; the tests above drive that branch
// through poll itself rather than calling the mover directly. This one pins the
// mover's own contract so a future refactor cannot quietly restore copy+remove
// semantics underneath it.
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
