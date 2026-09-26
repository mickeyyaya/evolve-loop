package guardcmd

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The hooks in .claude/settings.json run `evolve guard <name>` on every matching tool call, so a name the
// binary does not build turns each of those calls into a hook error.
func TestHookWiring_EveryWiredGuardIsAGuardTheBinaryBuilds(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	wired := regexp.MustCompile(`" guard ([a-z-]+)`).FindAllStringSubmatch(string(raw), -1)
	if len(wired) == 0 {
		t.Fatal("no `evolve guard <name>` hook found in .claude/settings.json; the wiring parse is broken")
	}
	for _, m := range wired {
		if _, err := buildGuard(m[1], t.TempDir(), false); err != nil {
			t.Errorf("hook runs `evolve guard %s`, which the binary cannot build: %v", m[1], err)
		}
	}
}
