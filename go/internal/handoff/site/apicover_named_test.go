package site_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/site"
)

func TestExportsAreNamed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := site.ForPhase(filepath.Join(root, "ws"), root, "")
	if s.Tree != root {
		t.Fatalf("ForPhase tree = %q", s.Tree)
	}
	if got := site.InboxResolver(root)("x"); got != "" {
		t.Fatalf("resolver = %q", got)
	}
	if _, err := site.ReadLedger(filepath.Join(root, "absent.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadLedger = %v, want a missing-file error", err)
	}
	if _, err := site.BuildRequest(s, 1, "tdd-build"); !request.PairAbsent(err) {
		t.Fatalf("BuildRequest = %v, want an absent pair", err)
	}
	var ev site.Evaluation = site.Evaluate(s, 1, "tdd-build")
	if !ev.RequestAbsent {
		t.Fatalf("Evaluate = %+v, want RequestAbsent", ev)
	}
}
