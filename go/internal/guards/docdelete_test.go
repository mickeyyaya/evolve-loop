package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// DocDelete is the port of scripts/hooks/doc-deletion-guard.sh.
// Rules:
//   - rm command targeting docs/** or knowledge-base/** → DENY
//   - mv from docs/** or knowledge-base/** → DENY unless the dest stays under
//     docs/ (reorganization within the single doc root is never a deletion;
//     the archive home is docs/private/research/archived-YYYY-MM-DD/ since
//     the 2026-08-05 doc-root consolidation — knowledge-base/research/ is
//     retired as an archive target)
//   - constructor policy can bypass
//   - Edit / Write tools pass through (cannot delete files in place)
func TestDocDelete_Name(t *testing.T) {
	g := NewDocDelete(false)
	if g.Name() != "docdelete" {
		t.Errorf("name=%q", g.Name())
	}
}

func TestDocDelete_AllowsNonRm(t *testing.T) {
	g := NewDocDelete(false)
	cases := []string{
		"ls docs/",
		"cat docs/README.md",
		"go test ./...",
	}
	for _, cmd := range cases {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if !dec.Allow {
			t.Errorf("%q denied: %s", cmd, dec.Reason)
		}
	}
}

func TestDocDelete_DeniesRmDocs(t *testing.T) {
	g := NewDocDelete(false)
	cases := []string{
		"rm docs/foo.md",
		"rm -rf docs/architecture",
		"rm -rf knowledge-base/research/old-stuff",
		"rm  knowledge-base/lessons.md",
	}
	for _, cmd := range cases {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if dec.Allow {
			t.Errorf("%q allowed (must deny)", cmd)
		}
	}
}

func TestDocDelete_BypassPolicyAllows(t *testing.T) {
	g := NewDocDelete(true)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "rm -rf docs/old"},
	})
	if !dec.Allow {
		t.Errorf("allow policy must permit deletion, got: %s", dec.Reason)
	}
}

func TestDocDelete_DeniesMvOutOfDocs(t *testing.T) {
	g := NewDocDelete(false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "mv docs/old.md /tmp/"},
	})
	if dec.Allow {
		t.Error("mv docs/* to /tmp must deny")
	}
}

func TestDocDelete_AllowsMvToArchive(t *testing.T) {
	g := NewDocDelete(false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "mv docs/old.md docs/private/research/archived-2026-05-22/old.md"},
	})
	if !dec.Allow {
		t.Errorf("mv to the docs-private archive home must allow, got: %s", dec.Reason)
	}
}

func TestDocDelete_AllowsConsolidationIntoDocs(t *testing.T) {
	g := NewDocDelete(false)
	cases := []string{
		"git mv knowledge-base/research/x.md docs/research/x.md",
		"git mv knowledge-base/research/token-optimization-2026 docs/research/token-optimization-2026",
		"mv docs/research/a.md docs/chronicle/a.md",
	}
	for _, cmd := range cases {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		})
		if !dec.Allow {
			t.Errorf("%q denied (moves INTO docs/ are reorganization, not deletion): %s", cmd, dec.Reason)
		}
	}
}

func TestDocDelete_DeniesMvToLegacyArchiveHome(t *testing.T) {
	g := NewDocDelete(false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "mv docs/old.md knowledge-base/research/archived-2026-09-01/old.md"},
	})
	if dec.Allow {
		t.Error("legacy archive home (knowledge-base/research/archived-*) is retired — must deny")
	}
}

func TestDocDelete_PassThroughForEditWrite(t *testing.T) {
	g := NewDocDelete(false)
	for _, tool := range []string{"Edit", "Write", "Read"} {
		dec := g.Decide(context.Background(), core.GuardInput{ToolName: tool})
		if !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}

func TestDocDelete_MissingCommandIsAllow(t *testing.T) {
	g := NewDocDelete(false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{}, // no command field
	})
	if !dec.Allow {
		t.Errorf("missing command must allow, got: %s", dec.Reason)
	}
}
