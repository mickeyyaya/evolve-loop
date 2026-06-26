package skillcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_GenerateWritesCommandStubs proves `skills generate` materializes a
// commands/<name>.md for every skill (delegating to evo:<name>), and that a
// follow-up check is clean — the projection is idempotent.
func TestRun_GenerateWritesCommandStubs(t *testing.T) {
	tmp := prepareSkillsTree(t) // skills present, commands/ absent
	var out, errBuf strings.Builder
	if code := Run(tmp, true, &out, &errBuf); code != 0 {
		t.Fatalf("generate: exit %d\nstderr:\n%s", code, errBuf.String())
	}
	raw, err := os.ReadFile(filepath.Join(tmp, "commands", "evo-loop.md"))
	if err != nil {
		t.Fatalf("commands/evo-loop.md not generated: %v", err)
	}
	for _, want := range []string{"evo:loop", "$ARGUMENTS", commandGenMarker} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("commands/loop.md missing %q\n%s", want, raw)
		}
	}
	out.Reset()
	errBuf.Reset()
	if code := Run(tmp, false, &out, &errBuf); code != 0 {
		t.Fatalf("check after generate: exit %d, want 0 (idempotent)\nstderr:\n%s", code, errBuf.String())
	}
}

// TestRun_CheckDetectsMissingCommand: deleting a generated stub trips the gate.
func TestRun_CheckDetectsMissingCommand(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var out, errBuf strings.Builder
	if code := Run(tmp, true, &out, &errBuf); code != 0 {
		t.Fatalf("generate: exit %d", code)
	}
	if err := os.Remove(filepath.Join(tmp, "commands", "evo-loop.md")); err != nil {
		t.Fatalf("rm command: %v", err)
	}
	out.Reset()
	errBuf.Reset()
	if code := Run(tmp, false, &out, &errBuf); code != 2 {
		t.Fatalf("check with missing command: exit %d, want 2\nstderr:\n%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "DRIFT") {
		t.Errorf("missing DRIFT report:\n%s", errBuf.String())
	}
}

// TestRun_OrphanCommand: a generated stub whose skill no longer exists is
// flagged by check and reaped by generate; a hand-authored command (no marker)
// is never touched.
func TestRun_OrphanCommand(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var out, errBuf strings.Builder
	if code := Run(tmp, true, &out, &errBuf); code != 0 {
		t.Fatalf("generate: exit %d", code)
	}
	orphan := filepath.Join(tmp, "commands", "evo-ghost.md") // generated marker, skill removed
	if err := os.WriteFile(orphan, []byte(RenderCommandStub("ghost", "x", "")), 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}
	hand := filepath.Join(tmp, "commands", "handmade.md") // no marker — must survive
	if err := os.WriteFile(hand, []byte("---\ndescription: mine\n---\nhand-authored\n"), 0o644); err != nil {
		t.Fatalf("write hand: %v", err)
	}

	out.Reset()
	errBuf.Reset()
	if code := Run(tmp, false, &out, &errBuf); code != 2 {
		t.Fatalf("check with orphan: exit %d, want 2", code)
	}

	out.Reset()
	errBuf.Reset()
	if code := Run(tmp, true, &out, &errBuf); code != 0 {
		t.Fatalf("regen: exit %d", code)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan ghost.md was not reaped")
	}
	if _, err := os.Stat(hand); err != nil {
		t.Errorf("hand-authored handmade.md was removed: %v", err)
	}
}
