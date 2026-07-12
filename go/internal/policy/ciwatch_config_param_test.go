package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCIWatchPolicy writes a policy.json document and loads it, so the knobs
// are proven to resolve from an on-disk policy.json — not from Go literals.
func loadCIWatchPolicy(t *testing.T, body string) Policy {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	return p
}

// TestCIWatchPolicy_KnobsFromPolicyJSON pins the AC4 contract for
// push-ci-watch-remote-parity (cycle-748): every knob (enabled, timeout, poll
// interval) resolves from a policy.json ci_watch block; an ABSENT block yields
// the compiled defaults (watch enabled — gates default ON as compiled Go
// defaults, the observer/fleet pattern); malformed values are rejected
// explicitly rather than silently zeroed.
func TestCIWatchPolicy_KnobsFromPolicyJSON(t *testing.T) {
	t.Run("absent block resolves compiled defaults", func(t *testing.T) {
		p := loadCIWatchPolicy(t, `{}`)
		cfg, err := p.CIWatchConfig()
		if err != nil {
			t.Fatalf("CIWatchConfig: %v", err)
		}
		if cfg.Enabled == nil || !*cfg.Enabled {
			t.Errorf("Enabled = %v, want compiled default true", cfg.Enabled)
		}
		if cfg.TimeoutS == nil || *cfg.TimeoutS != 900 {
			t.Errorf("TimeoutS = %v, want compiled default 900", cfg.TimeoutS)
		}
		if cfg.PollS == nil || *cfg.PollS != 30 {
			t.Errorf("PollS = %v, want compiled default 30", cfg.PollS)
		}
	})

	t.Run("present block overrides every knob", func(t *testing.T) {
		p := loadCIWatchPolicy(t, `{"ci_watch":{"enabled":false,"timeout_s":120,"poll_s":5}}`)
		cfg, err := p.CIWatchConfig()
		if err != nil {
			t.Fatalf("CIWatchConfig: %v", err)
		}
		if cfg.Enabled == nil || *cfg.Enabled {
			t.Errorf("Enabled = %v, want explicit false override", cfg.Enabled)
		}
		if cfg.TimeoutS == nil || *cfg.TimeoutS != 120 {
			t.Errorf("TimeoutS = %v, want 120", cfg.TimeoutS)
		}
		if cfg.PollS == nil || *cfg.PollS != 5 {
			t.Errorf("PollS = %v, want 5", cfg.PollS)
		}
	})

	t.Run("partial block keeps defaults for omitted knobs", func(t *testing.T) {
		p := loadCIWatchPolicy(t, `{"ci_watch":{"poll_s":10}}`)
		cfg, err := p.CIWatchConfig()
		if err != nil {
			t.Fatalf("CIWatchConfig: %v", err)
		}
		if cfg.Enabled == nil || !*cfg.Enabled {
			t.Errorf("Enabled = %v, want default true when omitted", cfg.Enabled)
		}
		if cfg.TimeoutS == nil || *cfg.TimeoutS != 900 {
			t.Errorf("TimeoutS = %v, want default 900 when omitted", cfg.TimeoutS)
		}
		if cfg.PollS == nil || *cfg.PollS != 10 {
			t.Errorf("PollS = %v, want 10", cfg.PollS)
		}
	})

	t.Run("programmatic override block resolves identically", func(t *testing.T) {
		enabled, timeout := false, 60
		p := Policy{CIWatch: &CIWatchPolicy{Enabled: &enabled, TimeoutS: &timeout}}
		cfg, err := p.CIWatchConfig()
		if err != nil {
			t.Fatalf("CIWatchConfig: %v", err)
		}
		if *cfg.Enabled || *cfg.TimeoutS != 60 || *cfg.PollS != 30 {
			t.Errorf("resolved = {enabled:%v timeout:%d poll:%d}, want false/60/30", *cfg.Enabled, *cfg.TimeoutS, *cfg.PollS)
		}
	})

	t.Run("malformed values rejected explicitly not silently zeroed", func(t *testing.T) {
		for name, body := range map[string]string{
			"negative timeout": `{"ci_watch":{"timeout_s":-1}}`,
			"zero poll":        `{"ci_watch":{"poll_s":0}}`,
		} {
			p := loadCIWatchPolicy(t, body)
			if _, err := p.CIWatchConfig(); err == nil {
				t.Errorf("%s: CIWatchConfig accepted malformed value, want explicit rejection", name)
			} else if !strings.Contains(err.Error(), "ci_watch") {
				t.Errorf("%s: error %v should name the ci_watch knob", name, err)
			}
		}
	})
}
