package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCommandEfforts_AnUnreadableWorkingDirectoryFallsBackToTheCompiledDefault(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stderr bytes.Buffer

	got := commandEffortsFrom(func() (string, error) { return "", errors.New("getcwd: no such file or directory") }, &stderr)

	if !reflect.DeepEqual(got, policy.EffortTable{}) || !strings.Contains(stderr.String(), "evolve bridge: WARN cannot read the working directory") {
		t.Errorf("commandEfforts = %+v, stderr=%q, want the empty table and a working-directory WARN", got, stderr.String())
	}
}

func TestCommandEfforts_AnUnparseablePolicyFallsBackToTheCompiledDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stderr bytes.Buffer

	got := commandEfforts(&stderr)

	if !reflect.DeepEqual(got, policy.EffortTable{}) || !strings.Contains(stderr.String(), "the effort is the compiled default") {
		t.Errorf("commandEfforts = %+v, stderr=%q, want the empty table and a policy WARN", got, stderr.String())
	}
}

func TestSetRoutingKey_RefusesAnEditWithNothingToSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		set  routingSet
		want string
	}{
		{routingSet{key: "tiers.deep"}, "tiers.deep needs a value"},
		{routingSet{key: "agents.auditor", model: "deep", effort: "high"}, "--model needs the chain of agents.auditor"},
	}
	for _, tc := range cases {
		block := policy.CLIRouting{Agents: map[string]policy.AgentRule{"auditor": {Effort: "low"}}}
		got, err := setRoutingKey(block, tc.set)
		if err == nil || err.Error() != tc.want || got.Agents["auditor"].Effort != "low" {
			t.Errorf("setRoutingKey(%+v) = %+v, %v, want the error %q and the block unchanged", tc.set, got, err, tc.want)
		}
	}
}

func TestUnsetRoutingKey_ClearsEachTopLevelKeyAndEachScopedEntry(t *testing.T) {
	t.Parallel()
	full := func() policy.CLIRouting {
		return policy.CLIRouting{
			CLIs: []string{"claude"}, Default: []string{"claude"}, AfterChain: "stop",
			Work:   map[string][]string{"build": {"claude"}},
			Agents: map[string]policy.AgentRule{"auditor": {CLI: []string{"claude"}}},
		}
	}
	cases := map[string]func(policy.CLIRouting) bool{
		"clis":           func(b policy.CLIRouting) bool { return b.CLIs == nil && b.Default != nil },
		"default":        func(b policy.CLIRouting) bool { return b.Default == nil && b.CLIs != nil },
		"after_chain":    func(b policy.CLIRouting) bool { return b.AfterChain == "" },
		"work.build":     func(b policy.CLIRouting) bool { _, ok := b.Work["build"]; return !ok },
		"agents.auditor": func(b policy.CLIRouting) bool { _, ok := b.Agents["auditor"]; return !ok },
	}
	for key, ok := range cases {
		got, err := unsetRoutingKey(full(), key)
		if err != nil || !ok(got) {
			t.Errorf("unsetRoutingKey(%q) = %+v, %v, want only that key cleared", key, got, err)
		}
	}
	if _, err := unsetRoutingKey(full(), "bogus"); err == nil || err.Error() != `unknown key "bogus"` {
		t.Errorf("unsetRoutingKey(bogus) error = %v, want unknown key", err)
	}
}

func TestUnsetEffort_ClearsOnlyTheEffortAndDropsAnEntryLeftEmpty(t *testing.T) {
	t.Parallel()
	block := policy.CLIRouting{Agents: map[string]policy.AgentRule{
		"auditor": {CLI: []string{"claude"}, Model: "deep", Effort: "high"},
		"router":  {Effort: "low"},
	}}

	got, err := unsetRoutingKey(block, "agents.auditor.effort")
	if err != nil || !reflect.DeepEqual(got.Agents["auditor"], policy.AgentRule{CLI: []string{"claude"}, Model: "deep"}) {
		t.Fatalf("unset agents.auditor.effort = %+v, %v, want the chain and model kept", got.Agents, err)
	}
	got, err = unsetRoutingKey(got, "agents.router.effort")
	if _, kept := got.Agents["router"]; err != nil || kept {
		t.Errorf("unset agents.router.effort = %+v, %v, want the effort-only entry removed", got.Agents, err)
	}
}

func TestUnsetEffort_RefusesAnEntryThatIsNotSetAndAGroupWithoutAnEffort(t *testing.T) {
	t.Parallel()
	for key, want := range map[string]string{
		"tiers.deep.effort":    "tiers.deep is not set",
		"agents.nobody.effort": "agents.nobody is not set",
		"work.build.effort":    "an effort is set only on tiers.<tier> and agents.<agent>, not work",
	} {
		if _, err := unsetRoutingKey(policy.CLIRouting{}, key); err == nil || err.Error() != want {
			t.Errorf("unsetRoutingKey(%q) error = %v, want %q", key, err, want)
		}
	}
}

func TestRemoveTopLevelKey_KeepsTheRestOfTheObjectValid(t *testing.T) {
	t.Parallel()
	cases := []struct{ raw, key, want string }{
		{`{"effort_level": "low"}`, "effort_level", `{}`},
		{`{"effort_level": "low", "name": "a", "cli": "b"}`, "effort_level", `{ "name": "a", "cli": "b"}`},
		{`{"name": "a", "effort_level": "low"}`, "effort_level", `{"name": "a"}`},
		{`{"name": "a"}`, "effort_level", `{"name": "a"}`},
	}
	for _, tc := range cases {
		got, err := removeTopLevelKey([]byte(tc.raw), tc.key)
		if err != nil || string(got) != tc.want {
			t.Errorf("removeTopLevelKey(%s) = %s, %v, want %s", tc.raw, got, err, tc.want)
		}
	}
}

func TestRemoveTopLevelKeys_RefusesWhatIsNotAJSONObject(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		`["a"]`:   "want a JSON object",
		``:        "want a JSON object",
		`{1: 2}`:  "",
		`{"a": }`: "",
	} {
		got, err := removeTopLevelKeys([]byte(raw), retiredEffortKeys...)
		if err == nil || !strings.Contains(err.Error(), want) || got != nil {
			t.Errorf("removeTopLevelKeys(%q) = %q, %v, want no output and an error containing %q", raw, got, err, want)
		}
	}
}

func TestWithoutAgentEffort_ClearsTheEffortOnACopy(t *testing.T) {
	t.Parallel()
	block := policy.CLIRouting{Agents: map[string]policy.AgentRule{"auditor": {Model: "deep", Effort: "high"}}}

	got := withoutAgentEffort(block, "auditor")

	if !reflect.DeepEqual(got.Agents["auditor"], policy.AgentRule{Model: "deep"}) || block.Agents["auditor"].Effort != "high" {
		t.Errorf("withoutAgentEffort = %+v (input now %+v), want the effort cleared on a copy only", got.Agents, block.Agents)
	}
}

func TestMoveOneProfileEffort_RefusesAProfileThatDisagreesWithTheTable(t *testing.T) {
	t.Parallel()
	level := "low"
	block := policy.CLIRouting{Agents: map[string]policy.AgentRule{"auditor": {Effort: "high"}}}

	got, err := moveOneProfileEffort(block, profileEffort{Name: "auditor", EffortLevel: &level}, "auditor.json")

	want := `auditor.json: effort_level "low" and cli_routing.agents.auditor.effort "high" differ: keep one`
	if err == nil || err.Error() != want || got.Agents["auditor"].Effort != "high" {
		t.Errorf("moveOneProfileEffort = %+v, %v, want %q and the table effort kept", got.Agents, err, want)
	}
}

func TestMoveProfileEfforts_AnUnreadableProfileDirIsAnError(t *testing.T) {
	t.Parallel()
	bad := filepath.Join(t.TempDir(), "a[")
	if _, err := moveProfileEfforts(bad, policy.CLIRouting{}, map[string][]byte{}); !errors.Is(err, filepath.ErrBadPattern) {
		t.Errorf("moveProfileEfforts(%q) error = %v, want %v", bad, err, filepath.ErrBadPattern)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "x.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	rewrites := map[string][]byte{}
	if _, err := moveProfileEfforts(dir, policy.CLIRouting{}, rewrites); err == nil || len(rewrites) != 0 {
		t.Errorf("moveProfileEfforts over a directory named x.json = %v (rewrites %v), want a read error and no rewrite", err, rewrites)
	}
}

func TestWriteProfileRewrites_AnUnwritableProfileIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeProfileRewrites(dir, map[string][]byte{"plain/x.json": []byte("{}")})

	if err == nil || !strings.Contains(err.Error(), "atomicwrite: mkdir") {
		t.Errorf("writeProfileRewrites = %v, want an atomicwrite mkdir error", err)
	}
}

func TestProfileOverlay_ServesARewriteOverTheBaseAndTheBaseOtherwise(t *testing.T) {
	t.Parallel()
	o := profileOverlay{
		base:  fstest.MapFS{"a.json": {Data: []byte("base-a")}, "b.json": {Data: []byte("base-b")}},
		files: map[string][]byte{"a.json": []byte("new-a")},
	}

	a, aerr := o.ReadFile("a.json")
	b, berr := o.ReadFile("b.json")
	f, oerr := o.Open("a.json")
	if oerr != nil {
		t.Fatalf("Open: %v", oerr)
	}
	defer f.Close()
	opened, _ := fs.ReadFile(fstest.MapFS{"x": {Data: mustReadAll(t, f)}}, "x")

	if string(a) != "new-a" || aerr != nil || string(b) != "base-b" || berr != nil {
		t.Errorf("ReadFile = %q/%v, %q/%v, want the rewrite for a.json and the base for b.json", a, aerr, b, berr)
	}
	if string(opened) != "base-a" {
		t.Errorf("Open(a.json) read %q, want the base file: Open is the fs.FS floor, ReadFile carries the overlay", opened)
	}
}

func mustReadAll(t *testing.T, f fs.File) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTierSummary_NamesEachTierWithItsCLIModelAndEffort(t *testing.T) {
	t.Parallel()
	got := tierSummary([]tierRoute{
		{Tier: "deep"},
		{Tier: "fast", CLI: "agy", Effort: "low", EffortSource: "compiled default"},
		{Tier: "balanced", CLI: "codex"},
		{Tier: "top", CLI: "claude", Model: "opus", Effort: "max", EffortSource: "cli_routing.tiers.top"},
	})

	want := "deep: no CLI the ceiling permits · fast: agy effort low (compiled default) · balanced: codex · top: claude opus effort max (cli_routing.tiers.top)"
	if got != want {
		t.Errorf("tierSummary =\n%q\nwant\n%q", got, want)
	}
}

func TestRunCLIRoutingWrite_RefusesABadFlagAndWrongArguments(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	_, before := readRoutingPolicy(t, root)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"set", "--bogus"}, "flag provided but not defined: -bogus"},
		{inRoot(root, "set", "tiers.deep"), "evolve cli-routing set: wrong arguments"},
		{inRoot(root, initVerb), "evolve cli-routing init: wrong arguments"},
	}
	for _, tc := range cases {
		rc, _, stderr := runRoutingVerb(t, tc.args...)
		if _, after := readRoutingPolicy(t, root); rc != exitRoutingUsage || !strings.Contains(stderr, tc.want) || !bytes.Equal(before, after) {
			t.Errorf("%v: rc=%d stderr=%q, want rc %d, %q and no write", tc.args, rc, stderr, exitRoutingUsage, tc.want)
		}
	}
}

func TestCLIRoutingInit_RefusesAProfileOverrideWithNoTableForm(t *testing.T) {
	root := routingWriteProject(t, `{"gc": {"mode": "enforce"}}`)
	path := filepath.Join(routingProfilesDir(root), "auditor.json")
	body := `{"name":"auditor","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"deep","effort_overrides":{"top":"max"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, before := readRoutingPolicy(t, root)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, initVerb, "--clis", "agy,claude")...)

	_, after := readRoutingPolicy(t, root)
	if raw, _ := os.ReadFile(path); rc != exitRoutingFinding || !strings.Contains(stderr, "effort_overrides.top") || string(raw) != body || !bytes.Equal(before, after) {
		t.Errorf("rc=%d stderr=%q, want a refusal naming effort_overrides.top and nothing written", rc, stderr)
	}
}

func TestWriteRoutingEdit_AFailedTableWriteIsAFinding(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}
	root := routingWriteProject(t, effortTable)
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.Chmod(evolveDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })
	var stdout, stderr bytes.Buffer
	w := routingWrite{verb: "cli-routing set", flags: routingWriteFlags{root: root}, rewrites: map[string][]byte{}, stdout: &stdout, stderr: &stderr}

	rc := writeRoutingEdit(w, setRoutingEdit(routingSet{key: "tiers.deep", effort: "high"}))

	if rc != exitRoutingFinding || !strings.Contains(stderr.String(), "evolve cli-routing set: atomicwrite: create temp") || strings.Contains(stdout.String(), "wrote") {
		t.Errorf("rc=%d stdout=%q stderr=%q, want a finding that names the failed write and no wrote line", rc, stdout.String(), stderr.String())
	}
}

func TestWriteRoutingEdit_AFailedProfileRewriteAfterTheTableIsAFinding(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	var stdout, stderr bytes.Buffer
	rewrites := map[string][]byte{"auditor.json/x.json": []byte("{}")}
	w := routingWrite{verb: "cli-routing migrate", flags: routingWriteFlags{root: root}, rewrites: rewrites, stdout: &stdout, stderr: &stderr}

	rc := writeRoutingEdit(w, setRoutingEdit(routingSet{key: "tiers.deep", effort: "high"}))

	pol, _ := readRoutingPolicy(t, root)
	if rc != exitRoutingFinding || !strings.Contains(stderr.String(), "the table is written, but a profile rewrite failed (run the verb again)") {
		t.Errorf("rc=%d stderr=%q, want the partial-write finding", rc, stderr.String())
	}
	if pol.CLIRouting.Tiers["deep"].Effort != "high" {
		t.Errorf("tiers.deep = %+v, want the table written before the profile failure", pol.CLIRouting.Tiers["deep"])
	}
}
