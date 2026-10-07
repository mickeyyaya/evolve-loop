package usageprobe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

type evidenceRig struct {
	source *EvidenceSource
	store  *clihealth.Store
	probes *atomic.Int32
	clock  *time.Time
	dir    string
}

func newEvidenceRig(t *testing.T, windows []quotastate.UsageWindow, probeErr error, regexWall bool) *evidenceRig {
	t.Helper()
	now := probeNow
	r := &evidenceRig{probes: &atomic.Int32{}, clock: &now, dir: t.TempDir()}
	r.store = clihealth.NewStore(t.TempDir(), func() time.Time { return *r.clock })
	r.source = &EvidenceSource{
		Prober: &Prober{
			Probe: func(context.Context, string) (string, error) {
				r.probes.Add(1)
				return "the usage screen", probeErr
			},
			Windows:  func(string, string) []quotastate.UsageWindow { return windows },
			Classify: func(string, string) bool { return regexWall },
			Store:    r.store,
			Log:      &bytes.Buffer{},
		},
		EvolveDir: r.dir,
		Timeout:   time.Minute,
		TTL:       10 * time.Minute,
		Now:       func() time.Time { return *r.clock },
		Act:       true,
	}
	return r
}

func TestEvidence_AnExhaustedWindowIsAVerifiedQuotaCauseAndBenchesItsFamily(t *testing.T) {
	r := newEvidenceRig(t, agyWindows(100), nil, false)

	ev := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy-claude"})

	if ev.Verdict != VerdictExhausted || !strings.Contains(ev.Detail, "CLAUDE AND GPT MODELS 5h") || len(ev.Windows) != 2 {
		t.Fatalf("evidence = %+v; want exhausted on the agy-claude group's windows", ev)
	}
	if !strings.HasPrefix(ev.Summary(), "quota exhausted") {
		t.Errorf("Summary() = %q", ev.Summary())
	}
	if _, benched := r.store.Active()["agy-claude"]; !benched {
		t.Errorf("a verified exhausted window must bench its family until the reset: %v", r.store.Active())
	}
	if gemini := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy"}); gemini.Verdict != VerdictHealthy || !gemini.Cached || r.probes.Load() != 1 {
		t.Errorf("the Gemini family from the same screen = %+v after %d probe(s); want healthy from the cache", gemini, r.probes.Load())
	}
}

func TestEvidence_HealthyWindowsRuleQuotaOutAndNameTheWindows(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)

	ev := r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"})

	if ev.Verdict != VerdictHealthy || !strings.HasPrefix(ev.Summary(), "quota ruled out") || !strings.Contains(ev.Summary(), "session session window 7% used") {
		t.Fatalf("evidence = %+v, summary %q; want quota ruled out with the windows named", ev, ev.Summary())
	}
	if len(r.store.Active()) != 0 {
		t.Errorf("healthy usage benched %v", r.store.Active())
	}
}

func TestEvidence_AFailedUsageQueryIsUnavailableAndPointsAtAuthInstallOrNetwork(t *testing.T) {
	r := newEvidenceRig(t, nil, errors.New("REPL prompt never appeared after 60s"), false)

	ev := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy"})

	if ev.Verdict != VerdictUnavailable || !strings.Contains(ev.Summary(), "REPL prompt never appeared") || !strings.Contains(ev.Summary(), "auth, install or network") {
		t.Fatalf("evidence = %+v, summary %q", ev, ev.Summary())
	}
	if again := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy"}); !again.Cached || r.probes.Load() != 1 {
		t.Errorf("a failed query must be cached too, or a burst of failures storms the probe: %+v after %d probes", again, r.probes.Load())
	}
}

func TestEvidence_NoUsageCommandOrNoWindowIsUnknown(t *testing.T) {
	unsupported := newEvidenceRig(t, nil, fmt.Errorf("no usage: %w", clicontrol.ErrUnsupported), false)
	if ev := unsupported.source.Explain(context.Background(), Query{CLI: "ollama", Family: "ollama"}); ev.Verdict != VerdictUnknown {
		t.Errorf("no usage command = %+v, want unknown", ev)
	}
	unread := newEvidenceRig(t, nil, nil, false)
	if ev := unread.source.Explain(context.Background(), Query{CLI: "codex", Family: "codex"}); ev.Verdict != VerdictUnknown || !strings.HasPrefix(ev.Summary(), "quota could not be verified") {
		t.Errorf("a screen with no window = %+v, want unknown", ev)
	}
	otherFamily := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	if ev := otherFamily.source.Explain(context.Background(), Query{CLI: "claude", Family: "agy-claude"}); ev.Verdict != VerdictUnknown {
		t.Errorf("a screen with no window for the family = %+v, want unknown", ev)
	}
}

func TestEvidence_TheRegexFallbackStillVerifiesAWall(t *testing.T) {
	r := newEvidenceRig(t, nil, nil, true)

	ev := r.source.Explain(context.Background(), Query{CLI: "codex", Family: "codex"})

	if ev.Verdict != VerdictExhausted || !strings.Contains(ev.Detail, "exhausted_regex") {
		t.Fatalf("evidence = %+v; want exhausted through the manifest's regex", ev)
	}
	if _, benched := r.store.Active()["codex"]; !benched {
		t.Errorf("the regex wall did not bench codex")
	}
}

func TestEvidence_TheCacheLastsTheTTLAndIsSharedThroughTheRecordedWindows(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"})
	*r.clock = r.clock.Add(9 * time.Minute)
	r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"})
	if r.probes.Load() != 1 {
		t.Fatalf("%d probes inside the TTL, want 1", r.probes.Load())
	}
	*r.clock = r.clock.Add(2 * time.Minute)
	r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"})
	if r.probes.Load() != 2 {
		t.Fatalf("%d probes after the TTL, want 2", r.probes.Load())
	}

	other := newEvidenceRig(t, nil, errors.New("must not be probed"), false)
	other.source.EvolveDir = r.dir
	*other.clock = *r.clock
	if ev := other.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"}); ev.Verdict != VerdictHealthy || !ev.Cached || other.probes.Load() != 0 {
		t.Errorf("another process with the same evolve dir = %+v after %d probes; want the recorded windows reused", ev, other.probes.Load())
	}
}

func TestEvidence_TheQueryIsBoundedByItsTimeout(t *testing.T) {
	r := newEvidenceRig(t, nil, nil, false)
	r.source.Timeout = 90 * time.Second
	var remaining time.Duration
	r.source.Prober.Probe = func(ctx context.Context, _ string) (string, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return "", errors.New("the probe ran with no deadline")
		}
		remaining = time.Until(deadline)
		return "", context.DeadlineExceeded
	}

	ev := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy"})

	if remaining <= 89*time.Second || remaining > 90*time.Second {
		t.Fatalf("the probe's deadline was %v away; want the 90s Timeout", remaining)
	}
	if ev.Verdict != VerdictUnavailable || !strings.Contains(ev.Detail, "deadline exceeded") {
		t.Errorf("evidence = %+v; a timed-out query is unavailable", ev)
	}
}

func TestEvidence_AnObservationOlderThanTheFailureIsNotEvidenceForIt(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(100, 0), nil, false)
	before := Observation{CLI: "claude", ObservedAt: probeNow.Add(-time.Minute), Windows: claudeWindows(7, 0)}
	if err := RecordObservation(r.dir, before); err != nil {
		t.Fatal(err)
	}

	ev := r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude", Since: probeNow})

	if r.probes.Load() != 1 || ev.Verdict != VerdictExhausted {
		t.Fatalf("evidence %+v after %d probe(s); a screen read before the failing attempt started cannot rule its cause out, so the CLI is queried again", ev, r.probes.Load())
	}
	if again := r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude", Since: probeNow.Add(-time.Minute)}); !again.Cached || r.probes.Load() != 1 {
		t.Errorf("a failure that started before the fresh read reuses it: %+v after %d probes", again, r.probes.Load())
	}
}

func TestEvidence_AReadOnlySourceNeitherBenchesNorRecords(t *testing.T) {
	r := newEvidenceRig(t, agyWindows(100), nil, false)
	r.source.Act = false

	ev := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy-claude"})

	if ev.Verdict != VerdictExhausted || len(r.store.Active()) != 0 {
		t.Fatalf("evidence = %+v, benches %v; a read-only query reports and changes nothing", ev, r.store.Active())
	}
	if _, err := os.Stat(WindowsPath(r.dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a read-only query recorded its windows (stat err %v)", err)
	}
}

func TestEvidence_AnExhaustedPerModelWindowIsNotAVerifiedFamilyCause(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(10, 100), nil, false)

	ev := r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"})

	if ev.Verdict == VerdictExhausted || strings.Contains(ev.Summary(), "verified") {
		t.Fatalf("verdict %s, summary %q; one model's drained week must never be reported as the family's verified cause", ev.Verdict, ev.Summary())
	}
	if ev.Verdict != VerdictHealthy || !strings.Contains(ev.Detail, "Fable") || !strings.Contains(ev.Detail, "per-model") {
		t.Errorf("evidence %+v; want healthy for the family with the drained Fable window named as a per-model note", ev)
	}
	if len(r.store.Active()) != 0 {
		t.Errorf("a per-model window benched %v", r.store.Active())
	}
}

func TestEvidence_ASlowQueryForOneCLIDoesNotBlockAnotherAndOneCLIIsQueriedOnce(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	release, started := make(chan struct{}), make(chan struct{}, 1)
	var claudeProbes atomic.Int32
	r.source.Prober.Probe = func(_ context.Context, cli string) (string, error) {
		if cli == "claude" {
			claudeProbes.Add(1)
			started <- struct{}{}
			<-release
		}
		return "the usage screen", nil
	}
	results := make(chan Evidence, 2)
	go func() { results <- r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"}) }()
	<-started
	go func() { results <- r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude"}) }()

	done := make(chan Evidence, 1)
	go func() { done <- r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy"}) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("agy's usage query waited on claude's: the cache lock is held across a query")
	}
	close(release)
	for i := 0; i < 2; i++ {
		if ev := <-results; ev.Verdict != VerdictHealthy {
			t.Errorf("claude evidence %+v", ev)
		}
	}
	if claudeProbes.Load() != 1 {
		t.Errorf("%d claude probes for two concurrent failures; one query must serve both", claudeProbes.Load())
	}
}

func TestEvidence_TheRegexFallbackBenchesTheFailingFamilyNotTheWholeBinary(t *testing.T) {
	r := newEvidenceRig(t, nil, nil, true)

	ev := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy-claude"})

	active := r.store.Active()
	if _, ok := active["agy-claude"]; !ok || ev.Verdict != VerdictExhausted {
		t.Fatalf("benches %v, evidence %+v; want the failing agy-claude benched", active, ev)
	}
	if _, ok := active["agy"]; ok {
		t.Errorf("an agy-claude failure benched agy's Gemini family through the whole-screen regex")
	}
}

func TestEvidence_AFailedQueryReadBeforeTheFailureIsStillReusedSoADownCLIIsNotStormed(t *testing.T) {
	r := newEvidenceRig(t, nil, errors.New("must not be probed again"), false)
	down := Observation{CLI: "claude", ObservedAt: probeNow.Add(-time.Minute), Error: "REPL prompt never appeared after 60s"}
	if err := RecordObservation(r.dir, down); err != nil {
		t.Fatal(err)
	}

	ev := r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude", Since: probeNow})

	if r.probes.Load() != 0 || ev.Verdict != VerdictUnavailable || !ev.Cached {
		t.Fatalf("evidence %+v after %d probe(s); a failed query rules nothing out, so it is reused inside the TTL instead of re-booting a down CLI for every failing attempt", ev, r.probes.Load())
	}
}

func recordObservation(t *testing.T, dir string, obs Observation) {
	t.Helper()
	if err := RecordObservation(dir, obs); err != nil {
		t.Fatal(err)
	}
}

func TestEvidence_AReadIsReusedForLessThanTheTTLAndQueriedAgainAtExactlyTheTTL(t *testing.T) {
	for name, tc := range map[string]struct {
		age    time.Duration
		probes int32
	}{
		"one nanosecond inside the TTL": {age: 10*time.Minute - time.Nanosecond, probes: 0},
		"exactly the TTL old":           {age: 10 * time.Minute, probes: 1},
	} {
		t.Run(name, func(t *testing.T) {
			r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
			recordObservation(t, r.dir, Observation{CLI: "claude", ObservedAt: probeNow.Add(-tc.age), Windows: claudeWindows(7, 0)})

			r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude", Since: probeNow.Add(-time.Hour)})

			if r.probes.Load() != tc.probes {
				t.Errorf("a read %v old took %d probe(s), want %d", tc.age, r.probes.Load(), tc.probes)
			}
		})
	}
}

func TestEvidence_AHealthyReadTakenTheInstantTheFailureStartedDescribesIt(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(100, 0), nil, false)
	failureStart := probeNow.Add(-time.Minute)
	recordObservation(t, r.dir, Observation{CLI: "claude", ObservedAt: failureStart, Windows: claudeWindows(7, 0)})

	ev := r.source.Explain(context.Background(), Query{CLI: "claude", Family: "claude", Since: failureStart})

	if r.probes.Load() != 0 || ev.Verdict != VerdictHealthy || !ev.Cached {
		t.Fatalf("evidence %+v after %d probe(s); a read taken as the failure began already describes it", ev, r.probes.Load())
	}
}

func TestEvidence_AnExhaustedReadIsReusedUntilItsResetAndQueriedAgainOnceTheResetComes(t *testing.T) {
	for name, tc := range map[string]struct {
		reset   time.Duration
		probes  int32
		verdict Verdict
	}{
		"the reset is still ahead": {reset: 3 * time.Minute, probes: 0, verdict: VerdictExhausted},
		"the reset is now":         {reset: 0, probes: 1, verdict: VerdictHealthy},
		"the reset has passed":     {reset: -3 * time.Minute, probes: 1, verdict: VerdictHealthy},
	} {
		t.Run(name, func(t *testing.T) {
			r := newEvidenceRig(t, agyWindows(40), nil, false)
			reset := probeNow.Add(tc.reset)
			recordObservation(t, r.dir, Observation{CLI: "agy", ObservedAt: probeNow.Add(-6 * time.Minute), Windows: []quotastate.UsageWindow{
				{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindWeek, PercentUsed: 27.32, Family: "agy-claude", ResetsAt: resetIn(163 * time.Hour)},
				{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindFiveHour, PercentUsed: 100, Exhausted: true, Family: "agy-claude", ResetsAt: &reset},
			}})

			ev := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy-claude", Since: probeNow.Add(-time.Second)})

			if r.probes.Load() != tc.probes || ev.Verdict != tc.verdict {
				t.Errorf("evidence %+v after %d probe(s); want %s after %d", ev, r.probes.Load(), tc.verdict, tc.probes)
			}
		})
	}
}

func TestEvidence_AReusedRegexWallVerifiesOnlyTheFamilyItWasReadFor(t *testing.T) {
	r := newEvidenceRig(t, nil, nil, true)
	r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy-claude"})

	sibling := r.source.Explain(context.Background(), Query{CLI: "agy", Family: "agy"})

	if sibling.Verdict == VerdictExhausted || r.probes.Load() != 1 {
		t.Fatalf("the Gemini family got %+v after %d probe(s); the regex wall was read and benched for agy-claude only", sibling, r.probes.Load())
	}
	if _, benched := r.store.Active()["agy"]; benched {
		t.Errorf("agy was benched by agy-claude's regex wall")
	}
}
