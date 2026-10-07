package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

func campaignProject(t *testing.T, policyBody string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "preliminary-study.json"), []byte(`{"name":"preliminary-study","cli":"codex-tmux","model_tier_default":"balanced"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	runner.DefaultRouter = nil
	return root
}

func TestInstallRootRouter_TheCampaignRoutesItsPhaseThroughTheCompiledTable(t *testing.T) {
	root := campaignProject(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)

	err := installRootRouter(root, io.Discard)

	if err != nil || runner.DefaultRouter == nil || runner.DefaultRouter.Policy().CLIRouting == nil {
		t.Fatalf("the campaign root installs the router it compiled, so its runner is never refused: err=%v router=%v", err, runner.DefaultRouter)
	}
}

func TestInstallRootRouter_ATableThatDoesNotCompileIsAnErrorAndInstallsNothing(t *testing.T) {
	root := campaignProject(t, `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)

	err := installRootRouter(root, io.Discard)

	if err == nil || runner.DefaultRouter != nil {
		t.Fatalf("a table with no claude is refused before any dispatch: err=%v router=%v", err, runner.DefaultRouter)
	}
}
