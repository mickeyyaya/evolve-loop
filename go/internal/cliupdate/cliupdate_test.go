package cliupdate_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

type fakeHost struct {
	versions   map[string][]string
	versionErr map[string][]error
	runErr     map[string]error
	probeErr   map[string]error
	smokeErr   map[string]error
	ineligible map[string]string
	onProbe    func()
	onRun      func()
	onSmoke    func()
	calls      []string
}

func (h *fakeHost) seams() cliupdate.Seams {
	return cliupdate.Seams{
		Eligible: func(family string) (bool, string) {
			h.calls = append(h.calls, "eligible "+family)
			why, no := h.ineligible[family]
			return !no, why
		},
		Probe: func(_ context.Context, family string) error {
			h.calls = append(h.calls, "probe "+family)
			if h.onProbe != nil {
				h.onProbe()
			}
			return h.probeErr[family]
		},
		Version: func(bin string) (string, error) {
			h.calls = append(h.calls, "version "+bin)
			return h.nextVersion(bin)
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
				h.onSmoke()
			}
			return h.smokeErr[family]
		},
	}
}

func (h *fakeHost) nextVersion(bin string) (string, error) {
	var err error
	if errs := h.versionErr[bin]; len(errs) > 0 {
		err, h.versionErr[bin] = errs[0], errs[1:]
	}
	vs := h.versions[bin]
	if len(vs) == 0 {
		return "", err
	}
	v := vs[0]
	if len(vs) > 1 {
		h.versions[bin] = vs[1:]
	}
	return v, err
}

func (h *fakeHost) called(prefix string) bool {
	for _, c := range h.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func claude() cliupdate.Family {
	return cliupdate.Family{Name: "claude", UpdateArgv: []string{"claude", "update"}}
}

func agy() cliupdate.Family {
	return cliupdate.Family{Name: "agy", UpdateArgv: []string{"agy", "update"}, AutoUpdateOffEnv: "AGY_CLI_DISABLE_AUTO_UPDATE"}
}

func agyLastSeenAt(version string) []cliupdate.Record {
	return []cliupdate.Record{
		{Family: "claude", Kind: cliupdate.KindBaseline, New: "2.1.285"},
		{Family: "agy", Kind: cliupdate.KindBaseline, New: "1.2.14"},
		{Family: "agy", Kind: cliupdate.KindUpdate, Old: "1.2.14", New: version},
	}
}

func only(t *testing.T, rep cliupdate.Report) cliupdate.Result {
	t.Helper()
	if len(rep.Results) != 1 {
		t.Fatalf("want one result, got %+v", rep.Results)
	}
	return rep.Results[0]
}

func TestUpdate_ARaisedVersionRecordsBothVersionsAndRunsTheSmoke(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil))

	if res.Status != cliupdate.StatusUpdated || res.OldVersion != "2.1.285" || res.NewVersion != "2.1.286" {
		t.Fatalf("want updated 2.1.285 → 2.1.286, got %+v", res)
	}
	want := []string{"eligible claude", "version claude", "probe claude", "run claude update", "version claude", "smoke claude"}
	if !reflect.DeepEqual(h.calls, want) {
		t.Errorf("calls = %q, want %q", h.calls, want)
	}
}

func TestUpdate_AnUnchangedVersionSkipsTheSmoke(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285"}}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil))

	if res.Status != cliupdate.StatusUnchanged || res.OldVersion != "2.1.285" || res.NewVersion != "2.1.285" {
		t.Fatalf("want unchanged at 2.1.285, got %+v", res)
	}
	if h.called("smoke") {
		t.Errorf("an unchanged CLI was smoke-tested: %q", h.calls)
	}
}

func TestUpdate_AFamilyWithoutAnUpdaterIsReportedNotFailed(t *testing.T) {
	h := &fakeHost{}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{{Name: "codex"}}, h.seams(), nil)

	res := only(t, rep)
	if res.Status != cliupdate.StatusNoUpdater || !strings.Contains(res.Detail, "update_argv") {
		t.Fatalf("want no-updater naming update_argv, got %+v", res)
	}
	if len(rep.Failed()) != 0 || len(h.calls) != 0 {
		t.Errorf("a family without an updater is neither failed nor probed: failed=%+v calls=%q", rep.Failed(), h.calls)
	}
}

func TestUpdate_AnIneligibleOrWalledFamilyIsNeverUpdated(t *testing.T) {
	h := &fakeHost{ineligible: map[string]string{"agy": "credential wall on the cli-health bench"}}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{agy()}, h.seams(), agyLastSeenAt("1.2.16"))

	res := only(t, rep)
	if res.Status != cliupdate.StatusSkipped || res.Detail != "credential wall on the cli-health bench" {
		t.Fatalf("want skipped with the selection reason, got %+v", res)
	}
	if want := []string{"eligible agy"}; !reflect.DeepEqual(h.calls, want) {
		t.Errorf("an ineligible family's version, probe, updater or smoke ran: %q", h.calls)
	}
	if len(rep.Failed()) != 0 {
		t.Errorf("a skipped family is not a failure: %+v", rep.Failed())
	}
}

func TestUpdate_AFamilyWhoseDoctorLiveProbeFailsIsSkippedWithoutAnUpdate(t *testing.T) {
	h := &fakeHost{
		versions: map[string][]string{"codex": {"0.153.4"}},
		probeErr: map[string]error{"codex": errors.New("doctor live codex-tmux rc=1 pattern=\"rate_limit\"")},
	}
	codex := cliupdate.Family{Name: "codex", UpdateArgv: []string{"codex", "update"}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{codex}, h.seams(), nil))

	if res.Status != cliupdate.StatusSkipped || !strings.Contains(res.Detail, "unsubscribed") || !strings.Contains(res.Detail, "rate_limit") {
		t.Fatalf("a family whose probe fails counts as unsubscribed: %+v", res)
	}
	if h.called("run") || h.called("smoke") {
		t.Errorf("an unsubscribed family is never updated or smoke-tested: %q", h.calls)
	}
}

func TestUpdate_AFoundChangeIsItsOwnProbe_ABrokenSelfUpdateHaltsInsteadOfBeingSkipped(t *testing.T) {
	h := &fakeHost{
		versions: map[string][]string{"agy": {"1.2.17"}},
		probeErr: map[string]error{"agy": errors.New("doctor live agy-tmux rc=80")},
		smokeErr: map[string]error{"agy": errors.New("doctor live agy-tmux rc=80")},
	}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{agy()}, h.seams(), agyLastSeenAt("1.2.16"))

	res := only(t, rep)
	if res.Status != cliupdate.StatusSmokeFailed || len(rep.SmokeFailed()) != 1 || !strings.Contains(res.Detail, "outside a boundary") {
		t.Fatalf("agy moved 1.2.16 → 1.2.17 and does not boot; that halts the next wave, it is not a skip: %+v", res)
	}
	if h.called("probe") || h.called("run") {
		t.Errorf("the found-change smoke is the probe, and a broken CLI is not updated: %q", h.calls)
	}
}

func TestUpdate_ASmokeFailureIsReportedAsTheHaltingFailure(t *testing.T) {
	h := &fakeHost{
		versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}},
		smokeErr: map[string]error{"claude": errors.New("doctor live claude-tmux rc=80")},
	}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil)

	res := only(t, rep)
	if res.Status != cliupdate.StatusSmokeFailed || res.NewVersion != "2.1.286" || !strings.Contains(res.Detail, "rc=80") {
		t.Fatalf("want smoke-failed at 2.1.286 naming the probe, got %+v", res)
	}
	if len(rep.Failed()) != 1 || len(rep.SmokeFailed()) != 1 {
		t.Errorf("a smoke failure is both a failure and a smoke failure: failed=%+v smoke=%+v", rep.Failed(), rep.SmokeFailed())
	}
}

func TestUpdate_AnUpdaterErrorStillSmokesTheInstallItTouched(t *testing.T) {
	h := &fakeHost{
		versions: map[string][]string{"claude": {"2.1.285"}},
		runErr:   map[string]error{"claude": errors.New("network unreachable")},
	}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil)

	res := only(t, rep)
	if res.Status != cliupdate.StatusUpdateFailed || !strings.Contains(res.Detail, "network unreachable") || !strings.Contains(res.Detail, "smoke OK") {
		t.Fatalf("want update-failed naming the updater error and the passing smoke, got %+v", res)
	}
	if h.calls[len(h.calls)-1] != "smoke claude" {
		t.Errorf("a failed updater must still smoke the install it touched: %q", h.calls)
	}
	if len(rep.Failed()) != 1 || len(rep.SmokeFailed()) != 0 {
		t.Errorf("an updater error with a healthy CLI fails the verb but does not halt a wave: failed=%+v smoke=%+v", rep.Failed(), rep.SmokeFailed())
	}
}

func TestUpdate_AnUpdaterErrorWhoseSmokeFailsIsASmokeFailure(t *testing.T) {
	h := &fakeHost{
		versions: map[string][]string{"claude": {"2.1.285"}},
		runErr:   map[string]error{"claude": errors.New("partial download")},
		smokeErr: map[string]error{"claude": errors.New("REPL never booted")},
	}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil))

	if res.Status != cliupdate.StatusSmokeFailed || !strings.Contains(res.Detail, "partial download") || !strings.Contains(res.Detail, "REPL never booted") {
		t.Fatalf("want smoke-failed naming both errors, got %+v", res)
	}
}

func TestUpdate_AnUnreadableVersionBeforeTheUpdateIsSmokedAndRunsNoUpdater(t *testing.T) {
	h := &fakeHost{versionErr: map[string][]error{"claude": {errors.New("claude --version: exit 1")}}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil))

	if res.Status != cliupdate.StatusUpdateFailed || !strings.Contains(res.Detail, "before the update") || !strings.Contains(res.Detail, "smoke OK") {
		t.Fatalf("want update-failed naming the version probe and the passing smoke, got %+v", res)
	}
	if want := []string{"eligible claude", "version claude", "smoke claude"}; !reflect.DeepEqual(h.calls, want) {
		t.Errorf("an unreadable starting version is smoke-tested and the updater does not run: %q", h.calls)
	}
}

func TestUpdate_AnUnreadableVersionWhoseSmokeFailsHalts(t *testing.T) {
	h := &fakeHost{
		versionErr: map[string][]error{"claude": {errors.New("claude --version: killed")}},
		smokeErr:   map[string]error{"claude": errors.New("rc=80")},
	}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil)

	if res := only(t, rep); res.Status != cliupdate.StatusSmokeFailed || len(rep.SmokeFailed()) != 1 || !strings.Contains(res.Detail, "before the update") {
		t.Fatalf("a CLI whose version cannot be read and which does not boot halts the next wave: %+v", res)
	}
}

func TestUpdate_AnUnreadableVersionAfterTheUpdateIsSmokedAndFailed(t *testing.T) {
	h := &fakeHost{
		versions:   map[string][]string{"claude": {"2.1.285", ""}},
		versionErr: map[string][]error{"claude": {nil, errors.New("claude --version: no such file")}},
	}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, h.seams(), nil))

	if res.Status != cliupdate.StatusUpdateFailed || res.NewVersion != "" || !strings.Contains(res.Detail, "after the update") {
		t.Fatalf("want update-failed naming the unreadable new version, got %+v", res)
	}
	if h.calls[len(h.calls)-1] != "smoke claude" {
		t.Errorf("an unreadable new version must be smoke-tested: %q", h.calls)
	}
}

func TestUpdate_ACancelledContextUpdatesNoFamily(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := only(t, cliupdate.Update(ctx, []cliupdate.Family{claude()}, h.seams(), nil))

	if res.Status != cliupdate.StatusSkipped || !strings.HasPrefix(res.Detail, "interrupted") || len(h.calls) != 0 {
		t.Fatalf("a cancelled boundary updates nothing: res=%+v calls=%q", res, h.calls)
	}
}

func TestUpdate_AnInterruptIsNeverASmokeFailure(t *testing.T) {
	cases := map[string]func(h *fakeHost, cancel context.CancelFunc){
		"during the probe": func(h *fakeHost, cancel context.CancelFunc) {
			h.versions = map[string][]string{"claude": {"2.1.285"}}
			h.probeErr = map[string]error{"claude": errors.New("signal: killed")}
			h.onProbe = cancel
		},
		"during the updater": func(h *fakeHost, cancel context.CancelFunc) {
			h.versions = map[string][]string{"claude": {"2.1.285", "2.1.286"}}
			h.onRun = cancel
		},
		"during the smoke": func(h *fakeHost, cancel context.CancelFunc) {
			h.versions = map[string][]string{"claude": {"2.1.285", "2.1.286"}}
			h.smokeErr = map[string]error{"claude": errors.New("signal: killed")}
			h.onSmoke = cancel
		},
		"during the found-change smoke": func(h *fakeHost, cancel context.CancelFunc) {
			h.versions = map[string][]string{"claude": {"2.1.286"}}
			h.smokeErr = map[string]error{"claude": errors.New("signal: killed")}
			h.onSmoke = cancel
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			h := &fakeHost{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			arrange(h, cancel)
			history := []cliupdate.Record{{Family: "claude", Kind: cliupdate.KindBaseline, New: "2.1.285"}}

			rep := cliupdate.Update(ctx, []cliupdate.Family{claude()}, h.seams(), history)

			res := only(t, rep)
			if res.Status != cliupdate.StatusSkipped || !strings.HasPrefix(res.Detail, "interrupted") || len(rep.SmokeFailed()) != 0 || len(rep.Failed()) != 0 {
				t.Fatalf("an interrupt is skipped: interrupted, never a failure or a halt: %+v", res)
			}
		})
	}
}

func TestUpdate_RunsEveryFamilyInOrder(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285"}, "agy": {"1.0.20", "1.0.21"}}}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{claude(), {Name: "ollama"}, agy()}, h.seams(), nil)

	var got []string
	for _, r := range rep.Results {
		got = append(got, r.Family+"="+string(r.Status))
	}
	if want := []string{"claude=unchanged", "ollama=no-updater", "agy=updated"}; !reflect.DeepEqual(got, want) || rep.DryRun {
		t.Errorf("results = %q dry=%v, want %q", got, rep.DryRun, want)
	}
}

func TestPlan_NamesWhatWouldRunAndMarksTheReportDry(t *testing.T) {
	rep := cliupdate.Plan([]cliupdate.Family{claude(), {Name: "codex"}})

	if !rep.DryRun || len(rep.Results) != 2 {
		t.Fatalf("want a dry report with two results, got %+v", rep)
	}
	planned, none := rep.Results[0], rep.Results[1]
	if planned.Status != cliupdate.StatusPlanned || !reflect.DeepEqual(planned.Argv, []string{"claude", "update"}) || !strings.Contains(planned.Detail, "claude update") {
		t.Errorf("claude: want planned naming its updater, got %+v", planned)
	}
	if none.Status != cliupdate.StatusNoUpdater {
		t.Errorf("codex: want no-updater, got %+v", none)
	}
	if len(rep.Failed()) != 0 {
		t.Errorf("a plan never fails: %+v", rep.Failed())
	}
}

func TestResultLine_SaysWhatHappenedToEachFamily(t *testing.T) {
	cases := []struct {
		res  cliupdate.Result
		want string
	}{
		{cliupdate.Result{Family: "claude", Status: cliupdate.StatusUpdated, OldVersion: "2.1.285", NewVersion: "2.1.286"}, "claude updated 2.1.285 → 2.1.286"},
		{cliupdate.Result{Family: "agy", Status: cliupdate.StatusUnchanged, OldVersion: "1.0.20", NewVersion: "1.0.20"}, "agy unchanged 1.0.20"},
		{cliupdate.Result{Family: "codex", Status: cliupdate.StatusSkipped, Detail: "credential wall"}, "codex skipped: credential wall"},
		{cliupdate.Result{Family: "claude", Status: cliupdate.StatusUpdateFailed, OldVersion: "2.1.285", Detail: "updater: x; smoke OK"}, "claude update-failed 2.1.285: updater: x; smoke OK"},
	}
	for _, tc := range cases {
		if got := tc.res.Line(); got != tc.want {
			t.Errorf("Line() = %q, want %q", got, tc.want)
		}
	}
}

func TestResultVersions_ShowsTheChangeOrTheOneVersionSeen(t *testing.T) {
	cases := map[string]cliupdate.Result{
		"2.1.285 → 2.1.286": {OldVersion: "2.1.285", NewVersion: "2.1.286"},
		"2.1.285":           {OldVersion: "2.1.285", NewVersion: "2.1.285"},
		"1.0.20":            {OldVersion: "1.0.20"},
		"":                  {Status: cliupdate.StatusNoUpdater},
	}
	for want, res := range cases {
		if got := res.Versions(); got != want {
			t.Errorf("Versions(%+v) = %q, want %q", res, got, want)
		}
	}
}

func TestUnwalled_ACredentialWallMakesAFamilyIneligible(t *testing.T) {
	eligible := cliupdate.Unwalled(func(family string) (string, bool) {
		return "operator: log the " + family + " CLI in again", family == "codex"
	})

	if ok, why := eligible("codex"); ok || !strings.Contains(why, "credential wall") || !strings.Contains(why, "log the codex CLI in") {
		t.Fatalf("a walled family is ineligible with the bench's action: ok=%v why=%q", ok, why)
	}
	if ok, why := eligible("claude"); !ok || why != "" {
		t.Fatalf("an unbenched family is eligible: ok=%v why=%q", ok, why)
	}
}

func TestUpdate_AnUnrecordedVersionChangeIsSmokeBootedBeforeTheUpdaterRuns(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"agy": {"1.2.17"}}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{agy()}, h.seams(), agyLastSeenAt("1.2.16")))

	want := []string{"eligible agy", "version agy", "smoke agy", "run agy update", "version agy"}
	if !reflect.DeepEqual(h.calls, want) {
		t.Fatalf("a version the last boundary did not record is smoke-booted before anything else runs it: calls=%q, want %q", h.calls, want)
	}
	if res.Status != cliupdate.StatusSelfUpdated || res.SelfUpdatedFrom != "1.2.16" || res.OldVersion != "1.2.17" || res.NewVersion != "1.2.17" {
		t.Fatalf("want self-updated 1.2.16 → 1.2.17, got %+v", res)
	}
	if got := res.Line(); got != "agy self-updated 1.2.16 → 1.2.17: outside a boundary; smoke OK" {
		t.Errorf("Line() = %q", got)
	}
}

func TestUpdate_ASelfUpdateFollowedByABoundaryUpdateNamesBoth(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"agy": {"1.2.17", "1.2.18"}}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{agy()}, h.seams(), agyLastSeenAt("1.2.16")))

	if res.Status != cliupdate.StatusUpdated || res.SelfUpdatedFrom != "1.2.16" || res.NewVersion != "1.2.18" {
		t.Fatalf("want updated with the self-update kept, got %+v", res)
	}
	if got := res.Line(); got != "agy updated 1.2.16 → 1.2.18: self-updated to 1.2.17 outside a boundary; smoke OK" {
		t.Errorf("Line() = %q", got)
	}
	if smokes := strings.Count(strings.Join(h.calls, ","), "smoke agy"); smokes != 2 {
		t.Errorf("both the found version and the updated one are smoke-booted: %q", h.calls)
	}
}

func TestUpdate_AVersionTheLastBoundaryRecordedIsNoSelfUpdate(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"agy": {"1.2.16"}}}

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{agy()}, h.seams(), agyLastSeenAt("1.2.16")))

	if res.Status != cliupdate.StatusUnchanged || res.SelfUpdatedFrom != "" || h.called("smoke") {
		t.Fatalf("a recorded version is not a self-update and needs no smoke: %+v calls=%q", res, h.calls)
	}
}
