package core

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestBenchedCLIsForRouting_ProjectsActiveSorted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Now()
	st := clihealth.NewStore(root, nil)
	_ = st.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit",
		BenchedAt: now, BenchedUntil: now.Add(time.Hour)})
	_ = st.Bench(clihealth.Entry{Family: "agy", Reason: "rate_limit",
		BenchedAt: now, BenchedUntil: now.Add(time.Hour)})
	_ = st.Bench(clihealth.Entry{Family: "claude", Reason: "rate_limit",
		BenchedAt: now.Add(-2 * time.Hour), BenchedUntil: now.Add(-time.Hour)}) // expired — excluded

	got := benchedCLIsForRouting(root)
	if len(got) != 2 || got[0].Family != "agy" || got[1].Family != "codex" {
		t.Errorf("got %+v, want [agy codex] (active only, family-sorted)", got)
	}
	if benchedCLIsForRouting(t.TempDir()) != nil {
		t.Error("empty store must project nil (no prompt section)")
	}
}
