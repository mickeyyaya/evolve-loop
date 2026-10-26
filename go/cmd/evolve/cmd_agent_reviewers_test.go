package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

var diffReviewerReports = map[string]string{
	"architecture-reviewer": "## Architecture Review Verdict\n### Findings\nNone.\n### Verdict\nVerdict: PASS\n",
	"code-reviewer":         "## Findings\nNone.\n## Verdict\nVerdict: PASS\n",
	"code-simplifier":       "## Simplifications\nNothing is worth a change.\n",
	"go-reviewer":           "## Findings\nNone.\n## Verdict\nVerdict: PASS\n",
}

const agyFirstRouting = `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}` + "\n"

type reviewerBridge struct {
	report   string
	requests []core.BridgeRequest
}

func (b *reviewerBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.requests = append(b.requests, req)
	if err := os.WriteFile(req.ArtifactPath, []byte(b.report), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	return core.BridgeResponse{}, nil
}

func (b *reviewerBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func diffReviewerProject(t *testing.T, name string) string {
	t.Helper()
	repo := repoRootForSkills(t)
	root := t.TempDir()
	for _, rel := range []string{
		"docs/architecture/phase-registry.json",
		".evolve/phases/" + name + "/phase.json",
		".evolve/profiles/diff-" + name + ".json",
		"agents/evolve-diff-" + name + ".md",
	} {
		copyFileForTest(t, filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(agyFirstRouting), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func isClaudeFamilyDriver(cli string) bool {
	return strings.HasPrefix(cli, "claude") || strings.HasPrefix(cli, "agy-claude")
}

func TestAgentRun_EachDiffReviewerRunsOnANonClaudeFamily(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	for name, report := range diffReviewerReports {
		t.Run(name, func(t *testing.T) {
			root := diffReviewerProject(t, name)
			b := &reviewerBridge{report: report}
			run := agentcmd.New(agentcmd.Deps{
				InstallRouter: rootRouterInstaller,
				Bridge:        func(string) core.Bridge { return b },
				Getenv:        func(string) string { return "" },
				Now:           func() time.Time { return time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC) },
			})
			var stdout, stderr bytes.Buffer
			code := run([]string{"run", name, "--project-root", root},
				strings.NewReader("diff --git a/x.go b/x.go\n+func X() {}\n"), &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit %d, want 0; stderr=%s", code, stderr.String())
			}
			if len(b.requests) != 1 {
				t.Fatalf("launches = %d, want 1", len(b.requests))
			}
			req := b.requests[0]
			if req.CLI == "" || isClaudeFamilyDriver(req.CLI) {
				t.Fatalf("launch CLI = %q, want a driver of a non-Claude family", req.CLI)
			}
			if !strings.Contains(req.Prompt, "+func X() {}") {
				t.Error("the prompt does not carry the diff input")
			}
			if stdout.String() != report {
				t.Errorf("stdout = %q, want the report %q", stdout.String(), report)
			}
		})
	}
}
