package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifiedActiveMints(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		dir := filepath.Join(root, ".evolve", "phases", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "phase.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("good-mint", `{"name":"good-mint","optional":true}`)
	write("bad-optional", `{"name":"bad-optional","optional":false}`)
	write("bad-mismatch", `{"name":"other-name","optional":true}`)
	write("bad-garbage", `{not json`)
	write("bad-huge", `{"name":"bad-huge","optional":true,"pad":"`+strings.Repeat("x", 1<<20)+`"}`)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "phases", "bad-symlink"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(root, ".evolve", "phases", "good-mint", "phase.json"),
		filepath.Join(root, ".evolve", "phases", "bad-symlink", "phase.json"),
	); err != nil {
		t.Fatal(err)
	}
	mints := map[string]bool{
		"good-mint": true, "bad-optional": true, "bad-mismatch": true,
		"bad-garbage": true, "bad-missing": true, "../bad-trav": true,
		"bad-huge": true, "bad-symlink": true,
	}
	got := verifiedActiveMints(root, mints)
	if len(got) != 1 || !got["good-mint"] {
		t.Errorf("verifiedActiveMints=%v, want {good-mint} only", got)
	}
	if out := verifiedActiveMints(root, nil); len(out) != 0 {
		t.Errorf("nil input must stay empty; got %v", out)
	}
}

func TestIsActiveMintPhasePath(t *testing.T) {
	t.Parallel()

	mints := map[string]bool{"gate-wiring-proof": true, "gate": true}
	cases := []struct {
		path string
		want bool
	}{
		{".evolve/phases/gate-wiring-proof/phase.json", true},
		{".evolve/phases/gate-wiring-proof", true},
		{".evolve/phases/gate-wiring-proof/nested/extra.json", false},
		{".evolve/phases/gate-wiring-proof/payload.sh", false},
		{".evolve/phases/gate-wiring-proof/", false},
		{".evolve/phases/evil/phase.json", false},
		{".evolve/phases/gate-wiring-proof-evil/x.json", false},
		{".evolve/phases/", false},
		{".evolve/phases", false},
		{".evolve/evals/gate-wiring-proof.md", false},
		{".evolve/phasesx/gate-wiring-proof/phase.json", false},
		{"go/internal/core/leak.go", false},
		{"x/.evolve/phases/gate-wiring-proof/phase.json", false},
	}
	for _, tc := range cases {
		if got := isActiveMintPhasePath(mints, tc.path); got != tc.want {
			t.Errorf("isActiveMintPhasePath(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
	if isActiveMintPhasePath(nil, ".evolve/phases/gate-wiring-proof/phase.json") {
		t.Error("nil registry must exempt nothing")
	}
}
