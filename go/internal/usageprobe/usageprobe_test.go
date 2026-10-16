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

func TestProber_BenchesCappedFamily(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{pane: map[string]string{"codex": "5h limit: 0% left (resets 14:39)"}}
	p := &Prober{
		Families: []string{"codex"},
		Probe:    rp.probe,
		Classify: func(_, pane string) bool { return pane != "" },
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if _, ok := st.Active()["codex"]; !ok {
		t.Fatal("codex not benched after a capped usage probe")
	}
}

func TestProber_HealthyFamilyNotBenched(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{pane: map[string]string{"claude": "Usage: 12% of weekly limit (resets in 3 days)"}}
	p := &Prober{
		Families: []string{"claude"},
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return false },
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if len(st.Active()) != 0 {
		t.Fatalf("healthy family benched: %v", st.Active())
	}
}

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

func TestProber_UnsupportedFamilySkipped(t *testing.T) {
	st := newStore(t)
	rp := &recordingProbe{
		pane: map[string]string{},
		err:  map[string]error{"ollama": fmt.Errorf("probe: %w", clicontrol.ErrUnsupported)},
	}
	p := &Prober{
		Families: []string{"ollama"},
		Probe:    rp.probe,
		Classify: func(_, _ string) bool { return true },
		Store:    st,
		Log:      io.Discard,
	}
	p.Run(context.Background())
	if len(st.Active()) != 0 {
		t.Fatalf("unsupported family benched: %v", st.Active())
	}
}

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
