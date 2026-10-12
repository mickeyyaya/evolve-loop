package landed

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type stepF3 struct {
	stdout string
	stderr string
	code   int
	err    error
}

type scriptedExecF3 struct {
	byCommand map[string]stepF3
	calls     []string
}

func (s *scriptedExecF3) git(dir string) gitexec.Git {
	return gitexec.Git{Dir: dir, Exec: func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		s.calls = append(s.calls, strings.Join(args, " "))
		for _, a := range args {
			if step, ok := s.byCommand[a]; ok {
				_, _ = io.WriteString(stdout, step.stdout)
				_, _ = io.WriteString(stderr, step.stderr)
				return step.code, step.err
			}
		}
		return 0, nil
	}}
}

func rawRecordF3(oldMode, newMode, status, path string) string {
	return ":" + oldMode + " " + newMode + " 1111111 2222222 " + status + "\x00" + path + "\x00"
}

func TestChanges_AFailingGitStepIsAnErrorNeverALandedVerdict(t *testing.T) {
	t.Parallel()
	oneChange := rawRecordF3("100644", "100644", "M", "a.txt")
	modeChange := rawRecordF3("100644", "100755", "M", "a.txt")
	cases := []struct {
		name    string
		steps   map[string]stepF3
		wantErr string
	}{
		{name: "the raw diff fails", steps: map[string]stepF3{"diff": {code: 128, stderr: "fatal: bad revision\n"}}, wantErr: "git diff --raw: rc=128"},
		{name: "the blob read fails", steps: map[string]stepF3{"diff": {stdout: oneChange}, "cat-file": {code: 128, stderr: "fatal: no repo\n"}}, wantErr: "git cat-file --batch: rc=128"},
		{name: "the origin mode lookup fails", steps: map[string]stepF3{
			"diff": {stdout: modeChange}, "cat-file": {stdout: "1111 blob 1\nx\n2222 blob 1\nx\n"}, "ls-tree": {code: 128, stderr: "fatal: not a tree\n"},
		}, wantErr: "git --literal-pathspecs ls-tree: rc=128"},
		{name: "the runner cannot start git", steps: map[string]stepF3{"diff": {err: errors.New("exec: git not found")}}, wantErr: "git not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &scriptedExecF3{byCommand: tc.steps}

			v, err := Changes(context.Background(), exec.git(t.TempDir()), "BASE")

			if v.Landed || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("verdict=%+v err=%v, want an error containing %q", v, err, tc.wantErr)
			}
		})
	}
}

func TestChanges_AWorktreePathThatCannotBeInspectedIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := &scriptedExecF3{byCommand: map[string]stepF3{
		"diff":     {stdout: rawRecordF3("100644", "100644", "M", "plain/child")},
		"cat-file": {stdout: "1111 blob 1\nx\n2222 blob 1\nx\n"},
	}}

	v, err := Changes(context.Background(), exec.git(dir), "BASE")

	if v.Landed || err == nil || errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("verdict=%+v err=%v", v, err)
	}
}

func TestChanges_NoChangedPathIsLandedWithoutReadingBlobs(t *testing.T) {
	t.Parallel()
	exec := &scriptedExecF3{byCommand: map[string]stepF3{"diff": {stdout: ""}}}

	v, err := Changes(context.Background(), exec.git(t.TempDir()), "BASE")

	if err != nil || !v.Landed || v.Reason != "" || len(exec.calls) != 1 {
		t.Fatalf("verdict=%+v err=%v calls=%v", v, err, exec.calls)
	}
}

func TestListChanges_RefusesWhatItCannotCompare(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		step    stepF3
		wantErr string
	}{
		{name: "git diff fails", step: stepF3{code: 1, stderr: "boom"}, wantErr: "git diff --raw: rc=1"},
		{name: "a record with the wrong field count", step: stepF3{stdout: ":100644 100644 aaa M\x00a.txt\x00"}, wantErr: "unexpected record"},
		{name: "a record with an extra field", step: stepF3{stdout: ":100644 100644 aaa bbb M extra\x00a.txt\x00"}, wantErr: "unexpected record"},
		{name: "a path with a newline in its name", step: stepF3{stdout: rawRecordF3("100644", "100644", "M", "we\nird.txt")}, wantErr: "has a newline in its name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &scriptedExecF3{byCommand: map[string]stepF3{"diff": tc.step}}

			changes, err := listChanges(context.Background(), exec.git(t.TempDir()), "BASE")

			if changes != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("changes=%v err=%v", changes, err)
			}
		})
	}
}

func TestListChanges_ParsesEveryRecordWithItsModesStatusAndPath(t *testing.T) {
	t.Parallel()
	out := rawRecordF3("100644", "100755", "M", "a b.txt") + rawRecordF3("000000", "100644", "A", "dir/new.go")
	exec := &scriptedExecF3{byCommand: map[string]stepF3{"diff": {stdout: out}}}

	changes, err := listChanges(context.Background(), exec.git(t.TempDir()), "BASE")

	want := []change{{oldMode: "100644", newMode: "100755", status: "M", path: "a b.txt"}, {oldMode: "000000", newMode: "100644", status: "A", path: "dir/new.go"}}
	if err != nil || len(changes) != 2 || changes[0] != want[0] || changes[1] != want[1] {
		t.Fatalf("changes=%+v err=%v", changes, err)
	}
}

func TestReadBlobsAndMainModes_ReportAFailingGitStep(t *testing.T) {
	t.Parallel()
	changes := []change{{oldMode: "100644", newMode: "100755", status: "M", path: "a.txt"}}
	exec := &scriptedExecF3{byCommand: map[string]stepF3{"cat-file": {code: 128, stderr: "no\n"}, "ls-tree": {code: 128, stderr: "no\n"}}}

	blobs, err := readBlobs(context.Background(), exec.git(t.TempDir()), "BASE", changes)
	if blobs != nil || err == nil || !strings.Contains(err.Error(), "git cat-file --batch: rc=128") {
		t.Fatalf("blobs=%v err=%v", blobs, err)
	}
	modes, err := mainModes(context.Background(), exec.git(t.TempDir()), changes)
	if modes != nil || err == nil || !strings.Contains(err.Error(), "ls-tree") {
		t.Fatalf("modes=%v err=%v", modes, err)
	}
}

func TestMainModes_OnlyAChangedModeAsksOriginAndMalformedEntriesAreSkipped(t *testing.T) {
	t.Parallel()
	changes := []change{
		{oldMode: "100644", newMode: "100644", status: "M", path: "same-mode.txt"},
		{oldMode: "100644", newMode: "100755", status: "M", path: "exec.sh"},
	}
	exec := &scriptedExecF3{byCommand: map[string]stepF3{"ls-tree": {stdout: "100755 blob abc\texec.sh\x00garbage without a tab\x00100644 blob\tshort.txt\x00"}}}

	modes, err := mainModes(context.Background(), exec.git(t.TempDir()), changes)

	if err != nil || len(modes) != 1 || modes["exec.sh"] != "100755" {
		t.Fatalf("modes=%v err=%v", modes, err)
	}
	if len(exec.calls) != 1 || !strings.Contains(exec.calls[0], "-- exec.sh") || strings.Contains(exec.calls[0], "same-mode.txt") {
		t.Fatalf("calls = %v", exec.calls)
	}
	untouched := &scriptedExecF3{}
	if modes, err := mainModes(context.Background(), untouched.git(t.TempDir()), changes[:1]); err != nil || len(modes) != 0 || len(untouched.calls) != 0 {
		t.Fatalf("no mode change means no git call: modes=%v err=%v calls=%v", modes, err, untouched.calls)
	}
}

func TestWorktreeContent_ClassifiesWhatTheTreeHoldsAtAPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file.txt", filepath.Join(dir, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	cases := []struct {
		name      string
		path      string
		want      Blob
		wantErr   bool
		wantErrIs error
	}{
		{name: "regular file", path: "file.txt", want: Blob{Data: []byte("body"), Present: true}},
		{name: "symlink is its target text", path: "link", want: Blob{Data: []byte("file.txt"), Present: true}},
		{name: "directory counts as absent", path: "adir", want: Blob{}},
		{name: "missing path counts as absent", path: "nope.txt", want: Blob{}},
		{name: "a path below a regular file cannot be inspected", path: "file.txt/child", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := worktreeContent(dir, change{path: tc.path})

			if (err != nil) != tc.wantErr || got.Present != tc.want.Present || string(got.Data) != string(tc.want.Data) {
				t.Fatalf("got=%+v err=%v, want %+v (err=%v)", got, err, tc.want, tc.wantErr)
			}
			if tc.wantErr && errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("an unreadable path must not read as a missing one: %v", err)
			}
		})
	}
}

func TestJudgePath_EachVerdictNamesItsPathAndReason(t *testing.T) {
	t.Parallel()
	present := func(s string) Blob { return Blob{Data: []byte(s), Present: true} }
	same := changeF3(t, "100644", "100644", "M")
	cases := []struct {
		name       string
		change     change
		v          pathVersions
		wantLanded bool
		wantReason string
	}{
		{name: "deleted on both sides", change: same, v: pathVersions{base: present("b")}, wantLanded: true},
		{name: "deleted in the tree only", change: same, v: pathVersions{base: present("b"), main: present("m")}, wantReason: "x.txt is deleted in the tree but still in origin/main"},
		{name: "added in the tree only", change: same, v: pathVersions{tree: present("t")}, wantReason: "x.txt is not in origin/main"},
		{name: "mode differs and origin has none", change: changeF3(t, "100644", "100755", "M"), v: pathVersions{base: present("b"), main: present("m"), tree: present("m")}, wantReason: "has mode 100755 in the tree but 000000 in origin/main"},
		{name: "mode differs from origin's", change: changeF3(t, "100644", "100755", "M"), v: pathVersions{base: present("b"), main: present("m"), tree: present("m"), mainMode: "100644"}, wantReason: "has mode 100755 in the tree but 100644 in origin/main"},
		{name: "mode matches and bytes equal", change: changeF3(t, "100644", "100755", "M"), v: pathVersions{base: present("b"), main: present("m"), tree: present("m"), mainMode: "100755"}, wantLanded: true},
		{name: "equal bytes", change: same, v: pathVersions{base: present("b"), main: present("m"), tree: present("m")}, wantLanded: true},
		{name: "no merge base copy", change: same, v: pathVersions{main: present("m"), tree: present("t")}, wantReason: "x.txt differs from origin/main"},
		{name: "binary content", change: same, v: pathVersions{base: present("b"), main: present("m\x00"), tree: present("t")}, wantReason: "x.txt differs from origin/main"},
		{name: "symlink change that differs", change: changeF3(t, "120000", "120000", "M"), v: pathVersions{base: present("b"), main: present("m"), tree: present("t"), mainMode: "120000"}, wantReason: "x.txt differs from origin/main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v, err := judgePath(context.Background(), gitexec.Default(t.TempDir()), tc.change, tc.v)

			if err != nil || v.Landed != tc.wantLanded || (tc.wantReason != "" && !strings.Contains(v.Reason, tc.wantReason)) || (tc.wantLanded && v.Reason != "") {
				t.Fatalf("verdict=%+v err=%v", v, err)
			}
		})
	}
}

func changeF3(t *testing.T, oldMode, newMode, status string) change {
	t.Helper()
	return change{oldMode: oldMode, newMode: newMode, status: status, path: "x.txt"}
}

func TestModeOrAbsent_NamesAMissingModeAsZeros(t *testing.T) {
	t.Parallel()

	if modeOrAbsent("") != gitModeAbsent || modeOrAbsent("100644") != "100644" {
		t.Fatalf("modeOrAbsent: %q %q", modeOrAbsent(""), modeOrAbsent("100644"))
	}
}

func TestMergeKeepsMainWith_AFailingWriteStopsBeforeGitRuns(t *testing.T) {
	t.Parallel()
	v := pathVersions{main: Blob{Data: []byte("M")}, base: Blob{Data: []byte("B")}, tree: Blob{Data: []byte("T")}}
	boom := errors.New("disk full")
	writes := 0
	failing := func(string, []byte, fs.FileMode) error {
		writes++
		return boom
	}
	exec := &scriptedExecF3{}

	got, err := mergeKeepsMainWith(context.Background(), exec.git(t.TempDir()).Exec, v, failing)

	if got || !errors.Is(err, boom) || writes != 1 || len(exec.calls) != 0 {
		t.Fatalf("got=%v err=%v writes=%d calls=%v", got, err, writes, exec.calls)
	}
}

func TestMergeKeepsMainWith_StagesTheThreeVersionsUnderTheirNames(t *testing.T) {
	t.Parallel()
	v := pathVersions{main: Blob{Data: []byte("M")}, base: Blob{Data: []byte("B")}, tree: Blob{Data: []byte("T")}}
	staged := map[string]string{}
	record := func(path string, data []byte, _ fs.FileMode) error {
		staged[filepath.Base(path)] = string(data)
		return nil
	}
	exec := &scriptedExecF3{byCommand: map[string]stepF3{"merge-file": {stdout: "M"}}}

	got, err := mergeKeepsMainWith(context.Background(), exec.git(t.TempDir()).Exec, v, record)

	if err != nil || !got || staged["main"] != "M" || staged["base"] != "B" || staged["tree"] != "T" || len(staged) != 3 {
		t.Fatalf("got=%v err=%v staged=%v", got, err, staged)
	}
	if len(exec.calls) != 1 || exec.calls[0] != "merge-file -p --quiet main base tree" {
		t.Fatalf("calls = %v", exec.calls)
	}
}

func TestMergeKeepsMain_EachOutcomeOfTheThreeWayMerge(t *testing.T) {
	t.Parallel()
	v := pathVersions{main: Blob{Data: []byte("main\n")}, base: Blob{Data: []byte("base\n")}, tree: Blob{Data: []byte("tree\n")}}
	cases := []struct {
		name    string
		step    stepF3
		want    bool
		wantErr string
	}{
		{name: "clean merge equal to main", step: stepF3{stdout: "main\n"}, want: true},
		{name: "clean merge that changes main", step: stepF3{stdout: "merged\n"}},
		{name: "conflict", step: stepF3{stdout: "main\n", code: 1}},
		{name: "runner failure", step: stepF3{err: errors.New("exec failed")}, wantErr: "git merge-file: exec failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &scriptedExecF3{byCommand: map[string]stepF3{"merge-file": tc.step}}

			got, err := mergeKeepsMain(context.Background(), exec.git(t.TempDir()).Exec, v)

			if got != tc.want || (tc.wantErr == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}

func TestMergeKeepsMain_AnUnusableTempDirectoryIsAnError(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	exec := &scriptedExecF3{}

	got, err := mergeKeepsMain(context.Background(), exec.git(t.TempDir()).Exec, pathVersions{})

	if got || err == nil || len(exec.calls) != 0 {
		t.Fatalf("got=%v err=%v calls=%v", got, err, exec.calls)
	}
}

func TestListUntracked_EachFailureAndTheEmptyCase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		steps   map[string]stepF3
		want    []UntrackedFile
		wantErr string
	}{
		{name: "ls-files fails", steps: map[string]stepF3{"ls-files": {code: 128, stderr: "no repo\n"}}, wantErr: "git ls-files --others: rc=128"},
		{name: "nothing untracked", steps: map[string]stepF3{"ls-files": {stdout: ""}}},
		{name: "a name with a newline", steps: map[string]stepF3{"ls-files": {stdout: "ok.txt\x00we\nird.txt\x00"}}, wantErr: "has a newline in its name"},
		{name: "hash-object fails", steps: map[string]stepF3{"ls-files": {stdout: "a.txt\x00"}, "hash-object": {code: 128, stderr: "bad\n"}}, wantErr: "git hash-object --stdin-paths: rc=128"},
		{name: "hash-object answers too few", steps: map[string]stepF3{"ls-files": {stdout: "a.txt\x00b.txt\x00"}, "hash-object": {stdout: "h1\n"}}, wantErr: "returned 1 hashes for 2 untracked files"},
		{name: "hash-object answers too many", steps: map[string]stepF3{"ls-files": {stdout: "a.txt\x00"}, "hash-object": {stdout: "h1\nh2\n"}}, wantErr: "returned 2 hashes for 1 untracked files"},
		{name: "names are sorted and paired with their hash", steps: map[string]stepF3{"ls-files": {stdout: "b.txt\x00a.txt\x00"}, "hash-object": {stdout: "ha\nhb\n"}}, want: []UntrackedFile{{Name: "a.txt", Hash: "ha"}, {Name: "b.txt", Hash: "hb"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &scriptedExecF3{byCommand: tc.steps}

			files, err := ListUntracked(context.Background(), exec.git(t.TempDir()))

			if tc.wantErr != "" {
				if files != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("files=%v err=%v", files, err)
				}
				return
			}
			if err != nil || len(files) != len(tc.want) {
				t.Fatalf("files=%v err=%v", files, err)
			}
			for i := range files {
				if files[i] != tc.want[i] {
					t.Fatalf("files=%v, want %v", files, tc.want)
				}
			}
		})
	}
}

func TestListUntracked_ARealFileWithANewlineInItsNameIsRefused(t *testing.T) {
	r := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(r.Dir, "we\nird.txt"), []byte("x"), 0o644); err != nil {
		t.Skip("filesystem refuses newline names:", err)
	}

	files, err := ListUntracked(context.Background(), gitexec.Default(r.Dir))

	if files != nil || err == nil || !strings.Contains(err.Error(), "has a newline in its name") {
		t.Fatalf("files=%v err=%v", files, err)
	}
}

func TestListUntracked_ARealRepositoryWithNothingUntrackedHasNoFiles(t *testing.T) {
	r := gittest.Fixture(t)

	files, err := ListUntracked(context.Background(), gitexec.Default(r.Dir))

	if files != nil || err != nil {
		t.Fatalf("files=%v err=%v", files, err)
	}
}

func TestUntracked_AFailingOrShortAnswerIsAnErrorNotAVerdict(t *testing.T) {
	t.Parallel()
	files := []UntrackedFile{{Name: "a.txt", Hash: "ha"}, {Name: "b.txt", Hash: "hb"}}
	cases := []struct {
		name    string
		step    stepF3
		wantErr string
	}{
		{name: "cat-file fails", step: stepF3{code: 128, stderr: "no\n"}, wantErr: "git cat-file --batch-check=%(objectname): rc=128"},
		{name: "too few answers", step: stepF3{stdout: "ha\n"}, wantErr: "answered 1 of 2 untracked files"},
		{name: "too many answers", step: stepF3{stdout: "ha\nhb\nhc\n"}, wantErr: "answered 3 of 2 untracked files"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &scriptedExecF3{byCommand: map[string]stepF3{"cat-file": tc.step}}

			v, err := Untracked(context.Background(), exec.git(t.TempDir()), files)

			if v.Landed || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("verdict=%+v err=%v", v, err)
			}
		})
	}
	t.Run("every answer equals its hash", func(t *testing.T) {
		t.Parallel()
		exec := &scriptedExecF3{byCommand: map[string]stepF3{"cat-file": {stdout: "ha\nhb\n"}}}

		v, err := Untracked(context.Background(), exec.git(t.TempDir()), files)

		if err != nil || !v.Landed || v.Reason != "" {
			t.Fatalf("verdict=%+v err=%v", v, err)
		}
	})
}

func TestGitIn_FormatsTheFailureWithItsFirstStderrLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		step stepF3
		args []string
		want string
	}{
		{name: "non-zero exit", step: stepF3{code: 128, stderr: "\n  \n fatal: boom \nsecond\n"}, args: []string{"diff", "--raw", "-z"}, want: "git diff --raw: rc=128 err=<nil>: fatal: boom"},
		{name: "runner error", step: stepF3{code: -1, err: errors.New("exec: not found")}, args: []string{"status"}, want: "git status: rc=-1 err=exec: not found: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &scriptedExecF3{byCommand: map[string]stepF3{tc.args[0]: tc.step}}

			out, err := gitIn(context.Background(), exec.git(t.TempDir()), "", tc.args...)

			if out != "" || err == nil || err.Error() != tc.want {
				t.Fatalf("out=%q err=%v, want %q", out, err, tc.want)
			}
		})
	}
	t.Run("success returns stdout untouched", func(t *testing.T) {
		t.Parallel()
		exec := &scriptedExecF3{byCommand: map[string]stepF3{"show": {stdout: " body \n"}}}

		out, err := gitIn(context.Background(), exec.git(t.TempDir()), "", "show")

		if err != nil || out != " body \n" {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
}

func TestFirstLine_IsTheFirstNonBlankTrimmedLine(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                  "",
		"\n \t\n":           "",
		"one":               "one",
		"one\ntwo":          "one",
		"\n\n  padded  \nx": "padded",
		"  \r\nwin\r\n":     "win",
		"héllo wörld\nz":    "héllo wörld",
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
