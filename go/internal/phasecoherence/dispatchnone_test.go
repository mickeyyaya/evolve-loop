package phasecoherence

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestDispatchNoneDirectBranches(t *testing.T) {
	tempDir := t.TempDir()
	nonePath := filepath.Join(tempDir, "none.md")
	if err := os.WriteFile(nonePath, []byte(personaMD("operator", `dispatch: none`)), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := fstest.MapFS{
		"agents/evolve-plain.md": {
			Data: []byte("# plain persona without frontmatter\n"),
		},
		"agents/evolve-managed.md": {
			Data: []byte(personaMD("managed", `dispatch: normal`)),
		},
	}

	tests := map[string]struct {
		opts Options
		name string
		want bool
	}{
		"override dispatch none": {
			opts: Options{AgentsFS: agents, Overrides: map[string]string{"operator": nonePath}},
			name: "operator",
			want: true,
		},
		"missing override file": {
			opts: Options{AgentsFS: agents, Overrides: map[string]string{"missing": filepath.Join(tempDir, "missing.md")}},
			name: "missing",
			want: false,
		},
		"no frontmatter": {
			opts: Options{AgentsFS: agents},
			name: "plain",
			want: false,
		},
		"non none dispatch": {
			opts: Options{AgentsFS: agents},
			name: "managed",
			want: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := dispatchNone(tt.opts, tt.name); got != tt.want {
				t.Fatalf("dispatchNone(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
