package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

func bridgeBackfillRun(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runBridge(args, nil, &out, &errb)
	return out.String(), errb.String(), code
}

func TestRunBridge_EachSubcommandHelpPrintsItsUsageAndExits0(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"probe":    "Usage: evolve bridge probe [--cli=NAME]\n",
		"report":   "Usage: evolve bridge report --workspace=PATH [--artifact-name=NAME]\n",
		"add-rule": "Usage: evolve bridge add-rule --cli=NAME --regex=R [--response=KEYS] [--policy=auto_respond|escalate] [--name=N] [--note=...]\n",
		"send":     "Usage: evolve bridge send --workspace=DIR --agent=NAME [--kind=command|interrupt|nudge|system_rule] [--source=cli] <body...>\n",
		"sessions": bridgeSessionsUsage + "\n",
	}
	for sub, want := range cases {
		for _, flag := range []string{"--help", "-h"} {
			out, errOut, code := bridgeBackfillRun(sub, flag)
			if code != 0 || out != want || errOut != "" {
				t.Errorf("bridge %s %s = %d stdout=%q stderr=%q, want 0 and %q", sub, flag, code, out, errOut, want)
			}
		}
	}
}

func TestRunBridge_ProbeFiltersToOneCLI(t *testing.T) {
	t.Parallel()
	out, _, code := bridgeBackfillRun("probe", "--cli=claude-tmux", "--ignored")

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	var got struct {
		Results []struct{ CLI string } `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].CLI != "claude-tmux" {
		t.Errorf("results = %+v, want only claude-tmux", got.Results)
	}
}

func TestRunBridge_AProbeErrorExits1(t *testing.T) {
	prev := bridgeProbe
	t.Cleanup(func() { bridgeProbe = prev })
	bridgeProbe = func(*bridge.Engine, context.Context) (core.BridgeProbe, error) {
		return core.BridgeProbe{}, errors.New("manifests unreadable")
	}

	out, errOut, code := bridgeBackfillRun("probe")

	if code != 1 || out != "" || errOut != "evolve bridge probe: manifests unreadable\n" {
		t.Errorf("code=%d stdout=%q stderr=%q, want 1 and the probe error", code, out, errOut)
	}
}

func TestRunBridge_Report(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("# done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Run("requires a workspace", func(t *testing.T) {
		t.Parallel()
		out, errOut, code := bridgeBackfillRun("report", "--artifact-name=x.md")
		if code != 10 || out != "" || errOut != "evolve bridge report: --workspace required\n" {
			t.Errorf("code=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
	t.Run("an absent workspace is refused", func(t *testing.T) {
		t.Parallel()
		out, errOut, code := bridgeBackfillRun("report", "--workspace="+filepath.Join(ws, "absent"))
		if code != 10 || out != "" || errOut != "evolve bridge report: file does not exist\n" {
			t.Errorf("code=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
	t.Run("the named artifact is reported", func(t *testing.T) {
		t.Parallel()
		out, errOut, code := bridgeBackfillRun("report", "--workspace="+ws, "--artifact-name=scout-report.md")
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, errOut)
		}
		var rep bridge.Report
		if err := json.Unmarshal([]byte(out), &rep); err != nil {
			t.Fatal(err)
		}
		if rep.Artifact.Path != filepath.Join(ws, "scout-report.md") || !rep.Artifact.Exists || rep.Artifact.SizeBytes != 7 {
			t.Errorf("artifact = %+v, want the existing 7-byte scout report", rep.Artifact)
		}
	})
	t.Run("the default artifact name is artifact.md", func(t *testing.T) {
		t.Parallel()
		out, _, code := bridgeBackfillRun("report", "--workspace="+ws)
		var rep bridge.Report
		if err := json.Unmarshal([]byte(out), &rep); err != nil || code != 0 {
			t.Fatalf("code=%d err=%v", code, err)
		}
		if rep.Artifact.Path != filepath.Join(ws, "artifact.md") || rep.Artifact.Exists {
			t.Errorf("artifact = %+v, want an absent artifact.md", rep.Artifact)
		}
	})
}

func TestRunBridge_ValidatePointsAtLaunch(t *testing.T) {
	t.Parallel()
	out, errOut, code := bridgeBackfillRun("validate")
	if code != 0 || out != "" || !strings.Contains(errOut, "pass --validate-only to `bridge launch`") {
		t.Errorf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func bridgeOverrideRule(t *testing.T, dir, cli, name string) bridge.ManifestPrompt {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, cli+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m bridge.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, p := range m.InteractivePrompts {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("rule %q not in %s", name, cli)
	return bridge.ManifestPrompt{}
}

func TestRunBridge_AddRuleWritesEveryFlagAndDefaultsTheRest(t *testing.T) {
	dir := t.TempDir()
	writeBridgePolicy(t, map[string]string{"manifest_dir": dir})

	out, errOut, code := bridgeBackfillRun("add-rule", "--cli=claude-p", "--regex=PICK ME", "--policy=auto_respond", "--name=pick_rule", "--note=from a test", "--response=1")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if out != fmt.Sprintf("appended rule %q to %s\n", "pick_rule", filepath.Join(dir, "claude-p.json")) {
		t.Errorf("stdout = %q", out)
	}
	got := bridgeOverrideRule(t, dir, "claude-p", "pick_rule")
	if got.Regex != "PICK ME" || got.Policy != "auto_respond" || got.Note != "from a test" || got.ResponseKeys != "1" {
		t.Errorf("rule = %+v, want every flag carried", got)
	}

	out, errOut, code = bridgeBackfillRun("add-rule", "--cli=claude-p", "--regex=WAIT")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	name := strings.TrimSuffix(strings.TrimPrefix(out, `appended rule "`), `" to `+filepath.Join(dir, "claude-p.json")+"\n")
	if !strings.HasPrefix(name, "claude_p_rule_") {
		t.Fatalf("generated name = %q, want claude_p_rule_<unix>", name)
	}
	got = bridgeOverrideRule(t, dir, "claude-p", name)
	if got.Policy != "escalate" || got.Note != "Added by evolve bridge add-rule" {
		t.Errorf("rule = %+v, want a keyless rule to escalate with the default note", got)
	}
}

func TestRunBridge_AddRuleForAnUnknownCLIFails(t *testing.T) {
	writeBridgePolicy(t, map[string]string{"manifest_dir": t.TempDir()})

	out, errOut, code := bridgeBackfillRun("add-rule", "--cli=no-such-cli", "--regex=X")

	if code != 10 || out != "" || !strings.HasPrefix(errOut, "evolve bridge add-rule: ") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 10 and the load error", code, out, errOut)
	}
}

func TestRunBridge_SendRefusals(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file-not-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"an unknown flag", []string{"--workspace=/w", "--agent=a", "--bogus=1", "body"}, "evolve bridge send: unknown flag \"--bogus=1\"\n"},
		{"an unwritable workspace", []string{"--workspace=" + blocker, "--agent=scout", "--source=test", "hello"}, "evolve bridge send: inbox: mkdir: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, errOut, code := bridgeBackfillRun(append([]string{"send"}, tc.args...)...)
			if code != 10 || out != "" || !strings.HasPrefix(errOut, tc.want) {
				t.Errorf("code=%d stdout=%q stderr=%q, want 10 and %q", code, out, errOut, tc.want)
			}
		})
	}
}

func TestRunBridge_SendCarriesTheSourceFlag(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	_, errOut, code := bridgeBackfillRun("send", "--workspace="+ws, "--agent=scout", "--source=operator", "--kind=nudge", "keep", "going")

	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	raw, err := os.ReadFile(inbox.Path(ws, "scout"))
	if err != nil {
		t.Fatal(err)
	}
	var env inbox.Envelope
	if err := json.Unmarshal(bytes.TrimSpace(raw), &env); err != nil {
		t.Fatal(err)
	}
	if env.Source != "operator" || env.Kind != "nudge" || env.Body != "keep going" {
		t.Errorf("envelope = %+v, want source=operator kind=nudge body=\"keep going\"", env)
	}
}

func TestRunBridge_ControlDispatchesToTheControlVerb(t *testing.T) {
	t.Parallel()
	out, _, code := bridgeBackfillRun("control", "--help")
	if code != 0 || !strings.HasPrefix(out, "Usage: evolve bridge control <family> <event>") {
		t.Errorf("code=%d stdout=%q, want the control usage", code, out)
	}
}

func TestRunBridge_DoctorDeepJSONMarksTheReportDeep(t *testing.T) {
	t.Parallel()
	out, errOut, code := bridgeBackfillRun("doctor", "--cli=no-such-cli", "--deep", "--json")

	var rep bridge.DoctorReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout %q is not JSON: %v (stderr=%q)", out, err, errOut)
	}
	if !rep.Deep || len(rep.Results) != 0 || code != 0 {
		t.Errorf("report = %+v code=%d, want deep=true, no results, exit 0", rep, code)
	}
}

func TestRunBridge_BillingSnapshotIntoAFileFails(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := bridgeBackfillRun("billing", "snapshot", filepath.Join(blocker, "sub"), "pre")

	if code != 10 || out != "" || !strings.HasPrefix(errOut, "evolve bridge billing: bridge:billing: mkdir ") {
		t.Errorf("code=%d stdout=%q stderr=%q, want 10 and the mkdir error", code, out, errOut)
	}
}

func TestRunBridge_VersionAliasesPrintTheVersion(t *testing.T) {
	t.Parallel()
	for _, alias := range []string{"version", "-v", "--version"} {
		out, errOut, code := bridgeBackfillRun(alias)
		if code != 0 || out != version.Get()+"\n" || errOut != "" {
			t.Errorf("bridge %s = %d stdout=%q stderr=%q, want the version", alias, code, out, errOut)
		}
	}
}

func TestBridgeSessionCells(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	if busyWord(true) != "busy" || busyWord(false) != "idle" {
		t.Errorf("busyWord = %q/%q, want busy/idle", busyWord(true), busyWord(false))
	}
	ages := []struct {
		then time.Time
		want int64
	}{
		{time.Time{}, -1},
		{now, 0},
		{now.Add(-1499 * time.Millisecond), 1},
		{now.Add(-1500 * time.Millisecond), 2},
		{now.Add(-time.Hour), 3600},
	}
	for _, a := range ages {
		if got := ageSeconds(now, a.then); got != a.want {
			t.Errorf("ageSeconds(now-%v) = %d, want %d", now.Sub(a.then), got, a.want)
		}
	}
	firsts := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"", ""}, ""},
		{[]string{"", "b", "c"}, "b"},
		{[]string{"ä"}, "ä"},
	}
	for _, f := range firsts {
		if got := firstNonEmpty(f.in...); got != f.want {
			t.Errorf("firstNonEmpty(%q) = %q, want %q", f.in, got, f.want)
		}
	}
	cycles := map[int]string{0: "-", 1: "1", 1806: "1806", -2: "-2"}
	for in, want := range cycles {
		if got := cycleCell(in); got != want {
			t.Errorf("cycleCell(%d) = %q, want %q", in, got, want)
		}
	}
	cells := map[int64]string{-1: "-", -5: "-", 0: "0s", 59: "59s", 3661: "1h1m1s"}
	for in, want := range cells {
		if got := ageCell(in); got != want {
			t.Errorf("ageCell(%d) = %q, want %q", in, got, want)
		}
	}
}

func FuzzAgeCell(f *testing.F) {
	for _, s := range []int64{-1, 0, 1, 59, 60, 3600, 86400} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, seconds int64) {
		got := ageCell(seconds)
		if (seconds < 0) != (got == "-") {
			t.Fatalf("ageCell(%d) = %q: the dash marks exactly the negative ages", seconds, got)
		}
	})
}

func TestWriteSessionsJSON(t *testing.T) {
	t.Run("rows are printed as an indented array", func(t *testing.T) {
		var out, errb bytes.Buffer
		code := writeSessionsJSON([]bridgeSessionRow{{Cycle: 3, Phase: "scout"}}, &out, &errb)
		var rows []bridgeSessionRow
		if code != 0 || json.Unmarshal(out.Bytes(), &rows) != nil || len(rows) != 1 || rows[0].Phase != "scout" || !strings.Contains(out.String(), "\n  {") {
			t.Errorf("code=%d stdout=%q, want one indented row", code, out.String())
		}
	})
	t.Run("no rows print null", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := writeSessionsJSON(nil, &out, &errb); code != 0 || out.String() != "null\n" {
			t.Errorf("code=%d stdout=%q", code, out.String())
		}
	})
	t.Run("a marshal failure exits 1", func(t *testing.T) {
		prev := sessionsMarshalIndent
		t.Cleanup(func() { sessionsMarshalIndent = prev })
		sessionsMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
		var out, errb bytes.Buffer
		if code := writeSessionsJSON(nil, &out, &errb); code != 1 || out.Len() != 0 || errb.String() != "evolve bridge sessions: boom\n" {
			t.Errorf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
		}
	})
}

func TestLiveSnapshots_SortsByCycleAndWarnsOnAnUnreadableSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	late, early := routerSnapshot(), routerSnapshot()
	late.Cycle, late.Session = 1806, "late"
	early.Cycle, early.Session = 9, "early"
	liveRunWithSnapshot(t, root, 1806, late)
	ws := liveRunWithSnapshot(t, root, 9, early)
	bad := filepath.Join(ws, "broken-pane-watch.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	snaps := liveSnapshots(root, sessionsNow, &stderr)

	if len(snaps) != 2 || snaps[0].Session != "early" || snaps[1].Session != "late" {
		t.Fatalf("snaps = %+v, want cycle 9 before cycle 1806", bridgeSessionNames(snaps))
	}
	if !strings.Contains(stderr.String(), "evolve bridge sessions: WARN "+ws+": ") {
		t.Errorf("stderr = %q, want a WARN naming the run dir", stderr.String())
	}
}

func bridgeSessionNames(snaps []panewatch.Snapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, fmt.Sprintf("%d:%s", s.Cycle, s.Session))
	}
	return out
}

func TestLiveLoopSockets_KeepsOnlyLiveLoopPIDSockets(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TMUX_TMPDIR", base)
	dir := bridge.TmuxSocketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	live := fmt.Sprintf("evolve-bridge-p%d", os.Getpid())
	for _, n := range []string{live, "evolve-bridge-p0", "evolve-bridge-pabc", "evolve-bridge-t42", "evolve-bridge-p2147483646", "other-socket"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := liveLoopSockets()

	if len(got) != 1 || got[0] != live {
		t.Errorf("liveLoopSockets = %q, want only %q", got, live)
	}
}

func TestLiveLoopSockets_AnEmptyOrUnlistableDirectoryYieldsNone(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	if got := liveLoopSockets(); got != nil {
		t.Errorf("liveLoopSockets = %q with no socket dir, want nil", got)
	}
	t.Setenv("TMUX_TMPDIR", filepath.Join(t.TempDir(), "bad[pattern"))
	if got := liveLoopSockets(); got != nil {
		t.Errorf("liveLoopSockets = %q on a malformed glob, want nil", got)
	}
}
