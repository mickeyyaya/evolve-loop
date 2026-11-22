package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	stageVerbKey    = "inbox.curator.stage"
	stageVerbPolicy = `{"inbox":{"curator":{"stage":"shadow","limit":3}},"other":{"knob":1}}`
)

func stageProjectRoot(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range map[string]string{
		".evolve/policy.json": policy,
		"go/go.mod":           "module fixture\n\ngo 1.23\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runStageVerb(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = runStage(args, nil, &out, &errOut)
	return out.String(), errOut.String(), code
}

func stageVersion(t *testing.T, root string) string {
	t.Helper()
	out, errOut, code := runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root)
	if code != cliExitFailed {
		t.Fatalf("verify without a chain exited %d, want %d; stderr: %s", code, cliExitFailed, errOut)
	}
	var res stageevidence.Result
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Version == "" {
		t.Fatalf("verify without a chain printed no decision version: %q (%v)", out, err)
	}
	return res.Version
}

func stageChainFile(t *testing.T, version string, fields ...map[string]string) string {
	t.Helper()
	chain := stageevidence.Chain{}
	for i, f := range fields {
		rec := stageevidence.Link("", stageevidence.Record{Key: stageVerbKey, Version: version, Kind: stageevidence.KindDecision, ID: i + 1, Seq: i + 1, Fields: f})
		chain.Records = append(chain.Records, rec)
		chain.Outcomes = append(chain.Outcomes, stageevidence.Outcome{ID: i + 1, Shipped: true})
	}
	return writeStageChain(t, chain)
}

func writeStageChain(t *testing.T, chain stageevidence.Chain) string {
	t.Helper()
	raw, err := json.MarshalIndent(chain, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "stage-chain.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeResult(t *testing.T, out string) stageevidence.Result {
	t.Helper()
	var res stageevidence.Result
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("verify --json printed %q: %v", out, err)
	}
	return res
}

func TestEveryStageHasScorerAndVerdict(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runStageVerb(t, "report", "--json", "--project-root", repo)
	if code != cliExitOK {
		t.Fatalf("stage report exited %d; stderr: %s", code, errOut)
	}
	var results []stageevidence.Result
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("stage report --json printed %q: %v", out, err)
	}
	var want []string
	for _, e := range gateregistry.StageEntries() {
		want = append(want, e.Shadow.PolicyKey)
		if scorer := stageevidence.ScorerFor(e.Shadow.PolicyKey); scorer == nil || scorer.Name() == "" {
			t.Errorf("stage key %s has no scorer", e.Shadow.PolicyKey)
		}
	}
	var got []string
	for _, r := range results {
		got = append(got, r.Key)
		if r.Scorer == "" || r.Version == "" {
			t.Errorf("%s: report line has scorer %q and version %q, want both set", r.Key, r.Scorer, r.Version)
		}
		if r.Ready || r.Error != "empty-chain" {
			t.Errorf("%s: ready %v error %q, want not ready with empty-chain while no evidence exists", r.Key, r.Ready, r.Error)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("stage report lists %d keys, the registry has %d; got %v want %v", len(got), len(want), got, want)
	}
}

func TestStageVerifyRecomputesTheVerdictFromTheChainFile(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	version := stageVersion(t, root)

	clean := stageChainFile(t, version, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "false"})
	out, errOut, code := runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", clean)
	res := decodeResult(t, out)
	if code != cliExitOK || !res.Ready || res.Decisions != 2 || res.Error != "" || res.Version != version {
		t.Errorf("clean chain: exit %d ready %v decisions %d error %q; stderr %s", code, res.Ready, res.Decisions, res.Error, errOut)
	}

	faulty := stageChainFile(t, version, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "true"})
	out, _, code = runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", faulty)
	res = decodeResult(t, out)
	wantCase := stageevidence.Case{Class: stageevidence.ClassFalseStop, Cycle: 2, Seq: 2}
	if code != cliExitFailed || res.Ready || res.FalseStops != 1 || !slices.Contains(res.Cases, wantCase) {
		t.Errorf("chain with a false stop: exit %d ready %v false stops %d cases %+v", code, res.Ready, res.FalseStops, res.Cases)
	}
}

func TestStageVerifyPrintsTheRangeTheDigestTheCountsAndEachCase(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	version := stageVersion(t, root)
	chain := stageChainFile(t, version, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "true"})
	jsonOut, _, _ := runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", chain)
	digest := decodeResult(t, jsonOut).ChainDigest
	if digest == "" {
		t.Fatal("the verified chain has no digest")
	}
	out, _, code := runStageVerb(t, "verify", stageVerbKey, "--project-root", root, "--chain", chain)
	if code != cliExitFailed {
		t.Errorf("text verify exited %d, want %d", code, cliExitFailed)
	}
	for _, line := range []string{
		"not ready",
		"decision version: " + version,
		"evidence range: cycle 1 to cycle 2",
		"chain digest: " + digest,
		"decisions: 2",
		"false stops: 1",
		"escapes: 0",
		"conflicts: 0",
		"incomplete cycles: 0",
		"false_stop cycle 2 seq 2",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("verify output lacks %q:\n%s", line, out)
		}
	}
}

func TestStageVerifyBrokenChainIsNotReadyWithATypedError(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	version := stageVersion(t, root)
	path := stageChainFile(t, version, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "false"})
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var chain stageevidence.Chain
	if err := json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	chain.Records[1].Fields = map[string]string{"would_stop": "true"}
	tampered := writeStageChain(t, chain)

	out, _, code := runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", tampered)
	res := decodeResult(t, out)
	if code != cliExitFailed || res.Ready || res.Error != "broken-chain" || res.Decisions != 0 {
		t.Errorf("tampered chain: exit %d ready %v error %q decisions %d, want not ready, broken-chain, no partial verdict", code, res.Ready, res.Error, res.Decisions)
	}
	text, _, _ := runStageVerb(t, "verify", stageVerbKey, "--project-root", root, "--chain", tampered)
	if !strings.Contains(text, "not ready") || !strings.Contains(text, "broken-chain") || !strings.Contains(text, "cycle 2") || !strings.Contains(text, "seq 2") {
		t.Errorf("text output does not name the layer and the first failing record:\n%s", text)
	}
}

func TestStageVerifyDoesNotCountEvidenceOfAnotherVersion(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	stale := stageChainFile(t, "version-from-an-older-decider", map[string]string{"would_stop": "false"}, map[string]string{"would_stop": "false"})
	out, _, code := runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", stale)
	res := decodeResult(t, out)
	if code != cliExitFailed || res.Ready || res.Decisions != 0 || res.OtherVersions["version-from-an-older-decider"] != 2 {
		t.Errorf("stale evidence: exit %d ready %v decisions %d other versions %v, want not ready, 0 counted, 2 reported", code, res.Ready, res.Decisions, res.OtherVersions)
	}
}

func TestStageVerifyKeepsEvidenceAcrossAnUnrelatedPolicyEditAndDropsItOnASubtreeEdit(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	version := stageVersion(t, root)
	chain := stageChainFile(t, version, map[string]string{"would_stop": "false"})
	policyPath := filepath.Join(root, ".evolve", "policy.json")

	if err := os.WriteFile(policyPath, []byte(`{"inbox":{"curator":{"stage":"shadow","limit":3}},"other":{"knob":42}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, _ := runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", chain)
	if res := decodeResult(t, out); res.Decisions != 1 || res.Version != version {
		t.Errorf("after an unrelated policy edit: decisions %d version %s, want 1 and %s", res.Decisions, res.Version, version)
	}

	if err := os.WriteFile(policyPath, []byte(`{"inbox":{"curator":{"stage":"shadow","limit":4}},"other":{"knob":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, _ = runStageVerb(t, "verify", stageVerbKey, "--json", "--project-root", root, "--chain", chain)
	if res := decodeResult(t, out); res.Decisions != 0 || res.Version == version || res.OtherVersions[version] != 1 {
		t.Errorf("after a subtree edit: decisions %d version %s other versions %v, want 0, a new version and the old one reported", res.Decisions, res.Version, res.OtherVersions)
	}
}

func TestStageVerbRefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no subcommand", nil, cliExitUsage},
		{"unknown subcommand", []string{"flip"}, cliExitUsage},
		{"verify without a key", []string{"verify", "--project-root", root}, cliExitUsage},
		{"verify of a key outside the registry", []string{"verify", "not.a.stage.key", "--project-root", root}, cliExitUsage},
		{"verify of an unreadable chain file", []string{"verify", stageVerbKey, "--project-root", root, "--chain", filepath.Join(root, "missing.json")}, cliExitIO},
	}
	for _, c := range cases {
		if _, errOut, code := runStageVerb(t, c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d; stderr: %s", c.name, code, c.want, errOut)
		}
	}
}

func TestStageReportPrintsOneLinePerStageKey(t *testing.T) {
	t.Parallel()
	root := stageProjectRoot(t, stageVerbPolicy)
	out, _, code := runStageVerb(t, "report", "--project-root", root)
	if code != cliExitOK {
		t.Fatalf("stage report exited %d", code)
	}
	for _, e := range gateregistry.StageEntries() {
		lines := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, e.Shadow.PolicyKey+" ") || strings.HasPrefix(line, e.Shadow.PolicyKey+"\t") {
				lines++
			}
		}
		if lines != 1 {
			t.Errorf("stage report has %d lines for %s, want 1", lines, e.Shadow.PolicyKey)
		}
	}
}
