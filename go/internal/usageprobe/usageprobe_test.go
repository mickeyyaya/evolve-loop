package usageprobe

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func newStore(t *testing.T) *clihealth.Store {
	t.Helper()
	return clihealth.NewStore(t.TempDir(), nil)
}

// recordingProbe captures which families were probed (concurrency-safe) and
// returns a scripted pane per family.
type recordingProbe struct {
	mu     sync.Mutex
	called []string
	pane   map[string]string
	err    map[string]error
}

func (r *recordingProbe) probe(_ context.Context, family string) (string, error) {
	r.mu.Lock()
	r.called = append(r.called, family)
	r.mu.Unlock()
	if r.err != nil {
		if e, ok := r.err[family]; ok {
			return "", e
		}
	}
	return r.pane[family], nil
}

func (r *recordingProbe) probedFamilies() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.called...)
	sort.Strings(out)
	return out
}

// TestProber_BenchesCappedFamily: a family whose usage pane classifies as
// exhausted is benched into the store so the dispatcher's pre-skip demotes it.
func TestProber_BenchesCappedFamily(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{pane: map[string]string{"codex": "5h limit: 0% left (resets 14:39)"}}
	p := &Prober{
		Families: []string{"codex"},
		Probe:    rp.probe,
		Classify: func(_, pane string) bool { return pane != "" }, // capped
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if _, ok := st.Active()["codex"]; !ok {
		t.Fatal("codex not benched after a capped usage probe")
	}
}

// TestProber_HealthyFamilyNotBenched: a healthy usage pane leaves the store
// empty (no false bench — worse than the status quo).
func TestProber_HealthyFamilyNotBenched(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{pane: map[string]string{"claude": "Usage: 12% of weekly limit (resets in 3 days)"}}
	p := &Prober{
		Families: []string{"claude"},
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return false }, // healthy
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if len(st.Active()) != 0 {
		t.Fatalf("healthy family benched: %v", st.Active())
	}
}

// TestProber_SkipsAlreadyBenched: a family with an ACTIVE bench is not re-probed
// (it is already pre-skipped; re-probing would re-boot a capped REPL).
func TestProber_SkipsAlreadyBenched(t *testing.T) {
	st := newStore(t)
	if _, err := st.BenchWall("codex", "rate_limit", "try again in 2 hours"); err != nil {
		t.Fatalf("pre-bench: %v", err)
	}
	rp := &recordingProbe{pane: map[string]string{"claude": "ok", "codex": "0% left"}}
	p := &Prober{
		Families: []string{"claude", "codex"},
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return false },
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if got := rp.probedFamilies(); len(got) != 1 || got[0] != "claude" {
		t.Errorf("probed=%v, want [claude] only (codex is actively benched)", got)
	}
}

// TestProber_UnsupportedFamilySkipped: ErrUnsupported is a silent no-op (ollama
// has no usage command), never a bench and never an error.
func TestProber_UnsupportedFamilySkipped(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{
		pane: map[string]string{},
		err:  map[string]error{"ollama": fmt.Errorf("probe: %w", clicontrol.ErrUnsupported)},
	}
	p := &Prober{
		Families: []string{"ollama"},
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return true }, // would bench if reached
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if len(st.Active()) != 0 {
		t.Fatalf("unsupported family benched: %v", st.Active())
	}
}

// TestProber_ProbeErrorFailOpen: a transport error neither benches nor panics —
// the probe is advisory, never allowed to break a cycle.
func TestProber_ProbeErrorFailOpen(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{err: map[string]error{"agy": fmt.Errorf("boot timeout")}}
	p := &Prober{
		Families: []string{"agy"},
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return true },
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if len(st.Active()) != 0 {
		t.Fatalf("errored probe benched: %v", st.Active())
	}
}

// TestProber_ConcurrentBenchesRaceSafe: all families probed in parallel and all
// capped ones benched — the flock-protected store accumulates every write. Run
// under -race to prove thread + process safety of the fan-out.
func TestProber_ConcurrentBenchesRaceSafe(t *testing.T) {
	st := newStore(t)
	fams := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	pane := map[string]string{}
	for _, f := range fams {
		pane[f] = "0% left"
	}
	rp := &recordingProbe{pane: pane}
	p := &Prober{
		Families: fams,
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return true },
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if got := len(st.Active()); got != len(fams) {
		t.Fatalf("benched %d families, want %d (lost a concurrent write)", got, len(fams))
	}
}
