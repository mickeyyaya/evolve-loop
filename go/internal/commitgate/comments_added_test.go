package commitgate

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const uncommentedAtHead = "package x\n\nfunc F() {}\n"

func TestRun_ACommitThatAddsACommentIsRefused(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]waiverCase{
		"a comment added to a changed file": {
			files:  "x.go\n",
			atHead: map[string]string{"x.go": uncommentedAtHead},
			onDisk: map[string]string{"x.go": "package x\n\n// F explains itself here.\nfunc F() {}\n"},
		},
		"a comment in a new file": {
			files:  "y.go\n",
			onDisk: map[string]string{"y.go": "package x\n\nfunc G() {\n\t// narration\n}\n"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c.wantRun = ExitFail
			res := runWaiverCase(t, c)
			if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "adds a comment") || res.Attestation != nil {
				t.Errorf("want a refusal naming the added comment and no attestation: %v", res.Logs)
			}
		})
	}
}

func TestRun_WhatAToolReadsIsNotAnAddedComment(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]waiverCase{
		"a directive": {
			files:   "x.go\n",
			atHead:  map[string]string{"x.go": uncommentedAtHead},
			onDisk:  map[string]string{"x.go": "package x\n\nfunc F() {\n\t_ = 1 //nolint:errcheck\n}\n"},
			wantRun: ExitFail,
		},
		"a new file's package doc": {
			files:   "x.go\n",
			onDisk:  map[string]string{"x.go": "// Package x names what it is for in six words.\npackage x\n\nfunc F() {}\n"},
			wantRun: ExitFail,
		},
		"a comment moved between files": {
			files:   "x.go\nz.go\n",
			atHead:  map[string]string{"x.go": uncommentedAtHead, "z.go": "package x\n\n// F is the entry point.\nfunc G() {}\n"},
			onDisk:  map[string]string{"x.go": "package x\n\n// F is the entry point.\nfunc F() {}\n", "z.go": "package x\n\nfunc G() {}\n"},
			wantRun: ExitPass,
		},
	} {
		t.Run(name, func(t *testing.T) {
			res := runWaiverCase(t, c)
			if strings.Contains(strings.Join(res.Logs, "\n"), "adds a comment") {
				t.Errorf("%s must not be refused as an added comment: %v", name, res.Logs)
			}
		})
	}
}

func TestRefuseAddedComments_ARenamedCommentedFileAddsNoComment(t *testing.T) {
	repo := gittest.Fixture(t)
	mustWrite(t, filepath.Join(repo.Dir, "a.go"), commentedAtHead)
	repo.Git("add", "a.go")
	repo.Git("commit", "-q", "-m", "base")
	repo.Git("mv", "a.go", "b.go")
	o := Options{RepoRoot: repo.Dir, Runner: sysexec.DefaultRunner}

	if res := (&Result{}); o.refuseAddedComments(context.Background(), []string{"b.go"}, res) != ExitPass {
		t.Fatalf("a git mv of a commented file must not read as added comments: %v", res.Logs)
	}

	mustWrite(t, filepath.Join(repo.Dir, "b.go"), commentedAtHead+"\n// new narration\nfunc G() {}\n")
	res := &Result{}
	if code := o.refuseAddedComments(context.Background(), []string{"b.go"}, res); code != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "// new narration") {
		t.Errorf("a renamed file that gains a comment: code = %d logs = %v, want the new comment refused", code, res.Logs)
	}
}

func TestRefuseAddedComments_AnUnreadableHeadIsAFaultNotANewFile(t *testing.T) {
	o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "x.go\x00"},
		{matchPrefix: "git cat-file", exit: 128},
	}}).run()}
	mustWrite(t, filepath.Join(o.RepoRoot, "x.go"), "package x\n\n// a comment\nfunc F() {}\n")
	res := &Result{}

	if code := o.refuseAddedComments(context.Background(), []string{"x.go"}, res); code != ExitGitFatal {
		t.Errorf("code = %d logs = %v, want ExitGitFatal: an unreadable HEAD is not a new file", code, res.Logs)
	}
}

func TestRefuseAddedComments_AnUnlistableChangeIsAFault(t *testing.T) {
	o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 128},
	}}).run()}
	res := &Result{}

	if code := o.refuseAddedComments(context.Background(), []string{"x.go"}, res); code != ExitGitFatal {
		t.Errorf("code = %d logs = %v, want ExitGitFatal when the change cannot be listed", code, res.Logs)
	}
}

func TestRefuseAddedComments_ANonASCIIPathIsRead(t *testing.T) {
	repo := gittest.Fixture(t)
	mustWrite(t, filepath.Join(repo.Dir, "base.go"), uncommentedAtHead)
	repo.Git("add", "base.go")
	repo.Git("commit", "-q", "-m", "base")
	mustWrite(t, filepath.Join(repo.Dir, "é.go"), "package x\n\nfunc G() {\n\t// narration\n}\n")
	repo.Git("add", "é.go")
	o := Options{RepoRoot: repo.Dir, Runner: sysexec.DefaultRunner}
	res := &Result{}

	if code := o.refuseAddedComments(context.Background(), nil, res); code != ExitFail || !strings.Contains(strings.Join(res.Logs, "\n"), "// narration") {
		t.Errorf("code = %d logs = %v, want the comment in é.go refused: git quotes a non-ASCII path unless the listing uses -z", code, res.Logs)
	}
}

func TestRefuseAddedComments_AFaultNamesItsCause(t *testing.T) {
	for name, c := range map[string]struct {
		rule scriptRule
		want string
	}{
		"git exits non-zero":  {scriptRule{matchPrefix: "git diff --name-only --no-renames -z HEAD", exit: 128}, "rc=128"},
		"git cannot be run":   {scriptRule{matchPrefix: "git diff --name-only --no-renames -z HEAD", err: errors.New("exec: git not found")}, "exec: git not found"},
		"HEAD cannot be read": {scriptRule{matchPrefix: "git cat-file", exit: 128}, "rc=128"},
		"git show cannot run": {scriptRule{matchPrefix: "git cat-file", err: errors.New("exec: git not found")}, "exec: git not found"},
	} {
		t.Run(name, func(t *testing.T) {
			rules := []scriptRule{c.rule}
			if !strings.HasPrefix(c.rule.matchPrefix, "git diff") {
				rules = append([]scriptRule{{matchPrefix: "git diff --name-only --no-renames -z HEAD", stdout: "x.go\x00"}}, rules...)
			}
			o := Options{RepoRoot: t.TempDir(), Runner: (&scriptRunner{rules: rules}).run()}
			mustWrite(t, filepath.Join(o.RepoRoot, "x.go"), "package x\n\n// a comment\nfunc F() {}\n")
			res := &Result{}

			code := o.refuseAddedComments(context.Background(), []string{"x.go"}, res)

			logs := strings.Join(res.Logs, "\n")
			if code != ExitGitFatal || !strings.Contains(logs, c.want) || strings.Contains(logs, "%!") {
				t.Errorf("code = %d logs = %q, want ExitGitFatal naming %q with no formatting residue", code, logs, c.want)
			}
		})
	}
}
