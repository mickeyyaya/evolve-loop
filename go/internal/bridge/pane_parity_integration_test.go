//go:build integration && darwin

package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

type parityTarget struct {
	pkg, tags string
	named     []string
	filtered  bool
}

var paneParityTargets = []parityTarget{
	{pkg: "./internal/swarm", named: []string{"TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget"}},
	{pkg: "./internal/bridge", tags: "integration", filtered: true,
		named: []string{"TestRealTmux_HappyPath", "TestRealTmux_AgentTmuxProbeCannotKillTheRunServer"}},
}

type goReport struct {
	results testResults
	dropped []string
	rc      int
}

func newGoReport(out string, rc int) goReport {
	results, dropped := parseTestJSON(out)
	return goReport{results: results, dropped: dropped, rc: rc}
}

func (r goReport) validate() error {
	if len(r.dropped) > 0 {
		return fmt.Errorf("the report has %d line(s) that are not test2json: %q", len(r.dropped), r.dropped)
	}
	pkg, ok := r.results[packageResult]
	if !ok {
		return fmt.Errorf("the report has no package result: it is truncated")
	}
	failed := slices.Contains(slices.Collect(maps.Values(r.results)), "fail")
	if r.rc != 0 && !failed {
		return fmt.Errorf("exit status %d but no test failed", r.rc)
	}
	if r.rc == 0 && (failed || pkg != "pass") {
		return fmt.Errorf("exit status 0 but the package result is %s", pkg)
	}
	return nil
}

func (p parityTarget) hostEvidence(r goReport) error {
	var missing []string
	for _, name := range p.named {
		if got := r.results[name]; got != "pass" {
			missing = append(missing, fmt.Sprintf("%s %s: host=%s, want pass", p.pkg, name, orMissing(got)))
		}
	}
	if len(missing) > 0 {
		return errors.New(strings.Join(missing, "; "))
	}
	return nil
}

func (p parityTarget) goTestArgs() []string {
	args := []string{"test", "-json", "-count=1"}
	if p.tags != "" {
		args = append(args, "-tags", p.tags)
	}
	if p.filtered {
		args = append(args, "-run", "^("+strings.Join(p.named, "|")+")$")
	}
	return append(args, p.pkg)
}

const (
	parityDoneMarker  = "PANE-PARITY-DONE"
	parityDispatchID  = "parity-dispatch"
	parityManifestKey = "EVOLVE_PARITY_MANIFEST"
)

type testResults map[string]string

func TestPaneParity_RealTmuxPackagesPassInTheSandboxedPaneAsOnTheHost(t *testing.T) {
	requireTmux(t)
	probe := requireNativeSandbox(t)
	for _, target := range paneParityTargets {
		t.Run(target.pkg, func(t *testing.T) {
			host := runOnHost(t, target)
			pane := runInPane(t, probe, target, nil)
			if diffs := paneOnlyFailures(target.pkg, host.results, pane.report.results); len(diffs) > 0 {
				t.Fatalf("pane parity: these tests pass on the host and fail in the sandboxed pane:\n%s", strings.Join(diffs, "\n"))
			}
			if err := paneEnvContract(repoRootForParity(t), pane.env); err != nil {
				t.Fatalf("pane env contract: %v", err)
			}
		})
	}
}

func TestPaneParity_ReportsTheCycle1853TriggerWhenThePtyRulesAreRemoved(t *testing.T) {
	requireTmux(t)
	probe := requireNativeSandbox(t)
	swarm := paneParityTargets[0]
	host := runOnHost(t, swarm)
	pane := runInPane(t, probe, swarm, withoutOwnPtyRules)
	diffs := paneOnlyFailures(swarm.pkg, host.results, pane.report.results)
	want := "./internal/swarm TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget: host=pass pane=fail"
	if !slices.Contains(diffs, want) {
		t.Fatalf("without the R1 pty rules the harness must report %q, got:\n%s", want, strings.Join(diffs, "\n"))
	}
}

func withoutOwnPtyRules(t *testing.T, profile string) {
	t.Helper()
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	lines := slices.DeleteFunc(strings.Split(string(raw), "\n"), func(l string) bool {
		return strings.Contains(l, "pseudo-tty") || strings.Contains(l, `"/dev/ptmx"`) || strings.Contains(l, "com.apple.sandbox.pty")
	})
	if err := os.WriteFile(profile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func paneOnlyFailures(pkg string, host, pane testResults) []string {
	var diffs []string
	for _, name := range slices.Sorted(maps.Keys(host)) {
		if host[name] == "pass" && pane[name] != "pass" {
			diffs = append(diffs, fmt.Sprintf("%s %s: host=pass pane=%s", pkg, name, orMissing(pane[name])))
		}
	}
	return diffs
}

func orMissing(action string) string {
	if action == "" {
		return "missing"
	}
	return action
}

func paneEnvContract(root string, env []string) error {
	for _, kv := range env {
		if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=") {
			return fmt.Errorf("the pane still carries %s", kv)
		}
	}
	for _, want := range []string{"EVOLVE_PROJECT_ROOT=" + root, "EVOLVE_DISPATCH_ID=" + parityDispatchID, parityManifestKey + "=1"} {
		if !slices.Contains(env, want) {
			return fmt.Errorf("the pane misses the boot export %s", want)
		}
	}
	return nil
}

func runOnHost(t *testing.T, target parityTarget) goReport {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", target.goTestArgs()...)
	cmd.Dir = filepath.Join(repoRootForParity(t), "go")
	cmd.Env = gotooltest.Env(paneEnv(os.Environ()))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	rc := 0
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		rc = exit.ExitCode()
	case err != nil:
		t.Fatalf("start the host run of %s: %v", target.pkg, err)
	}
	report := newGoReport(string(out), rc)
	if err := report.validate(); err != nil {
		t.Fatalf("the host run of %s is not evidence: %v\nstderr:\n%s", target.pkg, err, stderr.String())
	}
	if err := target.hostEvidence(report); err != nil {
		t.Fatalf("the host run of %s proves nothing: %v", target.pkg, err)
	}
	return report
}

type paneRun struct {
	report goReport
	env    []string
}

func runInPane(t *testing.T, probe sandbox.ProbeResult, target parityTarget, mutate func(*testing.T, string)) paneRun {
	t.Helper()
	cfg := itConfig(t, "x")
	cfg.ProjectRoot, cfg.RequireSandbox, cfg.DispatchID = repoRootForParity(t), true, parityDispatchID
	cfg.Realization.Env = map[string]string{parityManifestKey: "1"}
	file := func(name string) string { return filepath.Join(cfg.Workspace, name) }
	mustWrite(t, file("parity.sh"), fmt.Sprintf("env > %s\ngo %s > %s 2> %s\necho $? > %s\nprintf '%%s\\n' %s\nexec sleep 600\n",
		shellQuotePOSIX(file("pane.env")), joinPrefixForTmux(target.goTestArgs()), shellQuotePOSIX(file("pane.json")),
		shellQuotePOSIX(file("pane.stderr")), shellQuotePOSIX(file("pane.rc")), parityDoneMarker))
	deps := itDeps(250 * time.Millisecond)
	deps.Env, deps.LookupEnv, deps.BootTimeoutS = map[string]string{}, mapLookup(nil), 2400
	wrap := defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	deps.SandboxWrap = func(req SandboxWrapRequest) ([]string, bool) {
		prefix, ok := wrap(req)
		if ok && mutate != nil {
			mutate(t, prefix[2])
		}
		return prefix, ok
	}
	session := itSession("parity")
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(context.Background(), session) })
	_, code, err := bootTmuxREPL(context.Background(), cfg, deps, itLaunch(session, "/bin/sh "+shellQuotePOSIX(file("parity.sh")), parityDoneMarker, 0, false),
		replPreparation{prefix: "[parity]", workingDir: filepath.Join(cfg.ProjectRoot, "go")}, nil)
	if err != nil || code != ExitOK {
		t.Fatalf("the pane run of %s did not finish: code=%d err=%v", target.pkg, code, err)
	}
	return paneRun{report: paneReport(t, target, file), env: strings.Split(readFile(t, file("pane.env")), "\n")}
}

func paneReport(t *testing.T, target parityTarget, file func(string) string) goReport {
	t.Helper()
	rc, err := strconv.Atoi(strings.TrimSpace(readFile(t, file("pane.rc"))))
	if err != nil {
		t.Fatalf("the pane run of %s wrote no exit status: %v", target.pkg, err)
	}
	report := newGoReport(readFile(t, file("pane.json")), rc)
	if err := report.validate(); err != nil {
		t.Fatalf("the pane run of %s is not evidence: %v\nstderr:\n%s", target.pkg, err, readFile(t, file("pane.stderr")))
	}
	return report
}

func parseTestJSON(out string) (testResults, []string) {
	results, dropped := testResults{}, []string{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		var ev struct{ Action, Test string }
		if json.Unmarshal(scanner.Bytes(), &ev) != nil {
			dropped = append(dropped, scanner.Text())
			continue
		}
		name := ev.Test
		if name == "" {
			name = packageResult
		}
		if ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip" {
			results[name] = ev.Action
		}
	}
	return results, dropped
}

const packageResult = "(package)"

func repoRootForParity(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("pane parity: read the package directory: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
