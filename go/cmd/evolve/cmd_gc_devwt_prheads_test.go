package main

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestDevWorktreeGCOptions_TheWiredPullRequestHeadLookupReadsTheHeadsFromGh(t *testing.T) {
	fakeGhOnPath(t, `echo '[{"headRefName":"a","headRefOid":"aaaa111","state":"MERGED"},{"headRefName":"b","headRefOid":"bbbb222","state":"OPEN"},{"headRefName":"c","headRefOid":"","state":"CLOSED"}]'`)
	o := mustDevOptions(t, "/root", "/root/.evolve", gc.Policy{})
	if o.PRHeads == nil {
		t.Fatal("devWorktreeGCOptions leaves PRHeads unwired, so no sweep can tell a pull request head from a unique head")
	}
	heads, err := o.PRHeads(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !heads["aaaa111"] || !heads["bbbb222"] || len(heads) != 2 {
		t.Fatalf("heads = %v, want exactly the two non-empty head oids of every pull request state", heads)
	}
}

func TestDevWorktreeGCOptions_TheWiredPullRequestHeadLookupSaysUnavailableWithoutGh(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	o := mustDevOptions(t, "/root", "/root/.evolve", gc.Policy{})
	if o.PRHeads == nil {
		t.Fatal("devWorktreeGCOptions leaves PRHeads unwired")
	}
	if _, err := o.PRHeads(context.Background(), t.TempDir()); !errors.Is(err, gc.ErrPRLookupUnavailable) {
		t.Fatalf("err = %v, want gc.ErrPRLookupUnavailable", err)
	}
}

func TestDevWorktreeGCOptions_TheWiredPullRequestHeadLookupSurfacesAFailingGh(t *testing.T) {
	fakeGhOnPath(t, `echo 'auth required' >&2; exit 4`)
	o := mustDevOptions(t, "/root", "/root/.evolve", gc.Policy{})
	if o.PRHeads == nil {
		t.Fatal("devWorktreeGCOptions leaves PRHeads unwired")
	}
	if _, err := o.PRHeads(context.Background(), t.TempDir()); err == nil || errors.Is(err, gc.ErrPRLookupUnavailable) {
		t.Fatalf("err = %v, want a real failure", err)
	}
}
