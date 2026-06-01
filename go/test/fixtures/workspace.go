package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Workspace is a fully-built isolated project root under t.TempDir(): a
// project root containing a .evolve/ directory, optionally seeded with
// state.json, cycle-state.json, arbitrary files, and a git repo. It replaces
// the ~10 copy-pasted newStore()/SetupTempProject() variants. Callers that
// need a real storage adapter construct it themselves with storage.New(ws.EvolveDir)
// — Workspace deliberately does not import the storage package (that would
// form an import cycle with storage's own white-box tests).
type Workspace struct {
	T         *testing.T
	Root      string // project root (a t.TempDir())
	EvolveDir string // <Root>/.evolve
}

// Path joins parts onto the project root.
func (w *Workspace) Path(parts ...string) string {
	return filepath.Join(append([]string{w.Root}, parts...)...)
}

// CycleDir returns <Root>/.evolve/runs/cycle-<n>, creating it on first use.
func (w *Workspace) CycleDir(n int) string {
	w.T.Helper()
	dir := filepath.Join(w.EvolveDir, "runs", fmt.Sprintf("cycle-%d", n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		w.T.Fatalf("fixtures: mkdir cycle dir: %v", err)
	}
	return dir
}

// Write writes body to a path relative to the project root, creating parents.
func (w *Workspace) Write(rel, body string) string {
	w.T.Helper()
	full := w.Path(rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		w.T.Fatalf("fixtures: mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		w.T.Fatalf("fixtures: write %s: %v", rel, err)
	}
	return full
}

// WorkspaceBuilder is a fluent builder (the Builder pattern) for a Workspace.
// Every With* method returns the builder so calls chain; Build() materializes.
type WorkspaceBuilder struct {
	t          *testing.T
	state      *core.State
	cycleState *core.CycleState
	files      map[string]string         // rel path -> body, written under Root
	cycleFiles map[int]map[string]string // cycle -> {name -> body} under cycle dir
	gitInit    bool
}

// NewWorkspace starts a builder. With no further calls, Build() yields a bare
// project root + empty .evolve/ directory (the common case).
func NewWorkspace(t *testing.T) *WorkspaceBuilder {
	t.Helper()
	return &WorkspaceBuilder{t: t}
}

// WithState seeds .evolve/state.json with s (marshaled with core.State's own
// json tags — the on-disk schema for the modeled subset of fields).
func (b *WorkspaceBuilder) WithState(s core.State) *WorkspaceBuilder {
	b.state = &s
	return b
}

// WithCycleState seeds .evolve/cycle-state.json with cs.
func (b *WorkspaceBuilder) WithCycleState(cs core.CycleState) *WorkspaceBuilder {
	b.cycleState = &cs
	return b
}

// WithFiles seeds arbitrary files (rel path -> body) under the project root.
func (b *WorkspaceBuilder) WithFiles(files map[string]string) *WorkspaceBuilder {
	if b.files == nil {
		b.files = map[string]string{}
	}
	for k, v := range files {
		b.files[k] = v
	}
	return b
}

// WithCycleFiles seeds files under .evolve/runs/cycle-<n>/ (handoff artifacts).
func (b *WorkspaceBuilder) WithCycleFiles(cycle int, files map[string]string) *WorkspaceBuilder {
	if b.cycleFiles == nil {
		b.cycleFiles = map[int]map[string]string{}
	}
	dst := b.cycleFiles[cycle]
	if dst == nil {
		dst = map[string]string{}
		b.cycleFiles[cycle] = dst
	}
	for k, v := range files {
		dst[k] = v
	}
	return b
}

// WithGitInit initializes a git repo at the project root (init + a config
// identity), so tests that shell out to git operate on isolated state.
func (b *WorkspaceBuilder) WithGitInit() *WorkspaceBuilder {
	b.gitInit = true
	return b
}

// Build materializes the workspace on disk and returns it.
func (b *WorkspaceBuilder) Build() *Workspace {
	t := b.t
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("fixtures: mkdir .evolve: %v", err)
	}
	ws := &Workspace{T: t, Root: root, EvolveDir: evolveDir}

	if b.state != nil {
		writeJSON(t, filepath.Join(evolveDir, "state.json"), b.state)
	}
	if b.cycleState != nil {
		writeJSON(t, filepath.Join(evolveDir, "cycle-state.json"), b.cycleState)
	}
	for rel, body := range b.files {
		ws.Write(rel, body)
	}
	for cycle, files := range b.cycleFiles {
		dir := ws.CycleDir(cycle)
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatalf("fixtures: write cycle file %s: %v", name, err)
			}
		}
	}
	if b.gitInit {
		gitInit(t, root)
	}
	return ws
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("fixtures: marshal %s: %v", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("fixtures: write %s: %v", filepath.Base(path), err)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixtures: git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "fixtures")
}
