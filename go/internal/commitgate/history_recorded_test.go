package commitgate

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const historyAtHead = "package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc F() {}\n"

var historyGroup = commentaudit.HistoryEntry{
	File:   "x.go",
	Line:   3,
	Anchor: "func F() {}",
	Lines:  []string{"// F is the entry point.", "// It used to live in cycle 42's retry loop."},
}

const rootHistoryPage = commentaudit.HistoryArchiveDir + "/root.md"

func TestRun_RemovingHistoryWithoutRecordingItIsRefused(t *testing.T) {
	t.Parallel()
	res := runWaiverCase(t, waiverCase{
		files:   "x.go\n",
		atHead:  map[string]string{"x.go": historyAtHead},
		onDisk:  map[string]string{"x.go": plainTrimmed},
		wantRun: ExitFail,
	})

	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "REJECTED: x.go:3 ") || !strings.Contains(logs, "commentaudit history -base HEAD") || res.Attestation != nil {
		t.Errorf("want a refusal naming x.go:3 and the command that records it, and no attestation: %v", res.Logs)
	}
}

func TestRun_RemovingHistoryTheChangeRecordsIsWaived(t *testing.T) {
	t.Parallel()
	res := runWaiverCase(t, waiverCase{
		files:  "x.go\n" + rootHistoryPage + "\n",
		atHead: map[string]string{"x.go": historyAtHead},
		onDisk: map[string]string{
			"x.go":          plainTrimmed,
			rootHistoryPage: commentaudit.RenderHistorySection("round 13", []commentaudit.HistoryEntry{historyGroup}),
		},
		wantRun: ExitPass,
	})

	if res.Attestation == nil || res.Attestation.ReviewWaiver != commentOnlyWaiver {
		t.Errorf("a comment removal that records its history is still a proven comment-only change: %+v %v", res.Attestation, res.Logs)
	}
}

func TestRun_HistoryMovedWholeToAnotherFileNeedsNoRecord(t *testing.T) {
	t.Parallel()
	res := runWaiverCase(t, waiverCase{
		files:   "x.go\nz.go\n",
		atHead:  map[string]string{"x.go": historyAtHead, "z.go": "package x\n\nfunc G() {}\n"},
		onDisk:  map[string]string{"x.go": plainTrimmed, "z.go": "package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc G() {}\n"},
		wantRun: ExitPass,
	})

	if strings.Contains(strings.Join(res.Logs, "\n"), "REJECTED") {
		t.Errorf("a group that reappears whole elsewhere in the change is moved, not removed: %v", res.Logs)
	}
}

func TestRun_ARewrittenHistoryPageIsRefusedEvenWhenTheChangeIsWaived(t *testing.T) {
	t.Parallel()
	recorded := commentaudit.RenderHistorySection("round 12", []commentaudit.HistoryEntry{historyGroup})
	res := runWaiverCase(t, waiverCase{
		files:   "x.go\n" + rootHistoryPage + "\n",
		atHead:  map[string]string{"x.go": commentedAtHead, rootHistoryPage: recorded},
		onDisk:  map[string]string{"x.go": trimmedComment, rootHistoryPage: "\n## round 12\n"},
		wantRun: ExitFail,
	})

	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "REJECTED: "+rootHistoryPage+" ") || res.Attestation != nil {
		t.Errorf("a comment-only change that cuts the archive must be refused by name before the waiver: %v", res.Logs)
	}
}

func TestRefuseUnrecordedHistory_TheCommandTheRefusalNamesSatisfiesIt(t *testing.T) {
	repo := gittest.Fixture(t)
	mustWrite(t, filepath.Join(repo.Dir, "go", "internal", "p", "p.go"), strings.Replace(historyAtHead, "package x", "package p", 1))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	mustWrite(t, filepath.Join(repo.Dir, "go", "internal", "p", "p.go"), "package p\n\nfunc F() {}\n")
	repo.Git("add", "-A")
	o := Options{RepoRoot: repo.Dir, Runner: sysexec.DefaultRunner}
	ctx := context.Background()

	if res := (&Result{}); o.refuseUnrecordedHistory(ctx, nil, res) != ExitFail {
		t.Fatalf("an unrecorded removal must be refused first: %v", res.Logs)
	}
	var stdout, stderr bytes.Buffer
	if code := commentaudit.Main([]string{"history", "-base", "HEAD", "-label", "round 13", "-out", commentaudit.HistoryArchiveDir}, &stdout, &stderr, stagedGit{repo.Dir, o.git(ctx)}); code != 0 {
		t.Fatalf("commentaudit history: exit %d: %s", code, stderr.String())
	}
	repo.Git("add", "-A")

	if res := (&Result{}); o.refuseUnrecordedHistory(ctx, nil, res) != ExitPass {
		t.Errorf("after the named command records the history, the gate must pass: %v", res.Logs)
	}
}

func TestRefuseUnrecordedHistory_AFaultIsNotARefusal(t *testing.T) {
	for name, c := range map[string]struct {
		rules []scriptRule
		want  int
	}{
		"an unreadable HEAD": {[]scriptRule{
			{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "x.go\x00"},
			{matchPrefix: "git cat-file", exit: 128},
		}, ExitGitFatal},
		"an unlistable change": {[]scriptRule{
			{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 128},
		}, ExitGitFatal},
		"a Go file that does not parse": {[]scriptRule{
			{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "x.go\x00"},
			{matchPrefix: "git show HEAD:x.go", stdout: historyAtHead},
		}, ExitFail},
	} {
		t.Run(name, func(t *testing.T) {
			o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: c.rules}).run()}
			mustWrite(t, filepath.Join(o.RepoRoot, "x.go"), "package x\n\nfunc {\n")
			res := &Result{}

			code := o.refuseUnrecordedHistory(context.Background(), []string{"x.go"}, res)

			if logs := strings.Join(res.Logs, "\n"); code != c.want || !strings.Contains(logs, "history check:") {
				t.Errorf("code = %d logs = %v, want %d naming the history check", code, res.Logs, c.want)
			}
		})
	}
}

func TestRefuseRewrittenHistory_AnUnreadableArchiveIsAFault(t *testing.T) {
	for name, rules := range map[string][]scriptRule{
		"an unreadable HEAD": {
			{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: rootHistoryPage + "\x00"},
			{matchPrefix: "git cat-file", exit: 128},
		},
		"an unlistable change": {{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 128}},
	} {
		t.Run(name, func(t *testing.T) {
			o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: rules}).run()}
			res := &Result{}

			code := o.refuseRewrittenHistory(context.Background(), []string{rootHistoryPage}, res)

			if logs := strings.Join(res.Logs, "\n"); code != ExitGitFatal || !strings.Contains(logs, "history archive check:") {
				t.Errorf("code = %d logs = %v, want ExitGitFatal naming the archive check", code, res.Logs)
			}
		})
	}
}

type stagedGit struct {
	dir string
	run func(args ...string) ([]byte, error)
}

func (g stagedGit) ChangedFiles(base string) ([]string, error) {
	out, err := g.run("diff", "--name-only", "--no-renames", base)
	return strings.Fields(string(out)), err
}

func (g stagedGit) Show(base, path string) ([]byte, error) {
	return commentaudit.ReadAtBase(g.run, base)(path)
}

func (g stagedGit) Root() (string, error) { return g.dir, nil }
