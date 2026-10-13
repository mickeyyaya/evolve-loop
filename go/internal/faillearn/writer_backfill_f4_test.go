package faillearn

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type failingStagedFileF4 struct {
	stagedFile
	writeErr, chmodErr, closeErr error
}

func (f *failingStagedFileF4) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.stagedFile.Write(p)
}

func (f *failingStagedFileF4) Chmod(m fs.FileMode) error {
	if f.chmodErr != nil {
		return f.chmodErr
	}
	return f.stagedFile.Chmod(m)
}

func (f *failingStagedFileF4) Close() error {
	err := f.stagedFile.Close()
	if f.closeErr != nil {
		return f.closeErr
	}
	return err
}

func stageWithF4(t *testing.T, wrap func(stagedFile) stagedFile) {
	t.Helper()
	orig := stageCreateTemp
	t.Cleanup(func() { stageCreateTemp = orig })
	stageCreateTemp = func(dir, pattern string) (stagedFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return wrap(f), nil
	}
}

func assertNoStagingLeftF4(t *testing.T, dir string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	if len(left) != 0 {
		t.Errorf("staging files must be cleaned up: %v", left)
	}
}

func readOnlyDirF4(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if f, err := os.CreateTemp(dir, "probe"); err == nil {
		f.Close()
		t.Skip("directory permissions are not enforced for this user")
	}
	return dir
}

func TestWriteIfAbsent_FailureStagesReturnTheCauseAndPublishNothing(t *testing.T) {
	boom := errors.New("injected")
	cases := []struct {
		name string
		wrap func(stagedFile) stagedFile
	}{
		{"write fails", func(f stagedFile) stagedFile { return &failingStagedFileF4{stagedFile: f, writeErr: boom} }},
		{"chmod fails", func(f stagedFile) stagedFile { return &failingStagedFileF4{stagedFile: f, chmodErr: boom} }},
		{"close fails", func(f stagedFile) stagedFile { return &failingStagedFileF4{stagedFile: f, closeErr: boom} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stageWithF4(t, tc.wrap)
			path := filepath.Join(dir, "out.yaml")

			skipped, err := writeIfAbsent(path, []byte("data"))

			if !errors.Is(err, boom) || skipped {
				t.Fatalf("skipped=%v err=%v", skipped, err)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("nothing may be published on failure: %v", statErr)
			}
			assertNoStagingLeftF4(t, dir)
		})
	}
}

type closeHookFileF4 struct {
	stagedFile
	onClose func()
}

func (f *closeHookFileF4) Close() error {
	err := f.stagedFile.Close()
	f.onClose()
	return err
}

func TestWriteIfAbsent_LinkRaceLostToAConcurrentWriterIsASkipNotAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.yaml")
	stageWithF4(t, func(f stagedFile) stagedFile {
		return &closeHookFileF4{stagedFile: f, onClose: func() {
			if err := os.WriteFile(path, []byte("winner"), 0o644); err != nil {
				t.Error(err)
			}
		}}
	})

	skipped, err := writeIfAbsent(path, []byte("loser"))

	if err != nil || !skipped {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "winner" {
		t.Errorf("the concurrent winner must not be clobbered: %q", got)
	}
	assertNoStagingLeftF4(t, dir)
}

func TestWriteIfAbsent_OtherLinkErrorIsReturned(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sub")
	path := filepath.Join(dir, "out.yaml")
	stageWithF4(t, func(f stagedFile) stagedFile {
		return &closeHookFileF4{stagedFile: f, onClose: func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error(err)
			}
		}}
	})

	skipped, err := writeIfAbsent(path, []byte("x"))

	if !errors.Is(err, syscall.ENOENT) || skipped {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("nothing may be published: %v", statErr)
	}
}

func TestWriteIfAbsent_FilesystemErrors(t *testing.T) {
	t.Run("a path below a regular file is a stat error, not an absent file", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		skipped, err := writeIfAbsent(filepath.Join(blocker, "x.yaml"), []byte("x"))
		if err == nil || skipped {
			t.Fatalf("skipped=%v err=%v", skipped, err)
		}
	})
	t.Run("an uncreatable parent directory is returned", func(t *testing.T) {
		ro := readOnlyDirF4(t)
		skipped, err := writeIfAbsent(filepath.Join(ro, "sub", "x.yaml"), []byte("x"))
		if err == nil || skipped {
			t.Fatalf("skipped=%v err=%v", skipped, err)
		}
	})
	t.Run("an unwritable directory fails at the staging file", func(t *testing.T) {
		ro := readOnlyDirF4(t)
		skipped, err := writeIfAbsent(filepath.Join(ro, "x.yaml"), []byte("x"))
		if err == nil || skipped {
			t.Fatalf("skipped=%v err=%v", skipped, err)
		}
	})
}

func TestWriteArtifacts_LessonWriteFailureIsWrappedWithTheLessonID(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "lessons")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ev := fixtureEvent()
	id, _ := RenderLessonYAML(ev)

	err := WriteArtifacts(ev, "", blocker)

	if err == nil || !strings.Contains(err.Error(), "faillearn: write lesson "+id) {
		t.Fatalf("err = %v", err)
	}
}

func TestPreserveDiagnosis_NoWorkspaceReturnsOnlyTheCause(t *testing.T) {
	cause := errors.New("queue down")
	if got := (writeConfig{}).preserveDiagnosis(fixtureEvent(), "", cause); got != cause {
		t.Fatalf("got %v", got)
	}
}

func TestPreserveDiagnosis_WithoutItemsSaysNoneRecordedAndStillReturnsTheCause(t *testing.T) {
	runDir := t.TempDir()
	cause := errors.New("queue down")

	got := (writeConfig{}).preserveDiagnosis(fixtureEvent(), runDir, cause)

	if got != cause {
		t.Fatalf("got %v, want the cause unchanged", got)
	}
	body, err := os.ReadFile(filepath.Join(runDir, unqueuedRetroName))
	if err != nil || !strings.Contains(string(body), "- (none recorded)\n") {
		t.Fatalf("degraded artifact = %q err=%v", body, err)
	}
}

func TestPreserveDiagnosis_UnwritableWorkspaceJoinsBothErrors(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "run")
	if err := os.WriteFile(runDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("queue down")

	got := (writeConfig{}).preserveDiagnosis(fixtureEvent(), runDir, cause)

	if !errors.Is(got, cause) || !strings.Contains(got.Error(), "faillearn: write "+unqueuedRetroName) {
		t.Fatalf("got %v", got)
	}
}

func TestWriteInboxItems_UnencodableItemAndUnreadableCollisionFailLoudly(t *testing.T) {
	t.Run("a NaN weight cannot be encoded", func(t *testing.T) {
		cfg := writeConfig{inboxDir: t.TempDir(), inboxItems: []InboxItem{{ID: "n1", Title: "t", Weight: math.NaN(), PriorityClass: "correctness"}}}
		err := cfg.writeInboxItems()
		if err == nil || !strings.Contains(err.Error(), "encode inbox item n1") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("an existing item that cannot be read cannot be confirmed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "u1.json")
		if err := os.WriteFile(path, []byte("{}"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		if f, err := os.Open(path); err == nil {
			f.Close()
			t.Skip("file permissions are not enforced for this user")
		}
		cfg := writeConfig{inboxDir: dir, inboxItems: []InboxItem{{ID: "u1", Title: "t", PriorityClass: "correctness"}}}
		err := cfg.writeInboxItems()
		if err == nil || !strings.Contains(err.Error(), "already exists but cannot be read") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTruncateRunes_MultibyteStringWithinTheRuneBudgetIsReturnedWhole(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"héllo", 5, "héllo"},
		{"héllo", 4, "héll"},
		{"日本語", 3, "日本語"},
		{"日本語", 2, "日本"},
		{"", 0, ""},
		{"ab", 0, ""},
	}
	for _, tc := range cases {
		if got := truncateRunes(tc.in, tc.max); got != tc.want {
			t.Errorf("truncateRunes(%q,%d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestIsNearDuplicate_SkipsNonLessonEntriesAndUnreadableFiles(t *testing.T) {
	ev := fixtureEvent()
	_, lesson := RenderLessonYAML(ev)

	t.Run("a body with no observation tokens is never a duplicate", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "a.yaml"), lesson, 0o644); err != nil {
			t.Fatal(err)
		}
		if isNearDuplicate(dir, []byte("not: [valid"), 0.1) || isNearDuplicate(dir, nil, 0.1) {
			t.Error("a body without tokens must not match anything")
		}
	})
	t.Run("directories, other extensions and unreadable files are skipped; a later match is found", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "a-dir.yaml"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b.txt"), lesson, 0o644); err != nil {
			t.Fatal(err)
		}
		unreadable := filepath.Join(dir, "c.yaml")
		if err := os.WriteFile(unreadable, lesson, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })
		if f, err := os.Open(unreadable); err == nil {
			f.Close()
			t.Skip("file permissions are not enforced for this user")
		}
		if isNearDuplicate(dir, lesson, 0.5) {
			t.Fatal("only skipped entries exist, so no duplicate may be reported")
		}
		if err := os.WriteFile(filepath.Join(dir, "d.yaml"), lesson, 0o644); err != nil {
			t.Fatal(err)
		}
		if !isNearDuplicate(dir, lesson, 0.5) {
			t.Fatal("the identical readable lesson must be a duplicate")
		}
	})
}
