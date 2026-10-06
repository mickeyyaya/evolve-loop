package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

var cliUpdateNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

type fakeCLIHost struct {
	families []cliupdate.Family
	versions map[string][]string
	runErr   map[string]error
	probeErr map[string]error
	smokeErr map[string]error
	onRun    func()
	onSmoke  func(family string)
	calls    []string
}

func (h *fakeCLIHost) install(t *testing.T) {
	t.Helper()
	prev := cliUpdateWiringFn
	cliUpdateWiringFn = func(string, io.Writer) cliUpdateWiring {
		return cliUpdateWiring{families: h.families, seams: h.seams(), now: func() time.Time { return cliUpdateNow }}
	}
	t.Cleanup(func() { cliUpdateWiringFn = prev })
}

func (h *fakeCLIHost) seams() cliupdate.Seams {
	return cliupdate.Seams{
		Eligible: func(family string) (bool, string) {
			h.calls = append(h.calls, "eligible "+family)
			return true, ""
		},
		Probe: func(_ context.Context, family string) error {
			h.calls = append(h.calls, "probe "+family)
			return h.probeErr[family]
		},
		Version: func(bin string) (string, error) {
			h.calls = append(h.calls, "version "+bin)
			vs := h.versions[bin]
			v := vs[0]
			if len(vs) > 1 {
				h.versions[bin] = vs[1:]
			}
			return v, nil
		},
		Run: func(_ context.Context, f cliupdate.Family) error {
			h.calls = append(h.calls, "run "+strings.Join(f.UpdateArgv, " "))
			if h.onRun != nil {
				h.onRun()
			}
			return h.runErr[f.Name]
		},
		Smoke: func(_ context.Context, family string) error {
			h.calls = append(h.calls, "smoke "+family)
			if h.onSmoke != nil {
				h.onSmoke(family)
			}
			return h.smokeErr[family]
		},
	}
}

func claudeRaisedHost() *fakeCLIHost {
	return &fakeCLIHost{
		families: []cliupdate.Family{{Name: "claude", UpdateArgv: []string{"claude", "update"}}, {Name: "codex"}},
		versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}},
	}
}

func runCLIUpdateVerb(t *testing.T, args ...string) (rc int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc = runCLICommand(args, nil, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestCLIUpdate_PrintsOneRowPerFamilyAndRecordsTheVersionChange(t *testing.T) {
	root := t.TempDir()
	h := claudeRaisedHost()
	h.install(t)

	rc, stdout, stderr := runCLIUpdateVerb(t, "update", "--project-root", root)

	if rc != 0 {
		t.Fatalf("rc=%d, want 0\nstdout=%s\nstderr=%s", rc, stdout, stderr)
	}
	for _, want := range []string{"FAMILY", "STATUS", "claude", "updated", "2.1.285 → 2.1.286", "codex", "no-updater", "update_argv"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table missing %q:\n%s", want, stdout)
		}
	}
	records, err := cliupdate.LoadRecords(filepath.Join(root, ".evolve"))
	if err != nil || !reflect.DeepEqual(records, []cliupdate.Record{{Family: "claude", Kind: cliupdate.KindUpdate, Old: "2.1.285", New: "2.1.286", At: cliUpdateNow}}) {
		t.Errorf("the verb must record the change for cli-version-drift: %+v, %v", records, err)
	}
}

func TestCLIUpdate_JSONPrintsTheWholeReport(t *testing.T) {
	h := claudeRaisedHost()
	h.install(t)

	rc, stdout, _ := runCLIUpdateVerb(t, "update", "--json", "--project-root", t.TempDir())

	var rep cliupdate.Report
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil || rc != 0 {
		t.Fatalf("want a JSON report and rc 0, got rc=%d err=%v\n%s", rc, err, stdout)
	}
	if rep.DryRun || len(rep.Results) != 2 || rep.Results[0].NewVersion != "2.1.286" || rep.Results[1].Status != cliupdate.StatusNoUpdater {
		t.Errorf("unexpected report %+v", rep)
	}
}

func TestCLIUpdate_AnUpdateOrSmokeFailureExitsOne(t *testing.T) {
	cases := map[string]func(*fakeCLIHost){
		"update-failed": func(h *fakeCLIHost) { h.runErr = map[string]error{"claude": errors.New("network unreachable")} },
		"smoke-failed":  func(h *fakeCLIHost) { h.smokeErr = map[string]error{"claude": errors.New("rc=80")} },
	}
	for status, break_ := range cases {
		t.Run(status, func(t *testing.T) {
			h := claudeRaisedHost()
			break_(h)
			h.install(t)

			rc, stdout, _ := runCLIUpdateVerb(t, "update", "--project-root", t.TempDir())

			if rc != 1 || !strings.Contains(stdout, status) {
				t.Fatalf("want rc 1 and a %s row, got rc=%d\n%s", status, rc, stdout)
			}
		})
	}
}

func TestCLIUpdate_DryRunCallsNothingAndRecordsNothing(t *testing.T) {
	root := t.TempDir()
	h := claudeRaisedHost()
	h.install(t)

	rc, stdout, _ := runCLIUpdateVerb(t, "update", "--dry-run", "--project-root", root)

	if rc != 0 || len(h.calls) != 0 {
		t.Fatalf("a dry run probes, updates and smokes nothing: rc=%d calls=%q", rc, h.calls)
	}
	if !strings.Contains(stdout, "dry run") || !strings.Contains(stdout, "planned") || !strings.Contains(stdout, "claude update") {
		t.Errorf("a dry run names what would run:\n%s", stdout)
	}
	if _, err := os.Stat(cliupdate.RecordsPath(filepath.Join(root, ".evolve"))); !os.IsNotExist(err) {
		t.Errorf("a dry run records nothing: stat err=%v", err)
	}
}

func TestCLIUpdate_UsageErrorsExitTwoAndUpdateNothing(t *testing.T) {
	h := claudeRaisedHost()
	h.install(t)
	for name, args := range map[string][]string{
		"no subcommand":      nil,
		"unknown subcommand": {"upgrade"},
		"unknown flag":       {"update", "--force"},
		"stray argument":     {"update", "claude"},
	} {
		t.Run(name, func(t *testing.T) {
			rc, _, stderr := runCLIUpdateVerb(t, args...)
			if rc != 2 || !strings.Contains(stderr, "evolve cli update") {
				t.Errorf("want rc 2 with the usage, got rc=%d stderr=%q", rc, stderr)
			}
		})
	}
	if len(h.calls) != 0 {
		t.Errorf("a usage error must not reach the updater: %q", h.calls)
	}
}

func TestCLIUpdate_IsTheCLICommandGroup(t *testing.T) {
	c := lookupCommand("cli")
	if c == nil || c.Run == nil || !strings.Contains(c.Summary, "update") {
		t.Fatalf("evolve cli is registered with an update summary: %+v", c)
	}
	if !strings.Contains(usage, "cli update") {
		t.Errorf("the usage listing names evolve cli update")
	}
}

func TestManifestUpdateFamilies_ReadsEachFamilysUpdateArgvFromItsManifest(t *testing.T) {
	var log bytes.Buffer
	got := manifestUpdateFamilies([]string{"claude", "codex", "agy"}, bridge.LoadManifest, &log)

	want := []cliupdate.Family{
		{Name: "claude", UpdateArgv: []string{"claude", "update"}},
		{Name: "codex"},
		{Name: "agy", UpdateArgv: []string{"agy", "update"}, AutoUpdateOffEnv: "AGY_CLI_DISABLE_AUTO_UPDATE"},
	}
	if !reflect.DeepEqual(got, want) || log.Len() != 0 {
		t.Errorf("families = %+v (log %q), want %+v", got, log.String(), want)
	}
}

func TestManifestUpdateFamilies_AnUnloadableManifestWarnsAndSkipsTheFamily(t *testing.T) {
	var log bytes.Buffer
	load := func(name string) (bridge.Manifest, error) {
		if name == "agy-tmux" {
			return bridge.Manifest{}, errors.New("invalid JSON")
		}
		return bridge.Manifest{UpdateArgv: []string{"claude", "update"}}, nil
	}

	got := manifestUpdateFamilies([]string{"claude", "agy"}, load, &log)

	if len(got) != 1 || got[0].Name != "claude" || !strings.Contains(log.String(), "agy") || !strings.Contains(log.String(), "invalid JSON") {
		t.Errorf("want claude only and a WARN naming agy: %+v %q", got, log.String())
	}
}

func TestCredentialWall_OnlyACredentialBenchIsAWall(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, nil)
	if _, err := store.BenchWall("codex", clihealth.CredentialPattern, "Please log in"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BenchWall("agy", "rate_limit", "quota reached"); err != nil {
		t.Fatal(err)
	}
	walled := credentialWall(store)

	if action, ok := walled("codex"); !ok || !strings.Contains(action, "logged in again") {
		t.Errorf("codex has a credential bench: ok=%v action=%q", ok, action)
	}
	if _, ok := walled("agy"); ok {
		t.Errorf("a quota bench is not a subscription wall")
	}
	if _, ok := walled("claude"); ok {
		t.Errorf("an unbenched family is not walled")
	}
}

func TestLiveCheck_ProbesTheFamilysTmuxDriverAndNamesAFailure(t *testing.T) {
	var probed []string
	check := liveCheck(func(context.Context) liveProbe {
		return func(driver string) (int, string, string) {
			probed = append(probed, driver)
			if driver == "codex-tmux" {
				return 1, "rate_limit", "You've hit your usage limit"
			}
			return bridge.ExitOK, "", ""
		}
	})

	if err := check(context.Background(), "claude"); err != nil {
		t.Errorf("a healthy probe passes, got %v", err)
	}
	err := check(context.Background(), "codex")
	if err == nil || !strings.Contains(err.Error(), "codex-tmux") || !strings.Contains(err.Error(), "rc=1") || !strings.Contains(err.Error(), "rate_limit") {
		t.Errorf("want an error naming the driver, rc and pattern, got %v", err)
	}
	if !reflect.DeepEqual(probed, []string{"claude-tmux", "codex-tmux"}) {
		t.Errorf("probed %q", probed)
	}
}

func TestCLIUpdate_RefusesWhileARunHoldsALiveLeaseUnlessDryRun(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	h := claudeRaisedHost()
	h.install(t)

	rc, _, stderr := runCLIUpdateVerb(t, "update", "--project-root", root)

	if rc != 2 || len(h.calls) != 0 || !strings.Contains(stderr, "cycle-9") || !strings.Contains(stderr, "within a wave") {
		t.Fatalf("a mid-wave update would pass cli-version-drift as expected; it must refuse with exit 2 naming the live run: rc=%d calls=%q stderr=%q", rc, h.calls, stderr)
	}
	if rc, _, _ := runCLIUpdateVerb(t, "update", "--dry-run", "--project-root", root); rc != 0 {
		t.Errorf("a dry run changes nothing, so it runs mid-wave: rc=%d", rc)
	}
}

func TestCLIUpdate_AnInterruptExitsOneThirtyNotOne(t *testing.T) {
	h := claudeRaisedHost()
	h.smokeErr = map[string]error{"claude": errors.New("signal: killed")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.onRun = cancel
	h.install(t)
	var out, errb bytes.Buffer

	rc := cliUpdateVerb(ctx, []string{"update", "--project-root", t.TempDir()}, &out, &errb)

	if rc != 130 || strings.Contains(out.String(), "smoke-failed") {
		t.Fatalf("SIGINT is an interrupt, not a failed update: rc=%d\n%s%s", rc, out.String(), errb.String())
	}
}
