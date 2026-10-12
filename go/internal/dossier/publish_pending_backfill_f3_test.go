package dossier

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func installHookF3(t *testing.T, repoDir, name, script string) {
	t.Helper()
	hooks := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeclitest.Install(t, filepath.Join(hooks, name), script)
}

func writeFileF3(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPublishPending_AnUnlistablePendingDirIsReturnedAndNothingIsPublished(t *testing.T) {
	root := t.TempDir()
	writeFileF3(t, PendingDir(root), "a file where the directory belongs")

	res, err := PublishPending(root, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "list pending closeouts") {
		t.Fatalf("err = %v", err)
	}
	if len(res.Published) != 0 || len(res.Skipped) != 0 || len(res.Failed) != 0 {
		t.Fatalf("res = %+v", res)
	}
}

func TestPublishPending_APendingCopyThatCannotBeClearedIsWarnedAboutAndTheCycleStaysPublished(t *testing.T) {
	r := planeWithPendingDossier(t)
	installHookF3(t, r.Dir, "post-commit", "#!/bin/sh\nrm -f .evolve/dossiers-pending/cycle-7.json\n")
	var log bytes.Buffer

	res, err := PublishPending(r.Dir, &log)

	if err != nil || len(res.Published) != 1 || res.Published[0] != 7 || len(res.Failed) != 0 {
		t.Fatalf("PublishPending = (%+v, %v)", res, err)
	}
	if !strings.Contains(log.String(), "cycle 7 is published, but its pending copy remains") {
		t.Fatalf("log = %q", log.String())
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); !strings.Contains(got, "knowledge-base/cycles/cycle-7.json") {
		t.Fatalf("HEAD = %q", got)
	}
}

func TestReadPendingFile_ReadsARegularFileAndNamesTheFailureOtherwise(t *testing.T) {
	dir := t.TempDir()
	writeFileF3(t, filepath.Join(dir, "cycle-1.json"), "payload")
	if err := os.MkdirAll(filepath.Join(dir, "cycle-2.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := readPendingFile(filepath.Join(dir, "cycle-1.json"))
	if err != nil || string(got) != "payload" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := readPendingFile(filepath.Join(dir, "absent.json")); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "read pending absent.json") {
		t.Fatalf("absent: %v", err)
	}
	if _, err := readPendingFile(filepath.Join(dir, "cycle-2.json")); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory: %v", err)
	}
}

func TestReadPendingFile_AnUnreadableRegularFileIsAReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	path := filepath.Join(t.TempDir(), "cycle-3.json")
	writeFileF3(t, path, "x")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	got, err := readPendingFile(path)

	if got != nil || !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "read pending cycle-3.json") {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestCorpusHoldsPair_EveryCombinationOfWhatTheCorpusHolds(t *testing.T) {
	jsonB, mdB := []byte("json"), []byte("md")
	cases := []struct {
		name     string
		seed     func(t *testing.T, corpus string)
		wantHeld bool
		wantErr  string
	}{
		{name: "empty corpus holds nothing", seed: func(*testing.T, string) {}},
		{name: "both identical", seed: func(t *testing.T, corpus string) {
			writeFileF3(t, filepath.Join(corpus, "cycle-1.json"), "json")
			writeFileF3(t, filepath.Join(corpus, "cycle-1.md"), "md")
		}, wantHeld: true},
		{name: "only the json is held", seed: func(t *testing.T, corpus string) {
			writeFileF3(t, filepath.Join(corpus, "cycle-1.json"), "json")
		}, wantErr: "holds half of cycle-1"},
		{name: "only the markdown is held", seed: func(t *testing.T, corpus string) {
			writeFileF3(t, filepath.Join(corpus, "cycle-1.md"), "md")
		}, wantErr: "holds half of cycle-1"},
		{name: "a different json", seed: func(t *testing.T, corpus string) {
			writeFileF3(t, filepath.Join(corpus, "cycle-1.json"), "other")
		}, wantErr: "different cycle-1.json"},
		{name: "a different markdown", seed: func(t *testing.T, corpus string) {
			writeFileF3(t, filepath.Join(corpus, "cycle-1.json"), "json")
			writeFileF3(t, filepath.Join(corpus, "cycle-1.md"), "other")
		}, wantErr: "different cycle-1.md"},
		{name: "the json path is a directory", seed: func(t *testing.T, corpus string) {
			if err := os.MkdirAll(filepath.Join(corpus, "cycle-1.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "read corpus cycle-1.json"},
		{name: "the markdown path is a directory", seed: func(t *testing.T, corpus string) {
			writeFileF3(t, filepath.Join(corpus, "cycle-1.json"), "json")
			if err := os.MkdirAll(filepath.Join(corpus, "cycle-1.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "read corpus cycle-1.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corpus := t.TempDir()
			tc.seed(t, corpus)

			held, err := corpusHoldsPair(corpus, "cycle-1", jsonB, mdB)

			if held != tc.wantHeld || (tc.wantErr == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("held=%v err=%v, want held=%v err containing %q", held, err, tc.wantHeld, tc.wantErr)
			}
		})
	}
}

func TestCommitIntoCorpus_ACorpusThatCannotBeCreatedFailsBeforeAnyWrite(t *testing.T) {
	root := t.TempDir()
	writeFileF3(t, filepath.Join(root, "knowledge-base"), "a file where the directory belongs")

	err := commitIntoCorpus(root, CyclesDir(root), "cycle-4", []byte("j"), []byte("m"))

	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "mkdir" || strings.Contains(err.Error(), "\n") {
		t.Fatalf("want exactly the MkdirAll failure, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "knowledge-base", "cycles")); statErr == nil {
		t.Fatal("nothing may be created under a blocked corpus")
	}
}

func TestRemoveIfPresent_RemovesAFileToleratesAbsenceAndReportsEverythingElse(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	writeFileF3(t, file, "x")
	full := filepath.Join(dir, "full")
	writeFileF3(t, filepath.Join(full, "child"), "x")
	blocked := filepath.Join(file, "below")

	if err := removeIfPresent(file); err != nil {
		t.Fatalf("remove present: %v", err)
	}
	if _, err := os.Lstat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the file survived: %v", err)
	}
	if err := removeIfPresent(file); err != nil {
		t.Fatalf("an absent path is already clean: %v", err)
	}
	if err := removeIfPresent(full); err == nil {
		t.Fatal("a non-empty directory cannot be removed and must be reported")
	}
	writeFileF3(t, file, "x")
	if err := removeIfPresent(blocked); err == nil {
		t.Fatal("a path below a regular file is an error, not an absence")
	}
}

func TestMarkdownWriteWouldRender_EachRefusalAndTheRenderedMarkdown(t *testing.T) {
	d := pendingCloseout()
	jsonB, err := RenderJSON(d)
	if err != nil {
		t.Fatal(err)
	}
	mdB, err := RenderMarkdown(d)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{"cycle":7,"goal":"","final_verdict":"PASS","phases":[{"name":"build","verdict":"PASS"}]}`)
	boom := errors.New("render boom")

	cases := []struct {
		name    string
		n       int
		json    []byte
		md      []byte
		render  func(*Dossier) ([]byte, error)
		want    []byte
		wantErr string
	}{
		{name: "matching pair", n: 7, json: jsonB, md: mdB, render: RenderMarkdown, want: mdB},
		{name: "json only renders the markdown", n: 7, json: jsonB, md: nil, render: RenderMarkdown, want: mdB},
		{name: "not json", n: 7, json: []byte("{torn"), md: nil, render: RenderMarkdown, wantErr: "ParseJSON"},
		{name: "another cycle", n: 8, json: jsonB, md: mdB, render: RenderMarkdown, wantErr: "pending cycle-8 holds cycle 7"},
		{name: "invalid dossier", n: 7, json: invalid, md: nil, render: RenderMarkdown, wantErr: "goal is empty"},
		{name: "markdown renderer fails", n: 7, json: jsonB, md: nil, render: func(*Dossier) ([]byte, error) { return nil, boom }, wantErr: "render boom"},
		{name: "json differs from the rendering", n: 7, json: append(bytes.Clone(jsonB), ' '), md: mdB, render: RenderMarkdown, wantErr: "not what Write renders"},
		{name: "markdown differs from the rendering", n: 7, json: jsonB, md: []byte("edited"), render: RenderMarkdown, wantErr: "not what Write renders"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := markdownWriteWouldRender(tc.n, tc.json, tc.md, tc.render)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || got != nil {
					t.Fatalf("got=%q err=%v, want error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || !bytes.Equal(got, tc.want) {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}

type scriptedGitF3 struct {
	failDiff   bool
	commitErr  error
	calls      []string
	commitExit int
}

func (s *scriptedGitF3) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
	s.calls = append(s.calls, args[0])
	switch args[0] {
	case "diff":
		if s.failDiff {
			return -1, errors.New("git binary vanished")
		}
		return 1, nil
	case "commit":
		return s.commitExit, s.commitErr
	}
	return 0, nil
}

func TestCommitPairGit_AFailingDiffProbeOrCommitSpawnIsReportedWithoutRetry(t *testing.T) {
	t.Run("diff probe cannot run", func(t *testing.T) {
		s := &scriptedGitF3{failDiff: true}

		err := commitPairGit(gitexec.Git{Dir: t.TempDir(), Exec: s.run}, "cycle-9")

		if err == nil || !strings.Contains(err.Error(), "git diff cycle-9") || strings.Contains(strings.Join(s.calls, ","), "commit") {
			t.Fatalf("err=%v calls=%v", err, s.calls)
		}
	})
	t.Run("commit cannot be spawned", func(t *testing.T) {
		s := &scriptedGitF3{commitErr: errors.New("spawn failed")}

		err := commitPairGit(gitexec.Git{Dir: t.TempDir(), Exec: s.run}, "cycle-9")

		if err == nil || !strings.Contains(err.Error(), "git commit cycle-9: spawn failed") {
			t.Fatalf("err=%v", err)
		}
		commits, resets := 0, 0
		for _, c := range s.calls {
			switch c {
			case "commit":
				commits++
			case "reset":
				resets++
			}
		}
		if commits != 1 || resets != 1 {
			t.Fatalf("a spawn failure is not transient and must unstage once: calls=%v", s.calls)
		}
	})
}

func TestCommitFailure_RendersTheSpawnErrorOrTheExitAndTrimmedStderr(t *testing.T) {
	spawn := commitFailure("cycle-1", -1, "ignored", errors.New("spawn failed"))
	if spawn.Error() != "dossier: git commit cycle-1: spawn failed" {
		t.Fatalf("spawn = %v", spawn)
	}
	exit := commitFailure("cycle-1", 128, "  fatal: nope \n", nil)
	if exit.Error() != "dossier: git commit cycle-1: exit 128: fatal: nope" {
		t.Fatalf("exit = %v", exit)
	}
}
