//go:build integration

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func okCall(prep func(*testing.T) map[string]string, args ...string) contractCall {
	return contractCall{kind: contractSuccess, prep: prep, args: args}
}

func usageCall(prep func(*testing.T) map[string]string, args ...string) contractCall {
	return contractCall{kind: contractUsage, prep: prep, args: args, want: cliExitUsage}
}

func failCall(want int, prep func(*testing.T) map[string]string, args ...string) contractCall {
	return contractCall{kind: contractNegative, prep: prep, args: args, want: want}
}

func unheld(c contractCall) contractCall {
	c.unheld = true
	return c
}

func withStdin(c contractCall, stdin string) contractCall {
	c.stdin = stdin
	return c
}

func custom(c contractCall, run func(*testing.T, map[string]string) func([]string, io.Writer, io.Writer) int) contractCall {
	c.run = run
	return c
}

const prFlag = "--project-root"

var contractCases = []contractVerbCases{
	{"status", []contractCall{okCall(contractPlane, "status", prFlag, "{root}"), usageCall(contractPlane, "status", "extra", prFlag, "{root}"), failCall(cliExitIO, contractBareRoot, "status", prFlag, "{root}")}},
	{"loop status", []contractCall{okCall(contractPlane, "loop", "status", prFlag, "{root}"), usageCall(contractPlane, "loop", "status", "foo", prFlag, "{root}"), failCall(cliExitIO, contractBareRoot, "loop", "status", prFlag, "{root}")}},
	{"wave status", []contractCall{okCall(contractPlane, "wave", "status", prFlag, "{root}"), usageCall(contractPlane, "wave", "status", "--bogus", prFlag, "{root}"), failCall(cliExitIO, contractBadWaves, "wave", "status", prFlag, "{root}")}},
	{"wave supply", []contractCall{okCall(contractPlane, "wave", "supply", prFlag, "{root}"), usageCall(contractPlane, "wave", "supply", "--bogus", prFlag, "{root}"), failCall(cliExitIO, contractInboxFile, "wave", "supply", prFlag, "{root}")}},
	{"wave watch", []contractCall{okCall(contractWaveRecorded, "wave", "watch", prFlag, "{root}"), usageCall(contractPlane, "wave", "watch", "--bogus", prFlag, "{root}"), failCall(cliExitIO, contractPlane, "wave", "watch", prFlag, "{root}")}},
	{"inbox list", []contractCall{okCall(contractInboxItem, "inbox", "list", prFlag, "{root}"), usageCall(contractPlane, "inbox", "list", "--status", "x", prFlag, "{root}"), failCall(cliExitIO, contractInboxFile, "inbox", "list", prFlag, "{root}")}},
	{"inbox show", []contractCall{okCall(contractInboxItem, "inbox", "show", "cli-inbox-show", prFlag, "{root}"), usageCall(contractPlane, "inbox", "show", prFlag, "{root}"), failCall(cliExitFailed, contractPlane, "inbox", "show", "absent", prFlag, "{root}")}},
	{"inbox rank", []contractCall{okCall(contractInboxItem, "inbox", "rank", prFlag, "{root}"), usageCall(contractPlane, "inbox", "rank", "--top", "0", prFlag, "{root}"), failCall(cliExitIO, contractBadPolicy, "inbox", "rank", prFlag, "{root}")}},
	{"inbox lint", []contractCall{okCall(contractInboxItem, "inbox", "lint", prFlag, "{root}"), usageCall(contractPlane, "inbox", "lint", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractUndeclaredItem, "inbox", "lint", prFlag, "{root}")}},
	{"inbox batches", []contractCall{okCall(contractInboxItem, "inbox", "batches", prFlag, "{root}"), usageCall(contractPlane, "inbox", "batches", "--max", "x", prFlag, "{root}"), failCall(cliExitFailed, contractInboxFile, "inbox", "batches", prFlag, "{root}")}},
	{"signals tail", []contractCall{okCall(contractPlane, "signals", "tail", prFlag, "{root}"), usageCall(contractPlane, "signals", "tail", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractPlane, "signals", "tail", "--cycle", "5", prFlag, "{root}")}},
	{"ledger verify", []contractCall{okCall(contractPlane, "ledger", "verify", prFlag, "{root}"), usageCall(contractPlane, "ledger", "verify", "--bogus", prFlag, "{root}"), failCall(cliExitIO, contractBrokenLedger, "ledger", "verify", prFlag, "{root}")}},
	{"dossier verify", []contractCall{okCall(contractPlane, "dossier", "verify", prFlag, "{root}"), usageCall(contractPlane, "dossier", "verify", "extra", prFlag, "{root}"), failCall(cliExitFailed, contractBadDossier, "dossier", "verify", prFlag, "{root}")}},
	{"cli-routing show", []contractCall{okCall(contractRouting, "cli-routing", "show", "--static", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", "show", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractBadPolicy, "cli-routing", "show", "--static", prFlag, "{root}")}},
	{"cli-routing check", []contractCall{okCall(contractRouting, "cli-routing", "check", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", "check", "extra", prFlag, "{root}"), failCall(cliExitFailed, contractBadPolicy, "cli-routing", "check", prFlag, "{root}")}},
	{"cli-routing explain", []contractCall{okCall(contractRouting, "cli-routing", "explain", "auditor", "--static", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", "explain", prFlag, "{root}"), failCall(cliExitFailed, contractBadPolicy, "cli-routing", "explain", "auditor", "--static", prFlag, "{root}")}},
	{"routing explain", []contractCall{okCall(contractPlane, "routing", "explain", "--cycle", "1", prFlag, "{root}"), usageCall(contractPlane, "routing", "explain", prFlag, "{root}"), failCall(cliExitFailed, contractUnreadableCwd, "routing", "explain", "--cycle", "1")}},
	{"clihealth list", []contractCall{okCall(contractPlane, "clihealth", "list", prFlag, "{root}"), usageCall(contractPlane, "clihealth", "list", "extra", prFlag, "{root}")}},
	{"clihealth usage", []contractCall{okCall(contractUsagePanes, "clihealth", "usage", "claude", prFlag, "{root}"), usageCall(contractPlane, "clihealth", "usage", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractUsagePanes, "clihealth", "usage", "agy", prFlag, "{root}")}},
	{"agent list", []contractCall{okCall(contractAgentRoot, "agent", "list", prFlag, "{root}"), usageCall(contractAgentRoot, "agent", "list", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractBareRoot, "agent", "list", prFlag, "{root}")}},
	{"ci classify", []contractCall{okCall(contractCIRoot, "ci", "classify", "run:501", prFlag, "{root}"), usageCall(contractPlane, "ci", "classify", prFlag, "{root}"), failCall(cliExitIO, contractNoGH, "ci", "classify", "run:501", prFlag, "{root}")}},
	{"scan secrets", []contractCall{okCall(contractGitRoot, "scan", "secrets", "--staged", prFlag, "{root}"), usageCall(contractGitRoot, "scan", "secrets", "--staged", "--diff", "HEAD", prFlag, "{root}"), failCall(cliExitIO, contractBareRoot, "scan", "secrets", "--staged", prFlag, "{root}")}},
	{"docs ste-lint", []contractCall{okCall(contractSteRoot, "docs", "ste-lint", "note.md", prFlag, "{root}"), usageCall(contractSteRoot, "docs", "ste-lint", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractSteRoot, "docs", "ste-lint", "--strict", "bad.md", prFlag, "{root}")}},
	{"quality report", []contractCall{okCall(contractPlane, "quality", "report", "--scope", "wave", prFlag, "{root}"), usageCall(contractPlane, "quality", "report", prFlag, "{root}")}},
	{"quality compare", []contractCall{okCall(contractWaveRecorded, "quality", "compare", "--index", "PQ1", "--a", "wave:1", "--b", "wave:2", prFlag, "{root}"), usageCall(contractPlane, "quality", "compare", "--index", "PQ9", prFlag, "{root}"), failCall(cliExitIO, contractPlane, "quality", "compare", "--index", "PQ1", "--a", "wave:1", "--b", "wave:2", prFlag, "{root}")}},
	{"agent run", []contractCall{custom(okCall(contractAgentRoot, "agent", "run", "adversarial-review", prFlag, "{root}"), contractAgentRun), usageCall(contractAgentRoot, "agent", "run", "adversarial-review", "--bogus", prFlag, "{root}"), unheld(failCall(cliExitRefused, contractAgentRoot, "agent", "run", "adversarial-review", prFlag, "{root}"))}},
	{"inbox add", []contractCall{withStdin(okCall(contractPlane, "inbox", "add", "--file", "-", prFlag, "{root}"), cliInboxItem), usageCall(contractPlane, "inbox", "add", prFlag, "{root}"), withStdin(failCall(cliExitFailed, contractPlane, "inbox", "add", "--file", "-", prFlag, "{root}"), `{"id":"x"}`)}},
	{"inbox consume", []contractCall{okCall(contractInboxItem, "inbox", "consume", "{item}", prFlag, "{root}"), usageCall(contractPlane, "inbox", "consume", prFlag, "{root}"), failCall(cliExitFailed, contractPlane, "inbox", "consume", "{root}/absent.json", prFlag, "{root}")}},
	{"inbox edit", []contractCall{okCall(contractInboxItem, "inbox", "edit", "cli-inbox-show", "--set", "weight=0.4", prFlag, "{root}"), usageCall(contractInboxItem, "inbox", "edit", "cli-inbox-show", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedItem, "inbox", "edit", "cli-inbox-show", "--set", "weight=0.4", prFlag, "{root}")}},
	{"inbox withdraw", []contractCall{okCall(contractInboxItem, "inbox", "withdraw", "cli-inbox-show", "mistake", prFlag, "{root}"), usageCall(contractInboxItem, "inbox", "withdraw", "cli-inbox-show", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedItem, "inbox", "withdraw", "cli-inbox-show", "mistake", prFlag, "{root}")}},
	{"inbox verify", []contractCall{okCall(contractOriginItem, "inbox", "verify", "stale", "--evidence", "checked", prFlag, "{root}"), usageCall(contractInboxItem, "inbox", "verify", "cli-inbox-show", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedItem, "inbox", "verify", "cli-inbox-show", "--evidence", "checked", prFlag, "{root}")}},
	{"inbox route-console", []contractCall{okCall(contractInboxItem, "inbox", "route-console", "cli-inbox-show", "operator owns it", "0", prFlag, "{root}"), usageCall(contractInboxItem, "inbox", "route-console", "cli-inbox-show", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedItem, "inbox", "route-console", "cli-inbox-show", "operator owns it", "0", prFlag, "{root}")}},
	{"inbox route-lane", []contractCall{okCall(contractConsoleItem, "inbox", "route-lane", "cli-inbox-show", "a lane may take it", prFlag, "{root}"), usageCall(contractInboxItem, "inbox", "route-lane", "cli-inbox-show", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedItem, "inbox", "route-lane", "cli-inbox-show", "a lane may take it", prFlag, "{root}")}},
	{"cli-routing set", []contractCall{okCall(contractRouting, "cli-routing", "set", "default", "claude", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", "set", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedRouting, "cli-routing", "set", "default", "claude", prFlag, "{root}")}},
	{"cli-routing unset", []contractCall{okCall(contractRoutedAgent, "cli-routing", "unset", "agents.auditor", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", "unset", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedRouting, "cli-routing", "unset", "default", prFlag, "{root}")}},
	{"cli-routing init", []contractCall{okCall(contractNoRoutingTable, "cli-routing", routingInitVerb, "--clis", "claude", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", routingInitVerb, prFlag, "{root}"), failCall(cliExitRefused, contractLeasedRouting, "cli-routing", routingInitVerb, "--clis", "claude", prFlag, "{root}")}},
	{"cli-routing migrate", []contractCall{okCall(contractRouting, "cli-routing", "migrate", "--dry-run", prFlag, "{root}"), usageCall(contractRouting, "cli-routing", "migrate", "extra", prFlag, "{root}"), failCall(cliExitRefused, contractLeasedRouting, "cli-routing", "migrate", prFlag, "{root}")}},
	{"wave note", []contractCall{okCall(contractPlane, "wave", "note", "add", "watch PR 814", prFlag, "{root}"), usageCall(contractPlane, "wave", "note", prFlag, "{root}"), failCall(cliExitIO, contractEvolveFile, "wave", "note", "list", prFlag, "{root}")}},
	{"checkpoint save", []contractCall{okCall(contractCheckpointHub, "checkpoint", "save", "--all", prFlag, "{root}"), usageCall(contractCheckpointHub, "checkpoint", "save", "--bogus", prFlag, "{root}"), failCall(cliExitFailed, contractBareRoot, "checkpoint", "save", prFlag, "{root}")}},
	{"checkpoint list", []contractCall{okCall(contractCheckpointHub, "checkpoint", "list", prFlag, "{root}"), usageCall(contractCheckpointHub, "checkpoint", "list", "extra", prFlag, "{root}"), failCall(cliExitFailed, contractBareRoot, "checkpoint", "list", prFlag, "{root}")}},
	{"checkpoint restore", []contractCall{okCall(contractCheckpointHub, "checkpoint", "restore", "{ref}", "--into", "{into}", prFlag, "{root}"), usageCall(contractCheckpointHub, "checkpoint", "restore", prFlag, "{root}"), failCall(cliExitFailed, contractCheckpointHub, "checkpoint", "restore", "absent/ref", "--into", "{into}", prFlag, "{root}")}},
	{"checkpoint prune", []contractCall{okCall(contractCheckpointHub, "checkpoint", "prune", prFlag, "{root}"), usageCall(contractCheckpointHub, "checkpoint", "prune", "extra", prFlag, "{root}"), failCall(cliExitFailed, contractBareRoot, "checkpoint", "prune", prFlag, "{root}")}},
	{"gc", []contractCall{okCall(contractGoCache, "gc", "--dry-run", "--only", "cache", prFlag, "{root}"), usageCall(contractPlane, "gc", "--only", "bogus", prFlag, "{root}"), failCall(cliExitFailed, contractPlane, "gc")}},
	{"loop-stop", []contractCall{okCall(contractPlane, "loop-stop", "--release", prFlag, "{root}"), usageCall(contractPlane, "loop-stop", "--wait", "--release", prFlag, "{root}"), failCall(cliExitIO, contractBareRoot, "loop-stop", prFlag, "{root}")}},
	{"wave next", []contractCall{okCall(contractPlane, "wave", "next", "--dry-run", prFlag, "{root}"), usageCall(contractPlane, "wave", "next", "--max-cycles", "0", prFlag, "{root}"), unheld(failCall(cliExitRefused, contractPlane, "wave", "next", "--dry-run", prFlag, "{root}"))}},
	{"boundary run", []contractCall{okCall(contractPlane, "boundary", "run", "--dry-run", "--goal-text-file", "{goal}", prFlag, "{root}"), usageCall(contractPlane, "boundary", "run", "--dry-run", prFlag, "{root}"), unheld(failCall(cliExitRefused, contractPlane, "boundary", "run", "--dry-run", "--goal-text-file", "{goal}", prFlag, "{root}"))}},
	{"pr merge", []contractCall{custom(okCall(contractPlane, "pr", "merge", "12", prFlag, "{root}"), contractPRMerge), usageCall(contractPlane, "pr", "merge", prFlag, "{root}"), failCall(cliExitIO, contractBareRoot, "pr", "merge", "12", prFlag, "{root}")}},
	{"sync-main", []contractCall{okCall(contractSyncRepo, "sync-main", prFlag, "{root}"), usageCall(contractSyncRepo, "sync-main", "extra", prFlag, "{root}"), failCall(cliExitRefused, contractDirtySyncRepo, "sync-main", prFlag, "{root}")}},
	{"cycle reset", []contractCall{okCall(contractResetRoot, "cycle", "reset", "--dry-run", prFlag, "{root}"), usageCall(contractResetRoot, "cycle", "reset", "--bogus", prFlag, "{root}"), failCall(cliExitRefused, contractLiveResetRoot, "cycle", "reset", prFlag, "{root}")}},
	{"cli update", []contractCall{okCall(contractCLIUpdateHost, "cli", "update", "--dry-run", prFlag, "{root}"), usageCall(contractCLIUpdateHost, "cli", "update", "extra", prFlag, "{root}"), failCall(cliExitRefused, contractLiveCLIUpdateHost, "cli", "update", prFlag, "{root}")}},
	{"loop", []contractCall{okCall(contractPlane, "loop", "--dry-run", "--goal-text", "fix the parser", prFlag, "{root}"), usageCall(contractPlane, "loop", "stop", "now", prFlag, "{root}"), unheld(failCall(cliExitRefused, contractPlane, "loop", "--dry-run", "--goal-text", "fix the parser", prFlag, "{root}"))}},
}

func contractBadWaves(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractWrite(t, vars["root"], ".evolve/waves/wave-1.json", "{bad")
	return vars
}

func contractInboxFile(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	contractWrite(t, root, ".evolve/inbox", "not a directory\n")
	return map[string]string{"root": root}
}

func contractUndeclaredItem(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractWrite(t, vars["root"], ".evolve/inbox/2026-10-27T00-00-00Z-no-surface.json", `{"id":"no-surface","kind":"feature","priority_class":"debuggability","weight":0.5,`+
		`"title":"an item with no files","summary":"s","fix":"f","acceptance":["a"]}`)
	return vars
}

func contractBrokenLedger(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractWrite(t, vars["root"], ".evolve/ledger.jsonl", "{not a ledger line\n")
	return vars
}

func contractBadDossier(t *testing.T) map[string]string {
	t.Helper()
	vars := contractPlane(t)
	contractWrite(t, vars["root"], "knowledge-base/cycles/cycle-1.json", "{bad")
	return vars
}

func contractEvolveFile(t *testing.T) map[string]string {
	t.Helper()
	root := t.TempDir()
	contractWrite(t, root, ".evolve", "not a directory\n")
	return map[string]string{"root": root}
}

func contractUnreadableCwd(t *testing.T) map[string]string {
	t.Helper()
	prev := routingGetwd
	routingGetwd = func() (string, error) { return "", errors.New("getwd: gone") }
	t.Cleanup(func() { routingGetwd = prev })
	return map[string]string{}
}

func contractNoGH(t *testing.T) map[string]string {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	return contractPlane(t)
}

func contractOriginItem(t *testing.T) map[string]string {
	t.Helper()
	root, _ := originMainRoot(t)
	return map[string]string{"root": root}
}

func contractConsoleItem(t *testing.T) map[string]string {
	t.Helper()
	vars := contractInboxItem(t)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"route-console", "cli-inbox-show", "operator owns it", "0", prFlag, vars["root"]}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("route the item to the console: %s", stderr.String())
	}
	return vars
}

func contractLeasedRouting(t *testing.T) map[string]string {
	t.Helper()
	vars := contractRouting(t)
	contractLiveLease(t, vars["root"])
	return vars
}

func contractToolPATH(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"git", "go", "cat"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("%s is not on PATH: %v", tool, err)
		}
		if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

func contractArgs(args []string, vars map[string]string) []string {
	out := make([]string, 0, len(args)+1)
	for _, a := range args {
		for k, v := range vars {
			a = strings.ReplaceAll(a, "{"+k+"}", v)
		}
		out = append(out, a)
	}
	return append(out, "--json")
}

func contractTreeHash(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		h.Write([]byte(rel + "\x00" + d.Type().String() + "\x00"))
		if d.Type().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h.Write(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func runContractCall(t *testing.T, v coreVerb, c contractCall) {
	t.Helper()
	vars := map[string]string{}
	if c.prep != nil {
		vars = c.prep(t)
	}
	args := contractArgs(c.args, vars)
	before := ""
	if root := vars["root"]; root != "" && v.class == availAlways && c.kind == contractSuccess {
		before = contractTreeHash(t, root)
	}
	var stdout, stderr bytes.Buffer
	var rc int
	switch {
	case c.run != nil:
		rc = runEnveloped(v, func(o, e io.Writer) int { return c.run(t, vars)(args, o, e) }, &stdout, &stderr)
	case c.unheld:
		rc = dispatchWith(dispatchGate{open: unheldStore(t, "enforce")}, args, strings.NewReader(c.stdin), &stdout, &stderr)
	default:
		rc = dispatchWith(dispatchGate{open: heldStore(t)}, args, strings.NewReader(c.stdin), &stdout, &stderr)
	}
	checkContractEnvelope(t, v, c, rc, stdout.String(), stderr.String())
	if before != "" && contractTreeHash(t, vars["root"]) != before {
		t.Errorf("%s is an always verb, but %v changed the tree of %s", v.path, args, vars["root"])
	}
}

func checkContractEnvelope(t *testing.T, v coreVerb, c contractCall, rc int, stdout, stderr string) {
	t.Helper()
	if rc != c.want {
		t.Fatalf("%s %s: exit %d, want %d\nstdout: %s\nstderr: %s", v.path, c.kind, rc, c.want, stdout, stderr)
	}
	kind, known := v.exitKind(rc)
	if rc != cliExitOK && !known {
		t.Errorf("%s: exit %d is not in the exit table of cli-contract.md §3", v.path, rc)
	}
	envs := decodeEnvelopes(t, stdout)
	if len(envs) == 0 || (!v.stream && len(envs) != 1) {
		t.Fatalf("%s %s: %d envelopes, want one\n%s", v.path, c.kind, len(envs), stdout)
	}
	for _, e := range envs {
		if e.Schema != v.schema() {
			t.Errorf("%s: schema %q, want %q", v.path, e.Schema, v.schema())
		}
	}
	last := envs[len(envs)-1]
	if last.OK != (rc == cliExitOK) || (rc == cliExitOK) != (last.Error == nil) {
		t.Errorf("%s %s: ok %t error %+v for exit %d", v.path, c.kind, last.OK, last.Error, rc)
	}
	if last.Error != nil && (last.Error.Code != rc || last.Error.Kind != kind || last.Error.Message == "") {
		t.Errorf("%s %s: error %+v, want code %d kind %q and a message", v.path, c.kind, last.Error, rc, kind)
	}
}

func contractEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", contractToolPATH(t))
	t.Setenv("GOCACHE", t.TempDir())
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
}

func TestCLIContract_EachCoreVerbPrintsOneEnvelope(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	contractEnvironment(t)
	seen := map[string]map[string]bool{}
	for _, vc := range contractCases {
		v, ok := lookupCoreVerb(vc.verb)
		if !ok {
			t.Fatalf("%s: not a core verb", vc.verb)
		}
		seen[vc.verb] = map[string]bool{}
		for _, c := range vc.calls {
			seen[vc.verb][c.kind] = true
			t.Run(vc.verb+"/"+c.kind, func(t *testing.T) { runContractCall(t, v, c) })
		}
	}
	for _, v := range coreVerbs {
		kinds := seen[v.path]
		_, noNegative := contractNoNegative[v.path]
		if !kinds[contractSuccess] || !kinds[contractUsage] || kinds[contractNegative] == noNegative {
			t.Errorf("%s: cases %v; want a success, a usage and a negative case (or a reason in contractNoNegative)", v.path, kinds)
		}
	}
}

func TestCLIContract_AlwaysVerbsWriteNothing(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	contractEnvironment(t)
	for _, vc := range contractCases {
		v, _ := lookupCoreVerb(vc.verb)
		if v.class != availAlways {
			continue
		}
		for _, c := range vc.calls {
			if c.kind == contractSuccess {
				t.Run(vc.verb, func(t *testing.T) { runContractCall(t, v, c) })
			}
		}
	}
}

func contractRoutedAgent(t *testing.T) map[string]string {
	t.Helper()
	vars := contractRouting(t)
	var stdout, stderr bytes.Buffer
	if rc := runCLIRouting([]string{"set", "agents.auditor", "claude", prFlag, vars["root"]}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("set agents.auditor: %s", stderr.String())
	}
	return vars
}

func contractNoRoutingTable(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"root": routingWriteProject(t, `{}`)}
}
