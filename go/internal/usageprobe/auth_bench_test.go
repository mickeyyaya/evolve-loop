package usageprobe

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func authBenchedProber(t *testing.T, root string, windows []quotastate.UsageWindow, probeErr error) (*Prober, *clihealth.Store, *[]signalcenter.Event) {
	t.Helper()
	st := clihealth.NewStore(root, nil)
	if _, err := st.BenchWall("claude", clihealth.CredentialPattern, "Please log in"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BenchWall("codex", "rate_limit", "try again in 2 hours"); err != nil {
		t.Fatal(err)
	}
	rp := &recordingProbe{pane: map[string]string{"claude": "usage", "codex": "usage"}}
	if probeErr != nil {
		rp.err = map[string]error{"claude": probeErr}
	}
	signals := signalcenter.New()
	got := &[]signalcenter.Event{}
	signals.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return &Prober{
		Families: []string{"claude", "codex"},
		Probe:    rp.probe,
		Windows:  func(string, string) []quotastate.UsageWindow { return windows },
		Store:    st,
		Signals:  signals,
		Log:      io.Discard,
	}, st, got
}

func TestProber_AUsageReadWithWindowsClearsTheAuthBench(t *testing.T) {
	t.Parallel()
	p, st, got := authBenchedProber(t, t.TempDir(), []quotastate.UsageWindow{{Scope: "session", Kind: "5h", PercentUsed: 12, Family: "claude"}}, nil)
	log := new(strings.Builder)
	p.Log = log

	p.Run(context.Background())

	active := st.Active()
	if _, benched := active["claude"]; benched {
		t.Fatalf("the claude auth bench stayed after its usage screen was read: %+v", active["claude"])
	}
	if _, benched := active["codex"]; !benched {
		t.Fatal("the codex rate-limit bench is gone; the usage probe never probes a quota-benched family")
	}
	if len(*got) != 1 || (*got)[0].Code != clihealth.CodeAuthBenchCleared || (*got)[0].Fields["source"] != string(clihealth.SourceUsageProbe) {
		t.Fatalf("events=%+v; want one auth-bench-cleared signal from the usage probe", *got)
	}
	if !strings.Contains(log.String(), "claude: its usage screen was read: the auth_recheck bench claude is cleared") {
		t.Fatalf("log=%q; want the clear on the log", log.String())
	}
}

func TestProber_AFailedOrWindowlessUsageReadKeepsTheAuthBench(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		windows []quotastate.UsageWindow
		err     error
	}{
		"no windows":   {},
		"probe failed": {err: os.ErrDeadlineExceeded},
	} {
		p, st, got := authBenchedProber(t, t.TempDir(), tc.windows, tc.err)
		p.Run(context.Background())
		if _, benched := st.Active()["claude"]; !benched {
			t.Errorf("%s: the auth bench is gone", name)
		}
		if len(*got) != 0 {
			t.Errorf("%s: events=%+v", name, *got)
		}
	}
}

func TestProber_AClearFailureIsLogged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p, _, _ := authBenchedProber(t, root, []quotastate.UsageWindow{{Scope: "session", Kind: "5h", PercentUsed: 12, Family: "claude"}}, nil)
	log := new(strings.Builder)
	p.Log = log
	dir := filepath.Join(root, ".evolve")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	p.clearAuth("claude")

	if !strings.Contains(log.String(), "WARN claude") {
		t.Fatalf("log=%q; want a WARN that names the family", log.String())
	}
}
