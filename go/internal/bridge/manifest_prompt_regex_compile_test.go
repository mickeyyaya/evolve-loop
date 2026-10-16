package bridge

import (
	"regexp"
	"testing"
)

func TestManifestInteractivePromptRegexesCompile(t *testing.T) {
	t.Parallel()
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
	if checked < len(clis) {
		t.Fatalf("only %d interactive prompts inspected across %d manifests — the guard is vacuous", checked, len(clis))
	}
}
