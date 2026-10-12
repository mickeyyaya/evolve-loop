package skilloverlay

import (
	"os"
	"strings"
	"testing"
)

func TestMaterialize_TheDocsWriteOverlayIsItsCompactProjectionWithoutFrontmatter(t *testing.T) {
	prefix, missing := Materialize("../../../skills", []string{"docs-write"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
	full, err := os.ReadFile("../../../skills/docs-write/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prefix, "# docs-write (preloaded)") {
		t.Errorf("the preloaded docs-write body must be COMPACT.md; got %d bytes", len(prefix))
	}
	if strings.Contains(prefix, "description: Preloaded form of docs-write") {
		t.Errorf("the frontmatter of COMPACT.md must be stripped")
	}
	if len(prefix) >= len(full) {
		t.Errorf("the block is %d bytes, not smaller than SKILL.md (%d): SKILL.md was injected", len(prefix), len(full))
	}
	if len(prefix) > 4096 {
		t.Errorf("the preloaded docs-write block is %d bytes; it stays under 4 KB", len(prefix))
	}
}
