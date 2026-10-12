package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCampaignEvolveDir_AbsolutizesRelativeRoot(t *testing.T) {
	t.Parallel()
	got := campaignEvolveDir("relative/root")
	if !filepath.IsAbs(got) {
		t.Fatalf("campaignEvolveDir must return an absolute path, got %q", got)
	}
	if !strings.HasSuffix(got, filepath.Join("relative", "root", ".evolve")) {
		t.Fatalf("path should end in relative/root/.evolve, got %q", got)
	}
}

func TestCampaignEvolveDir_EmptyRootIsAbsolute(t *testing.T) {
	t.Parallel()
	got := campaignEvolveDir("")
	if !filepath.IsAbs(got) {
		t.Fatalf("empty root must resolve to an absolute cwd-based path, got %q", got)
	}
	if !strings.HasSuffix(got, ".evolve") {
		t.Fatalf("path should end in .evolve, got %q", got)
	}
}
