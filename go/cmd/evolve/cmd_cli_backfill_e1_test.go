package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestUpdateCLIs_UnreadableRecordsWarnTwiceAndStillReport(t *testing.T) {
	h := claudeRaisedHost()
	h.install(t)
	evolveDir := t.TempDir()
	if err := os.WriteFile(cliupdate.RecordsPath(evolveDir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	rep := updateCLIs(context.Background(), t.TempDir(), evolveDir, false, &log)

	if len(rep.Results) != 2 || rep.Results[0].NewVersion != "2.1.286" {
		t.Errorf("report = %+v, want the update still reported", rep)
	}
	if !strings.Contains(log.String(), "[cli-update] WARN: could not read "+cliupdate.RecordsPath(evolveDir)) {
		t.Errorf("log = %q, want the read WARN", log.String())
	}
	if !strings.Contains(log.String(), "[cli-update] WARN: could not record the boundary updates in "+cliupdate.RecordsPath(evolveDir)) {
		t.Errorf("log = %q, want the record WARN", log.String())
	}
	if raw, _ := os.ReadFile(cliupdate.RecordsPath(evolveDir)); string(raw) != "{not json" {
		t.Errorf("records = %q, want the unreadable file left as it was", raw)
	}
}

func TestUpdateCLIs_ADryRunNeitherReadsNorRecords(t *testing.T) {
	h := claudeRaisedHost()
	h.install(t)
	evolveDir := t.TempDir()
	if err := os.WriteFile(cliupdate.RecordsPath(evolveDir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	rep := updateCLIs(context.Background(), t.TempDir(), evolveDir, true, &log)

	if !rep.DryRun || log.Len() != 0 || len(h.calls) != 0 {
		t.Errorf("dryRun=%v log=%q calls=%v, want a plan with no probe and no WARN", rep.DryRun, log.String(), h.calls)
	}
}

func TestProductionCLIUpdateWiring_ProbesAndSmokesThroughTheLiveDoctor(t *testing.T) {
	t.Parallel()
	var log bytes.Buffer
	w := productionCLIUpdateWiring(t.TempDir(), &log)

	for name, check := range map[string]func(context.Context, string) error{"probe": w.seams.Probe, "smoke": w.seams.Smoke} {
		err := check(context.Background(), "nosuchfamily")
		if err == nil || err.Error() != `doctor live nosuchfamily-tmux rc=10 pattern=""` {
			t.Errorf("%s(nosuchfamily) = %v, want the bad-flags refusal of an unknown driver", name, err)
		}
	}
	if w.now == nil || w.seams.Version == nil || w.seams.Run == nil || w.seams.Eligible == nil {
		t.Errorf("wiring = %+v, want every seam bound", w)
	}
}

func TestPaneTailLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
	}{
		{"no pane", "", "(no pane was captured)"},
		{"only blank lines", "\n  \n\t\n", "(no pane was captured)"},
		{"one line", "ready >", "ready >"},
		{"lines joined with a bar and spaces collapsed", "a  b\n\nc\t d\n", "a b | c d"},
		{"only the last six lines", "1\n2\n3\n4\n5\n6\n7\n8\n", "3 | 4 | 5 | 6 | 7 | 8"},
	}
	for _, tc := range cases {
		if got := paneTailLine(tc.in); got != tc.want {
			t.Errorf("%s: paneTailLine(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestCLIUpdateRoot_FlagThenEnvThenTheWorkingDirectory(t *testing.T) {
	env := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", env)
	var stderr bytes.Buffer
	if got := cliUpdateRoot("/flag/root", &stderr); got != "/flag/root" {
		t.Errorf("flag: got %q", got)
	}
	if got := cliUpdateRoot("", &stderr); got != env {
		t.Errorf("env: got %q, want %q", got, env)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := cliUpdateRoot("", &stderr); got != wd {
		t.Errorf("cwd: got %q, want %q", got, wd)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want no WARN for resolvable roots", stderr.String())
	}
}

func TestRoutingCandidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	echo := func(_ []byte, pol policy.Policy) (map[string]any, error) {
		return map[string]any{"cli_routing": policy.CLIRouting{CLIs: []string{"agy"}, Default: []string{"agy"}}}, nil
	}
	refuse := func([]byte, policy.Policy) (map[string]any, error) { return nil, errors.New("edit refused") }

	t.Run("a directory in place of the file is a read error", func(t *testing.T) {
		if _, err := routingCandidate(dir, echo); err == nil || !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("err = %v, want the read error", err)
		}
	})
	t.Run("a policy that does not parse is refused", func(t *testing.T) {
		_, err := routingCandidate(write("bad.json", `{"cli_routing":`), echo)
		if err == nil || !strings.HasPrefix(err.Error(), "the current policy.json does not parse: ") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("an edit error is returned as is", func(t *testing.T) {
		if _, err := routingCandidate(write("ok.json", `{}`), refuse); err == nil || err.Error() != "edit refused" {
			t.Errorf("err = %v", err)
		}
	})
	for _, body := range []string{"", "  \n\t"} {
		t.Run("a blank file reads as an empty object: "+strings.TrimSpace(body), func(t *testing.T) {
			got, err := routingCandidate(write("blank.json", body), echo)
			if err != nil {
				t.Fatal(err)
			}
			pol, perr := policy.Parse(got)
			if perr != nil || pol.CLIRouting == nil || !reflect.DeepEqual(pol.CLIRouting.CLIs, []string{"agy"}) {
				t.Errorf("candidate = %s (%v), want the patched block", got, perr)
			}
		})
	}
	t.Run("an absent file starts from an empty object", func(t *testing.T) {
		got, err := routingCandidate(filepath.Join(dir, "absent.json"), echo)
		if err != nil || !strings.Contains(string(got), `"cli_routing"`) {
			t.Errorf("candidate = %s, %v", got, err)
		}
	})
}

func TestOrEmptyObject(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "{}", " \n": "{}", "{}": "{}", `{"a":1}`: `{"a":1}`, " {} ": " {} "} {
		if got := string(orEmptyObject([]byte(in))); got != want {
			t.Errorf("orEmptyObject(%q) = %q, want %q", in, got, want)
		}
	}
	if got := string(orEmptyObject(nil)); got != "{}" {
		t.Errorf("orEmptyObject(nil) = %q", got)
	}
}

func TestRoutingEdits_WithoutABlockRefuse(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit routingEdit
		want string
	}{
		"set":   {setRoutingEdit(routingSet{key: "clis", values: []string{"agy"}}), "there is no cli_routing block: run evolve cli-routing init --clis first"},
		"unset": {unsetRoutingEdit("after_chain"), "there is no cli_routing block"},
	}
	for name, tc := range cases {
		patch, err := tc.edit(nil, policy.Policy{})
		if err == nil || err.Error() != tc.want || patch != nil {
			t.Errorf("%s: (%v, %v), want nil and %q", name, patch, err, tc.want)
		}
		block := policy.CLIRouting{CLIs: []string{"agy"}, Default: []string{"agy"}, AfterChain: "stop"}
		patch, err = tc.edit(nil, policy.Policy{CLIRouting: &block})
		if err != nil || patch["cli_routing"] == nil {
			t.Errorf("%s with a block: (%v, %v), want the edited block", name, patch, err)
		}
	}
}

func TestTierRoutes_AnEmptyTierChainFallsBackToTheModel(t *testing.T) {
	t.Parallel()
	got := tierRoutes(llmroute.Plan{Model: "balanced", Candidates: []string{"agy-tmux", "claude-tmux"},
		TierCeiling: map[string][]string{"balanced": {"claude"}}})

	if len(got) != 1 || got[0].Tier != "balanced" || got[0].CLI != "claude-tmux" || !reflect.DeepEqual(got[0].Filtered, []string{"agy-tmux"}) {
		t.Errorf("tierRoutes = %+v, want one balanced tier on claude with agy filtered", got)
	}
}

func TestRoutingView_AgentNotesNameABenchedFamilyInTheChain(t *testing.T) {
	t.Parallel()
	v := routingView{benched: map[string]bool{"claude": true, "codex": true}}

	notes := v.agentNotes("scout", agentRoute{Chain: []string{"agy-tmux", "claude-tmux"}})

	if !reflect.DeepEqual(notes, []string{"host: claude benched (demoted)"}) {
		t.Errorf("notes = %q, want only claude (codex is not in the chain)", notes)
	}
}

func TestSortedKeysOf(t *testing.T) {
	t.Parallel()
	if got := sortedKeysOf(nil); got == nil || len(got) != 0 {
		t.Errorf("sortedKeysOf(nil) = %#v, want an empty slice", got)
	}
	if got := sortedKeysOf(map[string]bool{"codex": true, "agy": false, "claude": true}); !reflect.DeepEqual(got, []string{"agy", "claude", "codex"}) {
		t.Errorf("sortedKeysOf = %q, want every key sorted", got)
	}
}

func TestPrintRoutingReport_NamesTheBenchAndARefusedAgent(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	printRoutingReport(&out, routingReport{
		Mode: "declared cli_routing table", Static: true, Benched: []string{"agy", "claude"},
		Agents: []agentRoute{
			{Agent: "auditor", Phase: "audit", Error: "no CLI"},
			{Agent: "scout", Error: "no CLI"},
			{Agent: "builder", Rule: "default", Chain: []string{"agy-tmux"}, Tiers: []tierRoute{{Tier: "balanced", CLI: "agy-tmux"}}},
		},
		Findings: []cliroute.Finding{{Severity: "warn", Key: "agent.x", Message: "m"}},
	})

	got := out.String()
	for _, want := range []string{
		"cli-routing: declared cli_routing table (static: no host probe, bench or discovery)\n",
		"benched now: agy, claude\n",
		"\nchain (refused)\n  no route\n",
		"auditor (phase audit)",
		"REFUSED: no CLI",
		"\nchain agy-tmux\n  balanced: agy-tmux\n",
		"\nfindings:\n[cli-routing] warn agent.x: m\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, " rule \n") {
		t.Errorf("a refused agent prints no rule line:\n%s", got)
	}
	if strings.Index(got, "(refused)") > strings.Index(got, "chain agy-tmux") {
		t.Errorf("the two refused agents form the larger group and print first:\n%s", got)
	}
}

func TestPrintRoutingReport_ALiveReportWithoutABenchOmitsTheBenchLine(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	printRoutingReport(&out, routingReport{Mode: "m"})
	if out.String() != "cli-routing: m (live: probe, cli-health bench and the discovered tail applied)\n" {
		t.Errorf("report = %q", out.String())
	}
	out.Reset()
	printRoutingReport(&out, routingReport{Mode: "m", Benched: []string{"agy"}})
	if !strings.HasSuffix(out.String(), "\nbenched now: agy\n") {
		t.Errorf("report = %q, want a single benched family named", out.String())
	}
}

func cliRoutingRoot(t *testing.T, policyBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunCLIRoutingExplainAndShow_AMalformedPolicyIsAFinding(t *testing.T) {
	root := cliRoutingRoot(t, `{"cli_routing":`)
	for verb, run := range map[string]func([]string, *bytes.Buffer, *bytes.Buffer) int{
		"explain": func(a []string, o, e *bytes.Buffer) int {
			return runCLIRoutingExplain(append([]string{"scout"}, a...), o, e)
		},
		"show": func(a []string, o, e *bytes.Buffer) int { return runCLIRoutingShow(a, o, e) },
	} {
		var out, errb bytes.Buffer
		code := run([]string{"--static", "--project-root", root}, &out, &errb)
		if code != exitRoutingFinding || out.Len() != 0 || !strings.Contains(errb.String(), "evolve cli-routing "+verb+": policy: parse ") {
			t.Errorf("%s: code=%d stdout=%q stderr=%q, want a finding exit and the parse error", verb, code, out.String(), errb.String())
		}
	}
}

func TestRunCLIRoutingShow_AnUnmarshallableReportIsAFinding(t *testing.T) {
	prev := cliRoutingShowMarshalIndent
	t.Cleanup(func() { cliRoutingShowMarshalIndent = prev })
	cliRoutingShowMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
	var out, errb bytes.Buffer

	root := cliRoutingRoot(t, `{}`)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}

	code := runCLIRoutingShow([]string{"--static", "--json", "--project-root", root}, &out, &errb)

	if code != exitRoutingFinding || out.Len() != 0 || errb.String() != "evolve cli-routing show: boom\n" {
		t.Errorf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestRoutingView_AnUnlistableProfilesDirFailsTheReport(t *testing.T) {
	t.Parallel()
	root := cliRoutingRoot(t, `{}`)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "profiles"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := routingView{root: root}.report()

	if err == nil || rep.Mode != "" || rep.Agents != nil {
		t.Errorf("report = (%+v, %v), want an empty report and the list error", rep, err)
	}
}

func TestRunCLIRoutingShow_AnAbsentProfilesDirFailsTheReport(t *testing.T) {
	root := cliRoutingRoot(t, `{}`)
	var out, errb bytes.Buffer

	code := runCLIRoutingShow([]string{"--static", "--project-root", root}, &out, &errb)

	if code != exitRoutingFinding || out.Len() != 0 || errb.String() != "evolve cli-routing show: open .: no such file or directory\n" {
		t.Errorf("code=%d stdout=%q stderr=%q, want the profile listing error", code, out.String(), errb.String())
	}
}

func TestCLIUpdateRoot_AnUnresolvableRootWarnsAndKeepsTheRawValue(t *testing.T) {
	prev := cliUpdateAbsoluteRoot
	t.Cleanup(func() { cliUpdateAbsoluteRoot = prev })
	var gotLabel, gotPath string
	cliUpdateAbsoluteRoot = func(label, p string, warn func(string)) string {
		gotLabel, gotPath = label, p
		warn("could not resolve")
		return p
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stderr bytes.Buffer

	got := cliUpdateRoot("", &stderr)

	if got != "." || gotLabel != "--project-root" || gotPath != "." {
		t.Errorf("cliUpdateRoot = %q (resolved %q %q), want the dot under --project-root", got, gotLabel, gotPath)
	}
	if stderr.String() != "evolve cli update: WARN: could not resolve\n" {
		t.Errorf("stderr = %q, want the prefixed WARN", stderr.String())
	}
}

func TestParseCLIRoutingFlags_AnUnreadableWorkingDirectoryAsksForTheFlag(t *testing.T) {
	prev := cliRoutingGetwd
	t.Cleanup(func() { cliRoutingGetwd = prev })
	cliRoutingGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stderr bytes.Buffer

	_, rest, ok := parseCLIRoutingFlags("show", []string{"--json"}, &stderr)

	if ok || rest != nil {
		t.Errorf("parseCLIRoutingFlags = (%v, %v), want a refusal", rest, ok)
	}
	if stderr.String() != "evolve cli-routing show: cannot read the working directory (getwd: no such file or directory): pass --project-root\n" {
		t.Errorf("stderr = %q, want the --project-root hint", stderr.String())
	}
}
