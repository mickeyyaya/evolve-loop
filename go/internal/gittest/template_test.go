package gittest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func resetTemplates(t *testing.T) {
	t.Helper()
	removeTemplates := func() {
		if err := RemoveTemplates(); err != nil {
			t.Fatalf("RemoveTemplates: %v", err)
		}
	}
	removeTemplates()
	t.Cleanup(removeTemplates)
}

func buildTwoCommitTemplate(r *Repo) error {
	r.Write("a.txt", "one\n")
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "first")
	r.Git("tag", "v1")
	r.Git("branch", "side")
	r.Write("b.txt", "two\n")
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "second")
	return nil
}

func TestFromTemplate_CopyHoldsTheTemplateCommitsBranchesTagsAndTree(t *testing.T) {
	resetTemplates(t)
	got := FromTemplate(t, "two-commit", buildTwoCommitTemplate)
	built := &Repo{Dir: templates.entry("two-commit").dir, tb: t}

	for _, args := range [][]string{
		{"rev-parse", "HEAD"}, {"rev-parse", "v1"}, {"rev-parse", "side"},
		{"for-each-ref", "--format=%(refname) %(objectname)"}, {"log", "--format=%s"},
	} {
		if g, w := got.Git(args...), built.Git(args...); g != w {
			t.Errorf("git %v in the copy = %q, want the template's %q", args, g, w)
		}
	}
	if status := got.Git("status", "--porcelain"); status != "" {
		t.Errorf("a fresh copy is dirty:\n%s", status)
	}
	if body, err := os.ReadFile(filepath.Join(got.Dir, "b.txt")); err != nil || string(body) != "two\n" {
		t.Errorf("copy b.txt = %q, %v, want the committed bytes", body, err)
	}
	assertPersistsMaintenanceConfig(t, got.Dir)
	if email := rawConfig(t, got.Dir, "user.email"); email != "gittest@example.com" {
		t.Errorf("copy identity = %q, want the fixture identity", email)
	}
}

func TestFromTemplate_CopiesDivergeAndNeverTouchTheTemplate(t *testing.T) {
	resetTemplates(t)
	first := FromTemplate(t, "diverge", buildTwoCommitTemplate)
	second := FromTemplate(t, "diverge", buildTwoCommitTemplate)
	head := second.Git("rev-parse", "HEAD")

	first.Write("only-first.txt", "x\n")
	first.Git("add", "-A")
	first.Git("commit", "-q", "-m", "first copy only")
	first.Git("checkout", "-q", "-b", "elsewhere", "v1")
	first.Git("tag", "first-only")

	if got := second.Git("rev-parse", "HEAD"); got != head {
		t.Errorf("second copy HEAD moved to %s after the first copy committed, want %s", got, head)
	}
	if got := second.Git("branch", "--list", "elsewhere"); got != "" {
		t.Errorf("second copy sees the first copy's branch: %q", got)
	}
	if got := second.Git("tag", "--list", "first-only"); got != "" {
		t.Errorf("second copy sees the first copy's tag: %q", got)
	}
	if _, err := os.Stat(filepath.Join(second.Dir, "only-first.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the first copy's file reached the second copy: %v", err)
	}
	third := FromTemplate(t, "diverge", buildTwoCommitTemplate)
	if got := third.Git("rev-parse", "HEAD"); got != head {
		t.Errorf("a copy made after writes elsewhere has HEAD %s, want the template's %s", got, head)
	}
	if got := third.Git("tag", "--list", "first-only"); got != "" {
		t.Errorf("the template was changed by a copy: tag %q", got)
	}
}

func TestFromTemplate_BuildsOnceForManyCallers(t *testing.T) {
	resetTemplates(t)
	execs := failGitExecs(t)
	builds := 0
	build := func(r *Repo) error {
		builds++
		return buildTwoCommitTemplate(r)
	}

	FromTemplate(t, "once", build)
	afterFirst := len(execs.cmds)
	for range 5 {
		FromTemplate(t, "once", build)
	}

	if builds != 1 {
		t.Errorf("the builder ran %d times for 6 callers, want 1", builds)
	}
	if len(execs.cmds) != afterFirst {
		t.Errorf("git ran %d more times for 5 later callers, want 0: a copy needs no git", len(execs.cmds)-afterFirst)
	}
}

func TestFromTemplate_BuildsOnceUnderConcurrentCallers(t *testing.T) {
	resetTemplates(t)
	var mu sync.Mutex
	builds := 0
	build := func(r *Repo) error {
		mu.Lock()
		builds++
		mu.Unlock()
		return buildTwoCommitTemplate(r)
	}
	heads := make([]string, 8)
	var wg sync.WaitGroup
	for i := range heads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			heads[i] = FromTemplate(t, "concurrent", build).Git("rev-parse", "HEAD")
		}()
	}
	wg.Wait()

	if builds != 1 {
		t.Errorf("the builder ran %d times for 8 concurrent callers, want 1", builds)
	}
	for i, h := range heads {
		if h != heads[0] || h == "" {
			t.Errorf("caller %d got HEAD %q, want %q", i, h, heads[0])
		}
	}
}

func TestFromTemplate_NamesAreIndependentTemplates(t *testing.T) {
	resetTemplates(t)
	a := FromTemplate(t, "name-a", func(r *Repo) error { r.Write("a.txt", "a\n"); return nil })
	b := FromTemplate(t, "name-b", func(r *Repo) error { r.Write("b.txt", "b\n"); return nil })

	if _, err := os.Stat(filepath.Join(a.Dir, "b.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("template name-a holds name-b's file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.Dir, "b.txt")); err != nil {
		t.Errorf("template name-b lacks its own file: %v", err)
	}
}

func TestFromTemplate_BuilderErrorReachesEveryCaller(t *testing.T) {
	resetTemplates(t)
	boom := errors.New("builder refused")
	builds := 0
	build := func(*Repo) error { builds++; return boom }

	for i := range 3 {
		rec := &recordingTB{TB: t}
		untilFatal(func() { FromTemplate(rec, "failing", build) })
		if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "failing") || !strings.Contains(rec.errs[0], boom.Error()) {
			t.Fatalf("caller %d failures = %q, want one naming the template and %q", i, rec.errs, boom)
		}
	}
	if builds != 1 {
		t.Errorf("a failing builder ran %d times for 3 callers, want 1", builds)
	}
}

func TestFromTemplate_FailingGitInTheBuilderReachesEveryCaller(t *testing.T) {
	resetTemplates(t)
	build := func(r *Repo) error {
		r.Git("rev-parse", "--verify", "refs/heads/absent")
		return nil
	}

	for i := range 2 {
		rec := &recordingTB{TB: t}
		untilFatal(func() { FromTemplate(rec, "failing-git", build) })
		if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "failing-git") || !strings.Contains(rec.errs[0], "refs/heads/absent") {
			t.Fatalf("caller %d failures = %q, want one naming the template and the failed git", i, rec.errs)
		}
	}
}

func TestFromTemplate_FailingWriteInTheBuilderReachesTheCaller(t *testing.T) {
	resetTemplates(t)
	build := func(r *Repo) error {
		r.Write("a.txt", "x\n")
		r.Write("a.txt/inside", "y\n")
		return nil
	}

	rec := &recordingTB{TB: t}
	untilFatal(func() { FromTemplate(rec, "failing-write", build) })

	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "failing-write") || !strings.Contains(rec.errs[0], "a.txt") {
		t.Fatalf("failures = %q, want one naming the template and the unwritable path", rec.errs)
	}
}

func TestFromTemplate_CopyKeepsModesAndEmptyDirs(t *testing.T) {
	resetTemplates(t)
	build := func(r *Repo) error {
		r.Write("run.sh", "#!/bin/sh\n")
		if err := os.Chmod(filepath.Join(r.Dir, "run.sh"), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(r.Dir, "empty", "deeper"), 0o755); err != nil {
			return err
		}
		r.Git("add", "-A")
		r.Git("commit", "-q", "-m", "modes")
		return nil
	}

	got := FromTemplate(t, "modes", build)

	info, err := os.Stat(filepath.Join(got.Dir, "run.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("run.sh mode = %v, %v, want 0755", info, err)
	}
	if _, err := os.Stat(filepath.Join(got.Dir, "empty", "deeper")); err != nil {
		t.Errorf("empty directory not copied: %v", err)
	}
	if status := got.Git("status", "--porcelain"); status != "" {
		t.Errorf("a copy with modes and links is dirty:\n%s", status)
	}
}

func TestFromTemplate_CopyIsRemovedWithItsTest(t *testing.T) {
	resetTemplates(t)
	var dir string
	t.Run("owner", func(t *testing.T) {
		dir = FromTemplate(t, "owned", buildTwoCommitTemplate).Dir
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("copy missing during its test: %v", err)
		}
	})
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("copy %s survived its test: %v", dir, err)
	}
}

func TestRemoveTemplates_ForgetsBuiltTemplatesAndDeletesTheirDirs(t *testing.T) {
	resetTemplates(t)
	builds := 0
	build := func(r *Repo) error { builds++; return buildTwoCommitTemplate(r) }
	FromTemplate(t, "forgotten", build)
	root := templates.entry("forgotten").dir
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("template dir %s missing after a build: %v", root, err)
	}

	if err := RemoveTemplates(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("template dir %s survived RemoveTemplates: %v", root, err)
	}
	FromTemplate(t, "forgotten", build)
	if builds != 2 {
		t.Errorf("builder ran %d times across a RemoveTemplates, want 2", builds)
	}
}

func TestRepoWrite_CreatesParentDirectories(t *testing.T) {
	r := Fixture(t)
	r.Write("deep/er/file.txt", "body\n")
	if body, err := os.ReadFile(filepath.Join(r.Dir, "deep", "er", "file.txt")); err != nil || string(body) != "body\n" {
		t.Errorf("file = %q, %v, want body", body, err)
	}
}

func TestFromTemplate_AFailedCopyFailsTheCaller(t *testing.T) {
	resetTemplates(t)
	FromTemplate(t, "broken-copy", buildTwoCommitTemplate)
	if err := os.RemoveAll(templates.entry("broken-copy").dir); err != nil {
		t.Fatal(err)
	}

	rec := &recordingTB{TB: t}
	untilFatal(func() { FromTemplate(rec, "broken-copy", buildTwoCommitTemplate) })

	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "broken-copy") {
		t.Fatalf("failures = %q, want one naming the template", rec.errs)
	}
	if !slices.ContainsFunc(rec.errs, func(s string) bool { return strings.Contains(s, "copy") }) {
		t.Errorf("failure does not say the copy failed: %q", rec.errs)
	}
}

func TestFromTemplate_ABuilderPanicReachesEveryCaller(t *testing.T) {
	resetTemplates(t)
	build := func(*Repo) error { panic("builder exploded") }

	for i := range 2 {
		rec := &recordingTB{TB: t}
		untilFatal(func() { FromTemplate(rec, "panicking", build) })
		if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "builder exploded") {
			t.Fatalf("caller %d failures = %q, want one naming the panic", i, rec.errs)
		}
	}
}

func TestFromTemplate_AnUnusableTempRootFailsTheCaller(t *testing.T) {
	resetTemplates(t)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))

	rec := &recordingTB{TB: t}
	untilFatal(func() { FromTemplate(rec, "no-root", buildTwoCommitTemplate) })

	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "no-root") {
		t.Fatalf("failures = %q, want one naming the template", rec.errs)
	}
}

func TestRepoWrite_FailsTheTestWhenThePathIsADirectory(t *testing.T) {
	r := Fixture(t)
	r.Write("dir/file.txt", "x\n")
	rec := &recordingTB{TB: t}

	untilFatal(func() { (&Repo{Dir: r.Dir, tb: rec}).Write("dir", "body") })

	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "dir") {
		t.Fatalf("failures = %q, want one naming dir", rec.errs)
	}
}

func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestSweepDeadTemplates_RemovesOnlyDirsOfExitedProcesses(t *testing.T) {
	tmp := t.TempDir()
	mk := func(name string) string {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	dead := mk(fmt.Sprintf("%s%d-abc", templatePrefix, exitedPID(t)))
	alive := mk(fmt.Sprintf("%s%d-abc", templatePrefix, os.Getpid()))
	unparsable := mk(templatePrefix + "not-a-pid")
	other := mk("unrelated-dir")

	sweepDeadTemplates(tmp)

	if _, err := os.Stat(dead); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the template dir of an exited process survived: %v", err)
	}
	for name, dir := range map[string]string{"live pid": alive, "unparsable pid": unparsable, "unrelated": other} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("the %s dir was removed: %v", name, err)
		}
	}
}

func TestFromTemplate_NamesItsDirAfterTheProcess(t *testing.T) {
	resetTemplates(t)
	FromTemplate(t, "named-by-pid", buildTwoCommitTemplate)

	base := filepath.Base(templates.entry("named-by-pid").dir)
	if !strings.HasPrefix(base, fmt.Sprintf("%s%d-", templatePrefix, os.Getpid())) {
		t.Errorf("template dir %q does not carry this process's pid", base)
	}
}

func TestSignalSaysAlive_OnlyNoErrorAndPermissionDeniedMeanAlive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"signal delivered", nil, true},
		{"process of another user", syscall.EPERM, true},
		{"wrapped permission denied", fmt.Errorf("signal: %w", syscall.EPERM), true},
		{"no such process", syscall.ESRCH, false},
		{"process already finished", os.ErrProcessDone, false},
	}
	for _, tc := range cases {
		if got := signalSaysAlive(tc.err); got != tc.want {
			t.Errorf("%s: signalSaysAlive(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestFromTemplate_ABuilderThatStopsItsGoroutineFailsEveryLaterCaller(t *testing.T) {
	resetTemplates(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		FromTemplate(&recordingTB{TB: t}, "aborting", func(*Repo) error {
			runtime.Goexit()
			return nil
		})
	}()
	<-done

	rec := &recordingTB{TB: t}
	untilFatal(func() { FromTemplate(rec, "aborting", buildTwoCommitTemplate) })
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "aborting") || !strings.Contains(rec.errs[0], errBuilderAborted.Error()) {
		t.Fatalf("later caller failures = %q, want one naming the template and %q", rec.errs, errBuilderAborted)
	}
}
