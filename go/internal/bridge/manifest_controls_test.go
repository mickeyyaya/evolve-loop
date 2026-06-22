package bridge

import (
	"regexp"
	"testing"
)

// TestManifestControls_RealManifests locks the per-CLI mapping table the probe
// depends on: every embedded interactive manifest resolves the abstract control
// events to the researched concrete commands, every usage exhausted_regex
// compiles, and ollama (a local model with no quota) correctly reports
// usage-unsupported while still mapping the universal clean_ctx → /clear.
func TestManifestControls_RealManifests(t *testing.T) {
	type want struct {
		event, send string
	}
	cases := map[string][]want{
		"claude-tmux": {{"usage", "/usage"}, {"status", "/status"}, {"clean_ctx", "/clear"}},
		"codex-tmux":  {{"usage", "/status"}, {"status", "/status"}, {"clean_ctx", "/clear"}},
		"agy-tmux":    {{"usage", "/usage"}, {"status", "/usage"}, {"clean_ctx", "/clear"}},
		"ollama-tmux": {{"status", "/show info"}, {"clean_ctx", "/clear"}},
	}
	for cli, wants := range cases {
		cli, wants := cli, wants
		t.Run(cli, func(t *testing.T) {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%q): %v", cli, err)
			}
			for _, w := range wants {
				spec, ok := m.Control(w.event)
				if !ok {
					t.Errorf("Control(%q) not found, want send=%q", w.event, w.send)
					continue
				}
				if spec.Send != w.send {
					t.Errorf("Control(%q).Send=%q, want %q", w.event, spec.Send, w.send)
				}
				if spec.ExhaustedRegex != "" {
					if _, err := regexp.Compile(spec.ExhaustedRegex); err != nil {
						t.Errorf("Control(%q).ExhaustedRegex does not compile: %v", w.event, err)
					}
				}
			}
		})
	}

	t.Run("ollama usage is unsupported (local, no quota)", func(t *testing.T) {
		m, err := LoadManifest("ollama-tmux")
		if err != nil {
			t.Fatalf("LoadManifest: %v", err)
		}
		if _, ok := m.Control("usage"); ok {
			t.Error("ollama Control(usage) ok=true, want false (no quota command)")
		}
	})
}

// TestManifestControl_Resolution verifies the per-CLI control mapping table:
// the `controls` manifest block parses into Controls, and Control(event)
// resolves a present event to its ControlSpec and reports a missing event
// (or a manifest with no controls at all) as not-found — the ErrUnsupported
// signal the abstract Controller turns into a clean no-op.
func TestManifestControl_Resolution(t *testing.T) {
	data := []byte(`{
		"cli": "fake-tmux",
		"binary": "fake",
		"controls": {
			"usage":     { "send": "/usage", "await": "prompt_marker", "exhausted_regex": "(?i)usage limit reached" },
			"clean_ctx": { "send": "/clear" }
		}
	}`)
	m, err := parseManifest("fake-tmux", data)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}

	t.Run("present event resolves full spec", func(t *testing.T) {
		spec, ok := m.Control("usage")
		if !ok {
			t.Fatal("Control(usage) ok=false, want true")
		}
		if spec.Send != "/usage" {
			t.Errorf("Send=%q, want /usage", spec.Send)
		}
		if spec.Await != "prompt_marker" {
			t.Errorf("Await=%q, want prompt_marker", spec.Await)
		}
		if spec.ExhaustedRegex != "(?i)usage limit reached" {
			t.Errorf("ExhaustedRegex=%q, want the usage-limit pattern", spec.ExhaustedRegex)
		}
	})

	t.Run("event with only send resolves", func(t *testing.T) {
		spec, ok := m.Control("clean_ctx")
		if !ok || spec.Send != "/clear" {
			t.Fatalf("Control(clean_ctx)=(%+v,%v), want ({Send:/clear},true)", spec, ok)
		}
	})

	t.Run("missing event is not-found", func(t *testing.T) {
		if _, ok := m.Control("status"); ok {
			t.Error("Control(status) ok=true, want false (event not in table)")
		}
	})
}

// TestManifestControl_NoControlsBlock verifies a manifest with no `controls`
// block (the pre-feature shape, and every CLI that supports no control events)
// reports every event as not-found without panicking on the nil map.
func TestManifestControl_NoControlsBlock(t *testing.T) {
	m, err := parseManifest("bare-tmux", []byte(`{"cli":"bare-tmux","binary":"bare"}`))
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if _, ok := m.Control("usage"); ok {
		t.Error("Control(usage) ok=true on a manifest with no controls, want false")
	}
}
