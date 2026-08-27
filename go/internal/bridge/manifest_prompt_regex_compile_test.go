package bridge

import (
	"regexp"
	"testing"
)

// manifest_prompt_regex_compile_test.go — every interactive_prompts regex, in
// every manifest, must COMPILE.
//
// decideAutoRespond swallows a compile error and moves on (autorespond.go:
// `re, err := regexp.Compile(p.Regex); if err != nil { continue }`), so a rule
// whose pattern is invalid is not a loud failure — it is a rule that silently
// does nothing, and the symptom is the hang it was written to prevent.
//
// Found the hard way 2026-08-27: a plan-mode rule was authored with a `{0,1200}`
// bound. Go's RE2 caps repeat counts at 1000, so the pattern never compiled and
// the rule was dead on arrival — visible only because a new test happened to
// assert the rule fired. `controls.exhausted_regex` has had a compile guard
// since manifest_controls_test.go; interactive_prompts, the field whose silent
// death costs a lane, had none.
func TestManifestInteractivePromptRegexesCompile(t *testing.T) {
	t.Parallel()
	// Glob-driven via ManifestNames(), NOT a hardcoded list: a guard against
	// silently-inert rules that is itself blind to three of seven manifests
	// reproduces the defect one level up. The same reasoning is written into
	// manifest_tier_coverage_test.go and realizer_tier_sentinel_test.go — a
	// future CLI cannot reintroduce the gap by simply not being listed here.
	clis := ManifestNames()
	if len(clis) == 0 {
		t.Fatal("ManifestNames() returned nothing — the guard would inspect no manifests at all")
	}
	checked := 0
	for _, cli := range clis {
		cli := cli
		t.Run(cli, func(t *testing.T) {
			t.Parallel()
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%q): %v", cli, err)
			}
			for _, p := range m.InteractivePrompts {
				if p.Regex == "" {
					t.Errorf("prompt %q has an empty regex — it can never fire", p.Name)
					continue
				}
				if _, err := regexp.Compile(p.Regex); err != nil {
					t.Errorf("prompt %q regex does not compile (the rule would be SILENTLY INERT): %v\n  regex: %s",
						p.Name, err, p.Regex)
				}
			}
		})
		m, err := LoadManifest(cli)
		if err == nil {
			checked += len(m.InteractivePrompts)
		}
	}
	// Anti-vacuity: a manifest set that loads zero rules would pass every
	// assertion above while proving nothing.
	if checked < len(clis) {
		t.Fatalf("only %d interactive prompts inspected across %d manifests — the guard is vacuous", checked, len(clis))
	}
}
