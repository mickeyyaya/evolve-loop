package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const consoleSeatDiff = "diff --git a/x.go b/x.go\n+func X() {}\n"

func consoleChainProject(t *testing.T, names ...string) string {
	t.Helper()
	root := diffReviewerProject(t, names[0])
	repo := repoRootForSkills(t)
	for _, name := range names[1:] {
		for _, rel := range []string{".evolve/phases/" + name + "/phase.json", ".evolve/profiles/diff-" + name + ".json", "agents/evolve-diff-" + name + ".md"} {
			copyFileForTest(t, filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
		}
	}
	return root
}

func consoleSeatRun(t *testing.T, root, name string, tree string) agentcmd.Envelope {
	t.Helper()
	b := &reviewerBridge{report: diffReviewerReports[name]}
	run := agentcmd.New(agentcmd.Deps{
		InstallRouter: rootRouterInstaller,
		Bridge:        func(string, *signalcenter.Center) core.Bridge { return b },
		Signals:       quietSignals,
		TreeState:     fixedTreeState(tree),
		Getenv:        func(string) string { return "" },
		Now:           func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", name, "--json", "--project-root", root}, strings.NewReader(consoleSeatDiff), &stdout, &stderr); code != 0 {
		t.Fatalf("%s: exit %d; stderr=%s", name, code, stderr.String())
	}
	var env agentcmd.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return env
}

func consoleGateRunner(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	cmd := strings.TrimSpace(name + " " + strings.Join(args, " "))
	switch {
	case cmd == "git diff --name-only HEAD":
		_, _ = io.WriteString(stdout, "x.go\n")
	case cmd == "git diff HEAD":
		_, _ = io.WriteString(stdout, consoleSeatDiff)
		return 1, nil
	}
	return 0, nil
}

func TestConsoleReviewChain_ANonClaudeSeatReviewsAndTheGateAttestsTheBridgeRecords(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	root := consoleChainProject(t, "code-simplifier", "go-reviewer")
	sum := sha256.Sum256([]byte(consoleSeatDiff))
	tree := hex.EncodeToString(sum[:])

	var tokens []string
	for _, name := range []string{"code-simplifier", "go-reviewer"} {
		env := consoleSeatRun(t, root, name, tree)
		rec, err := launchrecord.Read(filepath.Join(root, ".evolve"), env.RunLabel)
		if err != nil {
			t.Fatalf("%s left no bridge launch record: %v", name, err)
		}
		if rec.CLI == "" || isClaudeFamilyDriver(rec.CLI) || rec.Family == "claude" || !rec.BoundTo(tree) {
			t.Fatalf("%s record = %+v, want a non-Claude driver bound to the staged tree", name, rec)
		}
		tokens = append(tokens, "run:"+env.RunLabel)
	}
	if err := os.WriteFile(filepath.Join(root, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := commitgate.Options{
		RepoRoot: root, Reviewers: strings.Join(tokens, ","), Runner: consoleGateRunner, NoInstall: true,
		Env: os.Environ(), Now: func() time.Time { return time.Date(2026, 10, 27, 9, 5, 0, 0, time.UTC) },
	}.Run(context.Background())

	if res.ExitCode != commitgate.ExitPass || res.Attestation == nil {
		t.Fatalf("gate exit %d, want pass; logs=%v", res.ExitCode, res.Logs)
	}
	if got := strings.Join(res.Attestation.ReviewersRun, ","); got != "code-simplifier,go-reviewer" {
		t.Errorf("reviewers_run = %q, want the profiles of the two runs", got)
	}
	if len(res.Attestation.ReviewRuns) != 2 {
		t.Fatalf("review_runs = %+v, want the two bridge runs", res.Attestation.ReviewRuns)
	}
	for _, r := range res.Attestation.ReviewRuns {
		if isClaudeFamilyDriver(r.CLI) || r.Family == "claude" || r.Model == "" {
			t.Errorf("review run %+v, want a non-Claude family and a model", r)
		}
	}
}

func TestConsoleBuildSeat_RoutingDataMovesTheSeatToAnotherFamily(t *testing.T) {
	orig := runner.DefaultRouter
	t.Cleanup(func() { runner.DefaultRouter = orig })
	repo := repoRootForSkills(t)
	root := t.TempDir()
	for _, rel := range []string{"docs/architecture/phase-registry.json", ".evolve/phases/console-builder/phase.json", ".evolve/profiles/console-builder.json", "agents/evolve-console-builder.md"} {
		copyFileForTest(t, filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
	}
	agyHoldsTheSeat := `{"cli_routing": {"clis": ["agy", "claude"], "default": ["claude", "agy"], "agents": {"console-builder": {"cli": ["agy"]}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(agyHoldsTheSeat), 0o644); err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+filepath.Join(t.TempDir(), "wt")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &reviewerBridge{report: "## Changes\nx.go\n## Tests\n1/1 PASS\n## Verdict\nVerdict: PASS\n"}
	run := agentcmd.New(agentcmd.Deps{
		InstallRouter: rootRouterInstaller,
		Bridge:        func(string, *signalcenter.Center) core.Bridge { return b },
		Signals:       quietSignals,
		TreeState:     fixedTreeState("tree"),
		Getenv:        func(string) string { return "" },
		Now:           func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
	})
	var stdout, stderr bytes.Buffer

	code := run([]string{"run", "console-builder", "--project-root", root, "--worktree", wt, "--writable", "go/x.go"}, strings.NewReader("build item X"), &stdout, &stderr)

	if code != 0 || len(b.requests) != 1 {
		t.Fatalf("exit %d launches %d, want 0 and one launch; stderr=%s", code, len(b.requests), stderr.String())
	}
	if req := b.requests[0]; !strings.HasPrefix(req.CLI, "agy") || isClaudeFamilyDriver(req.CLI) || req.Worktree != wt {
		t.Errorf("launch cli=%q worktree=%q, want an agy driver in the worktree %s", req.CLI, req.Worktree, wt)
	}
}

func TestAgentRunRoot_PublishesUnderTheAgentRoleAndWiresTheBridge(t *testing.T) {
	useRealRootEvents(t)
	root := t.TempDir()
	signals, closeSignals := agentRunSignals(root, io.Discard)
	if b, ok := agentRunBridge(root, signals).(interface{ SignalsWired() bool }); !ok || !b.SignalsWired() {
		t.Fatal("the agent run bridge does not carry the Signal Center of its root")
	}
	signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleIsolation, Origin: "Test.emit", Kind: signalcenter.KindIsolationViolation,
		Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(3), Reason: "a nested launch",
	})
	closeSignals()

	log, err := channel.New(paths.EventChannelsRoot(filepath.Join(root, ".evolve")), "isolation.violation", channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := log.Read(0)
	if err != nil || len(batch.Records) != 1 || batch.Records[0].Source != roleAgent {
		t.Fatalf("isolation channel = %+v err=%v, want one record from the role %s", batch.Records, err, roleAgent)
	}
}
