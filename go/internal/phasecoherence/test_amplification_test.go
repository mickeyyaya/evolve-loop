package phasecoherence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAmplification_CanonicalRoleCaseVariantsUseDefaultLowercase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "capital scout is not exact scout arm", in: "Scout", want: "scout"},
		{name: "upper build is not exact build arm", in: "BUILD", want: "build"},
		{name: "capital auditor is not exact auditor arm", in: "Auditor", want: "auditor"},
		{name: "upper memo is not exact memo arm", in: "MEMO", want: "memo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalRole(tt.in); got != tt.want {
				t.Fatalf("canonicalRole(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAmplification_DispatchNoneAllowsSurroundingFrontmatterFields(t *testing.T) {
	dir := t.TempDir()
	persona := filepath.Join(dir, "noisy-persona.md")
	if err := os.WriteFile(persona, []byte(`---
name: noisy-persona
description: persona with dispatch disabled
dispatch: none
model: sonnet
---

Body text is irrelevant to dispatch selection.
`), 0o644); err != nil {
		t.Fatalf("write persona fixture: %v", err)
	}

	opts := Options{
		Overrides: map[string]string{
			"noisy-persona": persona,
		},
	}

	if !dispatchNone(opts, "noisy-persona") {
		t.Fatalf("dispatchNone returned false for dispatch:none with surrounding frontmatter fields")
	}
}
