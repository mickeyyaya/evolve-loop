// release_extra_test.go — covers the maybeCreateRelease branches the two
// existing tests (no-notes early skip, missing plugin.json) don't reach:
// the gh-invocation success path, dry-run, an unparseable version, and a
// non-zero gh exit. gh is stubbed via a capturing CmdRunner so no real
// GitHub call is made.
package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// ghCapture is a CmdRunner that records gh invocations and returns ghExit
// for them; every other binary returns (0, nil).
type ghCapture struct {
	calls  [][]string
	ghExit int
}

func (g *ghCapture) runner() CmdRunner {
	return func(_ context.Context, name, _ string, args, _ []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		if name == "gh" {
			g.calls = append(g.calls, append([]string{name}, args...))
			return g.ghExit, nil
		}
		return 0, nil
	}
}

func writePluginJSON(t *testing.T, root, body string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, ".claude-plugin", "plugin.json"), body)
}

func TestMaybeCreateRelease_Success_InvokesGhWithVersionTag(t *testing.T) {
	g := &ghCapture{ghExit: 0}
	root := t.TempDir()
	writePluginJSON(t, root, `{"version":"9.9.9"}`)
	opts := &Options{
		Class:       ClassRelease,
		ProjectRoot: root,
		PluginRoot:  root,
		Env:         map[string]string{"EVOLVE_SHIP_RELEASE_NOTES": "release notes body"},
		Runner:      g.runner(),
	}
	res := &RunResult{}
	if err := maybeCreateRelease(context.Background(), opts, res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.calls) != 1 {
		t.Fatalf("want exactly 1 gh invocation, got %d (%v)", len(g.calls), g.calls)
	}
	if !strings.Contains(strings.Join(g.calls[0], " "), "release create v9.9.9") {
		t.Errorf("gh not invoked with v9.9.9 tag; got %v", g.calls[0])
	}
	if !containsLog(*res, "GitHub release v9.9.9 created") {
		t.Errorf("missing success log: %v", res.Logs)
	}
}

func TestMaybeCreateRelease_DryRun_DoesNotInvokeGh(t *testing.T) {
	g := &ghCapture{}
	root := t.TempDir()
	writePluginJSON(t, root, `{"version":"1.2.3"}`)
	opts := &Options{
		Class:       ClassRelease,
		ProjectRoot: root,
		PluginRoot:  root,
		DryRun:      true,
		Env:         map[string]string{"EVOLVE_SHIP_RELEASE_NOTES": "notes"},
		Runner:      g.runner(),
	}
	res := &RunResult{}
	if err := maybeCreateRelease(context.Background(), opts, res); err != nil {
		t.Fatal(err)
	}
	if len(g.calls) != 0 {
		t.Errorf("dry-run must not invoke gh; got %v", g.calls)
	}
	if !containsLog(*res, "[DRY-RUN] would create GitHub release v1.2.3") {
		t.Errorf("missing dry-run log: %v", res.Logs)
	}
}

func TestMaybeCreateRelease_UnparseableVersion_SkipsWithWarn(t *testing.T) {
	g := &ghCapture{}
	root := t.TempDir()
	writePluginJSON(t, root, `{"name":"no-version-field"}`) // valid JSON, empty version
	opts := &Options{
		Class:       ClassRelease,
		ProjectRoot: root,
		PluginRoot:  root,
		Env:         map[string]string{"EVOLVE_SHIP_RELEASE_NOTES": "notes"},
		Runner:      g.runner(),
	}
	res := &RunResult{}
	if err := maybeCreateRelease(context.Background(), opts, res); err != nil {
		t.Fatal(err)
	}
	if len(g.calls) != 0 {
		t.Errorf("empty version must skip gh; got %v", g.calls)
	}
	if !containsLog(*res, "cannot parse plugin.json:version") {
		t.Errorf("missing parse-skip warn: %v", res.Logs)
	}
}

func TestMaybeCreateRelease_GhNonZero_WarnsAndContinues(t *testing.T) {
	g := &ghCapture{ghExit: 1} // gh fails (e.g., release already exists)
	root := t.TempDir()
	writePluginJSON(t, root, `{"version":"4.5.6"}`)
	opts := &Options{
		Class:       ClassRelease,
		ProjectRoot: root,
		PluginRoot:  root,
		Env:         map[string]string{"EVOLVE_SHIP_RELEASE_NOTES": "notes"},
		Runner:      g.runner(),
	}
	res := &RunResult{}
	// Non-zero gh is best-effort: it must NOT fail the ship.
	if err := maybeCreateRelease(context.Background(), opts, res); err != nil {
		t.Fatalf("gh failure must be non-fatal; got %v", err)
	}
	if !containsLog(*res, "WARN: gh release create failed") {
		t.Errorf("missing gh-failure warn: %v", res.Logs)
	}
}
