package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookupKeepsOnlyTheMergedHeadsOfEachBranch(t *testing.T) {
	fakeGhOnPath(t, `echo '[{"headRefName":"a","headRefOid":"aaaa111","state":"MERGED"},{"headRefName":"a","headRefOid":"aaaa222","state":"MERGED"},{"headRefName":"b","headRefOid":"bbbb222","state":"OPEN"},{"headRefName":"c","headRefOid":"cccc333","state":"CLOSED"},{"headRefName":"d","headRefOid":"","state":"MERGED"}]'`)
	o := mustDevOptions(t, "/root", "/root/.evolve", gc.Policy{})
	if o.MergedPRs == nil {
		t.Fatal("devWorktreeGCOptions leaves MergedPRs unwired, so no sweep can read a merged pull request")
	}
	got, err := o.MergedPRs(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"a": {"aaaa111", "aaaa222"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged heads = %v, want %v: an open or closed pull request, and an empty oid, never count as landed", got, want)
	}
}

func TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookupSaysUnavailableWithoutGh(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	o := mustDevOptions(t, "/root", "/root/.evolve", gc.Policy{})
	if o.MergedPRs == nil {
		t.Fatal("devWorktreeGCOptions leaves MergedPRs unwired")
	}
	if _, err := o.MergedPRs(context.Background(), t.TempDir()); !errors.Is(err, gc.ErrPRLookupUnavailable) {
		t.Fatalf("err = %v, want gc.ErrPRLookupUnavailable", err)
	}
}

func TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookupSurfacesAFailingGh(t *testing.T) {
	fakeGhOnPath(t, `echo 'auth required' >&2; exit 4`)
	o := mustDevOptions(t, "/root", "/root/.evolve", gc.Policy{})
	if o.MergedPRs == nil {
		t.Fatal("devWorktreeGCOptions leaves MergedPRs unwired")
	}
	if _, err := o.MergedPRs(context.Background(), t.TempDir()); err == nil || errors.Is(err, gc.ErrPRLookupUnavailable) {
		t.Fatalf("err = %v, want a failure that is not the unavailable sentinel", err)
	}
}

func TestRunGC_DryRunLandsADevWorktreeByItsMergedPullRequestTipAndNotAfterALaterCommit(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	landedDir := h.addDevAt("landed-lane", 0)
	laterDir := h.addDevAt("later-lane", 0)
	landedTip := h.store.Git("-C", landedDir, "rev-parse", "HEAD")
	laterMerged := h.store.Git("-C", laterDir, "rev-parse", "HEAD")
	if err := os.WriteFile(laterDir+"/later.txt", []byte("later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.store.Git("-C", laterDir, "add", "-A")
	h.store.Git("-C", laterDir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "later")
	fakeGhOnPath(t, `case "$*" in *"--state open"*) echo '[]';; *) echo '[{"headRefName":"br-landed-lane","headRefOid":"`+landedTip+`","state":"MERGED"},{"headRefName":"br-later-lane","headRefOid":"`+laterMerged+`","state":"MERGED"}]';; esac`)

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--dry-run", "--project-root", h.runtime}, nil, &stdout, &stderr)
	out := stdout.String()

	if rc != 0 {
		t.Fatalf("rc = %d; stdout=%s stderr=%s", rc, out, stderr.String())
	}
	if !strings.Contains(out, "landed early: branch br-landed-lane is in main") {
		t.Errorf("the branch whose merged pull request sits at its tip was not landed early:\n%s", out)
	}
	if strings.Contains(out, "landed early: branch br-later-lane") {
		t.Errorf("a commit after the merged head was read as landed:\n%s", out)
	}
	if !strings.Contains(out, "(active within the 14-day limit)") {
		t.Errorf("the later-commit branch lost its idle-limit keep:\n%s", out)
	}
}
