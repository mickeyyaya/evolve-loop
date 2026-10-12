package gitexec_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type scriptedGitF4 struct {
	calls [][]string
	steps map[string]func(args []string, stdout, stderr io.Writer) (int, error)
}

func (s *scriptedGitF4) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	s.calls = append(s.calls, args)
	if step, ok := s.steps[args[0]]; ok {
		return step(args, stdout, stderr)
	}
	return 0, nil
}

func (s *scriptedGitF4) git() gitexec.Git {
	return gitexec.Git{Dir: ".", Exec: sysexec.RunFunc(s.run)}
}

func exitF4(code int, stderr string) func([]string, io.Writer, io.Writer) (int, error) {
	return func(_ []string, _, e io.Writer) (int, error) {
		_, _ = io.WriteString(e, stderr)
		return code, nil
	}
}

func failF4(err error) func([]string, io.Writer, io.Writer) (int, error) {
	return func([]string, io.Writer, io.Writer) (int, error) { return -1, err }
}

func TestClassifyHead_EachFailingGitStepIsReportedWithItsCommand(t *testing.T) {
	boom := errors.New("spawn failed")
	cases := []struct {
		name    string
		steps   map[string]func([]string, io.Writer, io.Writer) (int, error)
		wantErr string
	}{
		{"cat-file cannot run", map[string]func([]string, io.Writer, io.Writer) (int, error){"cat-file": failF4(boom)}, "gitexec: cat-file abc"},
		{"merge-base cannot run", map[string]func([]string, io.Writer, io.Writer) (int, error){"merge-base": failF4(boom)}, "gitexec: merge-base abc"},
		{"merge-base exits with a usage error", map[string]func([]string, io.Writer, io.Writer) (int, error){"merge-base": exitF4(128, " fatal: Not a valid object name origin/main \n")}, "exit=128: fatal: Not a valid object name origin/main"},
		{"branch listing fails", map[string]func([]string, io.Writer, io.Writer) (int, error){"merge-base": exitF4(1, ""), "branch": failF4(boom)}, "gitexec: branch -r --contains abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &scriptedGitF4{steps: tc.steps}
			got, err := s.git().ClassifyHead(context.Background(), "abc")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if got != gitexec.HeadOnlyInBundle {
				t.Errorf("an unclassifiable head must stay ONLY_IN_BACKUP (never claim it is safe in main); got %v", got)
			}
		})
	}
}

func TestPatchApplied_EachFailingGitStepIsReportedAndExitCodesMapToTheAnswer(t *testing.T) {
	boom := errors.New("spawn failed")
	errCases := []struct {
		name    string
		steps   map[string]func([]string, io.Writer, io.Writer) (int, error)
		wantErr string
	}{
		{"read-tree fails", map[string]func([]string, io.Writer, io.Writer) (int, error){"read-tree": exitF4(128, "bad ref")}, "gitexec: read-tree origin/main"},
		{"apply cannot run", map[string]func([]string, io.Writer, io.Writer) (int, error){"apply": failF4(boom)}, "gitexec: apply --check p.patch"},
		{"apply exits with an unexpected code", map[string]func([]string, io.Writer, io.Writer) (int, error){"apply": exitF4(2, " corrupt patch ")}, "exit=2: corrupt patch"},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &scriptedGitF4{steps: tc.steps}
			applied, err := s.git().PatchApplied(context.Background(), "p.patch")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || applied {
				t.Fatalf("applied=%v err=%v, want %q", applied, err, tc.wantErr)
			}
		})
	}
	for code, want := range map[int]bool{0: true, 1: false} {
		s := &scriptedGitF4{steps: map[string]func([]string, io.Writer, io.Writer) (int, error){"apply": exitF4(code, "")}}
		applied, err := s.git().PatchApplied(context.Background(), "p.patch")
		if err != nil || applied != want {
			t.Errorf("apply exit %d: applied=%v err=%v, want %v", code, applied, err, want)
		}
	}
}

func TestPatchApplied_UnusableTempDirIsReported(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	s := &scriptedGitF4{}
	applied, err := s.git().PatchApplied(context.Background(), "p.patch")
	if err == nil || !strings.Contains(err.Error(), "gitexec: scratch index") || applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	if len(s.calls) != 0 {
		t.Errorf("no git command may run before the scratch index exists: %v", s.calls)
	}
}

func TestDirtyPaths_RunnerFailureIsWrapped(t *testing.T) {
	s := &scriptedGitF4{steps: map[string]func([]string, io.Writer, io.Writer) (int, error){"status": failF4(errors.New("no git"))}}
	paths, err := s.git().DirtyPaths(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gitexec: git status: no git") || paths != nil {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
}

func TestTreeOfIndexCopy_ScratchAndIndexCopyFailuresStopBeforeAnyGitWrite(t *testing.T) {
	t.Run("temp dir unusable", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
		s := &scriptedGitF4{steps: map[string]func([]string, io.Writer, io.Writer) (int, error){"rev-parse": func(_ []string, o, _ io.Writer) (int, error) {
			_, _ = io.WriteString(o, "/nonexistent/index\n")
			return 0, nil
		}}}
		tree, err := s.git().TreeOfIndexCopy(context.Background())
		if err == nil || !strings.Contains(err.Error(), "gitexec: scratch index") || tree != "" {
			t.Fatalf("tree=%q err=%v", tree, err)
		}
	})
	t.Run("index file missing", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "index")
		s := &scriptedGitF4{steps: map[string]func([]string, io.Writer, io.Writer) (int, error){"rev-parse": func(_ []string, o, _ io.Writer) (int, error) {
			_, _ = io.WriteString(o, missing+"\n")
			return 0, nil
		}}}
		tree, err := s.git().TreeOfIndexCopy(context.Background(), []string{"add", "x"})
		if err == nil || !strings.Contains(err.Error(), "gitexec: copy index "+missing) || tree != "" {
			t.Fatalf("tree=%q err=%v", tree, err)
		}
		for _, c := range s.calls {
			if c[0] == "add" || c[0] == "write-tree" {
				t.Errorf("no write may follow a failed index copy: %v", c)
			}
		}
	})
	t.Run("index file present is copied and used", func(t *testing.T) {
		idx := filepath.Join(t.TempDir(), "index")
		if err := os.WriteFile(idx, []byte("DIRC"), 0o644); err != nil {
			t.Fatal(err)
		}
		s := &scriptedGitF4{steps: map[string]func([]string, io.Writer, io.Writer) (int, error){
			"rev-parse":  func(_ []string, o, _ io.Writer) (int, error) { _, _ = io.WriteString(o, idx+"\n"); return 0, nil },
			"write-tree": func(_ []string, o, _ io.Writer) (int, error) { _, _ = io.WriteString(o, "treesha\n"); return 0, nil },
		}}
		tree, err := s.git().TreeOfIndexCopy(context.Background(), []string{"add", "x"})
		if err != nil || tree != "treesha" {
			t.Fatalf("tree=%q err=%v", tree, err)
		}
	})
}
