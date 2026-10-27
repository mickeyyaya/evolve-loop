//go:build integration

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	contractSuccess  = "success"
	contractUsage    = "usage"
	contractNegative = "negative"
)

var contractNoNegative = map[string]string{
	"clihealth list": "the list reads the bench file and treats an absent or unreadable file as no bench",
	"quality report": "the report exits 0 on each reading; only a usage error fails it",
	"quality show":   "the show exits 0 on each reading, as the report does; only a usage error fails it",
	"events kinds":   "the vocabulary is compiled into the binary; only a usage error fails it",
}

type contractCall struct {
	kind   string
	prep   func(t *testing.T) map[string]string
	args   []string
	stdin  string
	want   int
	unheld bool
	run    func(t *testing.T, vars map[string]string) func(args []string, stdout, stderr io.Writer) int
}

type contractVerbCases struct {
	verb  string
	calls []contractCall
}

type contractBridge struct{ report string }

func (b contractBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{CostUSD: 0.01}, os.WriteFile(req.ArtifactPath, []byte(b.report), 0o644)
}

func (contractBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func contractWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contractCopyRepo(t *testing.T, root string, rels ...string) {
	t.Helper()
	repo := findRepoRoot(t)
	for _, rel := range rels {
		raw, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatal(err)
		}
		contractWrite(t, root, rel, string(raw))
	}
}

func contractPlane(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	contractWrite(t, root, ".evolve/inbox/.keep", "")
	contractWrite(t, root, ".evolve/wave-goal.md", "Fix what the inbox ranks first.\n")
	contractWrite(t, root, ".evolve/state.json", `{"lastCycleNumber":3}`)
	contractWrite(t, root, "goal.txt", "fix the parser\n")
	return map[string]string{"root": root, "goal": filepath.Join(root, "goal.txt")}
}

func contractBareRoot(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"root": t.TempDir()}
}

func contractInboxItem(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-", "--project-root", vars["root"]}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Fatalf("file the item: rc %d %s", rc, stderr.String())
	}
	matches, _ := filepath.Glob(filepath.Join(vars["root"], ".evolve", "inbox", "*-cli-inbox-show.json"))
	vars["item"] = matches[0]
	return vars
}

func contractLiveLease(t *testing.T, root string) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func contractLeasedItem(t *testing.T) map[string]string {
	t.Helper()
	vars := contractInboxItem(t)
	contractLiveLease(t, vars["root"])
	return vars
}

func contractRouting(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"root": routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)}
}

func contractBadPolicy(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractWrite(t, vars["root"], ".evolve/policy.json", "{not json")
	return vars
}

func contractGitRoot(t *testing.T) map[string]string {
	t.Helper()
	r := gittest.Fixture(t)
	contractWrite(t, r.Dir, "README.md", "fixture\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	contractWrite(t, r.Dir, ".evolve/inbox/.keep", "")
	return map[string]string{"root": r.Dir}
}

func contractCheckpointHub(t *testing.T) map[string]string {
	t.Helper()
	tickingCheckpointClock(t)
	h := newCheckpointHub(t)
	console := h.worktree("console", "")
	dirty := h.worktree("dev/dirty", "feat/dirty")
	contractWrite(t, dirty, "a.txt", "work in progress\n")
	var stdout, stderr bytes.Buffer
	if rc := runCheckpoint([]string{"save", "--all", "--project-root", console}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("seed a checkpoint: %s", stderr.String())
	}
	ref := strings.TrimPrefix(strings.TrimSpace(h.refs()), "refs/checkpoints/")
	return map[string]string{"root": console, "ref": ref, "into": filepath.Join(h.root, "dev", "restored")}
}

func contractSyncRepo(t *testing.T) map[string]string {
	t.Helper()
	repo, _ := smInitRepoWithRemote(t)
	return map[string]string{"root": repo}
}

func contractDirtySyncRepo(t *testing.T) map[string]string {
	t.Helper()
	vars := contractSyncRepo(t)
	contractWrite(t, vars["root"], "base.txt", "a local edit\n")
	return vars
}

func contractResetRoot(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	seedReset(t, root, 395)
	return map[string]string{"root": root}
}

func contractLiveResetRoot(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	_, ws := seedReset(t, root, 395)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN9", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"root": root}
}

func contractWaveRecorded(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	if err := wave.NewStore(filepath.Join(vars["root"], ".evolve")).Save(wave.Record{Number: 1, PID: 999999}); err != nil {
		t.Fatal(err)
	}
	contractWrite(t, vars["root"], ".evolve/runs/.keep", "")
	return vars
}

func contractAgentRoot(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	contractCopyRepo(t, root, "docs/architecture/phase-registry.json", ".evolve/phases/adversarial-review/phase.json",
		".evolve/profiles/adversarial-review.json", "agents/evolve-adversarial-review.md")
	return map[string]string{"root": root}
}

func contractSteRoot(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractCopyRepo(t, vars["root"], "docs/conventions/ste100-writing.md")
	contractWrite(t, vars["root"], "note.md", "# Note\n\nThe verb prints one envelope.\n")
	contractWrite(t, vars["root"], "bad.md", "# Bad\n\nIt is utilized in order to facilitate the leveraging of things.\n")
	return vars
}

func contractCIRoot(t *testing.T) map[string]string {
	t.Helper()
	newCIFakeGH(t, map[string]string{"run.json": ciRunDoc(501, "completed", "success"), "compare.json": ciCompareDoc})
	return map[string]string{"root": ciProjectRoot(t, "", true)}
}

func contractAgentRun(t *testing.T, vars map[string]string) func([]string, io.Writer, io.Writer) int {
	t.Helper()
	run := agentcmd.New(agentcmd.Deps{
		Bridge: func(string, *signalcenter.Center) core.Bridge {
			return contractBridge{report: "## Threat Model\nOne function.\n## Findings\nNone.\n## Verdict\nPASS\n"}
		},
		Getenv: func(string) string { return "" },
	})
	return func(args []string, stdout, stderr io.Writer) int {
		return run(args[1:], strings.NewReader("diff --git a/x.go b/x.go\n+func X() {}\n"), stdout, stderr)
	}
}

func contractPRMerge(t *testing.T, _ map[string]string) func([]string, io.Writer, io.Writer) int {
	t.Helper()
	green := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	gh := &fakePRGitHub{
		views:  map[string]string{"12": openPR("h12")},
		behind: map[string]string{"h12": `{"behind_by":0}`},
		merged: map[string]string{"12": `{"state":"MERGED","mergeCommit":{"oid":"` + mergeSHA + `"}}`},
	}
	return func(args []string, stdout, stderr io.Writer) int {
		a, err := parsePRMergeArgs(args[2:])
		if err != nil {
			t.Fatal(err)
		}
		m, _, _ := greenMerger(gh, green, nil)
		m.stdout, m.stderr = stdout, stderr
		return mergePRs(m, a, stdout, stderr)
	}
}

func contractUsagePanes(t *testing.T) map[string]string {
	t.Helper()
	installUsagePanes(t, map[string]string{"claude": usageFixture(t, "claude_usage_fable_exhausted.txt")},
		map[string]error{"agy": io.ErrUnexpectedEOF})
	return contractPlane(t)
}

func contractCLIUpdateHost(t *testing.T) map[string]string {
	t.Helper()
	claudeRaisedHost().install(t)
	return contractPlane(t)
}

func contractLiveCLIUpdateHost(t *testing.T) map[string]string {
	t.Helper()
	vars := contractCLIUpdateHost(t)
	contractLiveLease(t, vars["root"])
	return vars
}

func contractGoCache(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractWrite(t, vars["root"], ".evolve/policy.json", `{"gc":{"go_cache_ttl_hours":1}}`)
	return vars
}
