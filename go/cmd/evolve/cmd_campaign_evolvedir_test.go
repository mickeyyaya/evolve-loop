package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// campaignEvolveDir must always yield an ABSOLUTE .evolve path so a relative
// --project-root on the `campaign status` path resolves the same progress file
// the `campaign run` path wrote (run absolutizes its root; status must too).
func TestCampaignEvolveDir_AbsolutizesRelativeRoot(t *testing.T) {
	got := campaignEvolveDir("relative/root")
	if !filepath.IsAbs(got) {
		t.Fatalf("campaignEvolveDir must return an absolute path, got %q", got)
	}
	if !strings.HasSuffix(got, filepath.Join("relative", "root", ".evolve")) {
		t.Fatalf("path should end in relative/root/.evolve, got %q", got)
	}
}

func TestCampaignEvolveDir_EmptyRootIsAbsolute(t *testing.T) {
	got := campaignEvolveDir("")
	if !filepath.IsAbs(got) {
		t.Fatalf("empty root must resolve to an absolute cwd-based path, got %q", got)
	}
	if !strings.HasSuffix(got, ".evolve") {
		t.Fatalf("path should end in .evolve, got %q", got)
	}
}
