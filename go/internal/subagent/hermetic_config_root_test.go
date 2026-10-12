package subagent

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const hermeticWorktree = "/repo/worktrees/hermetic"

type inertBridge struct{}

func (inertBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}

func (inertBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func writeSentinelTranscript(t *testing.T, configRoot string) {
	t.Helper()
	dir := filepath.Join(configRoot, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","cwd":"` + hermeticWorktree + `","timestamp":"2026-07-08T12:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"go"}]}}
{"type":"assistant","cwd":"` + hermeticWorktree + `","timestamp":"2026-07-08T12:00:05Z","message":{"id":"m1","usage":{"input_tokens":80,"output_tokens":15,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExecAdapter_APinnedConfigRootKeepsTheTokenScanOffTheHostHome(t *testing.T) {
	hostHome, pinned := t.TempDir(), t.TempDir()
	writeSentinelTranscript(t, filepath.Join(hostHome, ".claude"))
	var captured gobridge.Deps
	prev := newAdapterBridge
	newAdapterBridge = func(d gobridge.Deps) core.Bridge { captured = d; return inertBridge{} }
	t.Cleanup(func() { newAdapterBridge = prev })
	env := map[string]string{"RESOLVED_CLI": "claude", "HOME": hostHome, "CLAUDE_CONFIG_DIR": pinned, "PROMPT_FILE": ""}

	if _, err := execAdapter(context.Background(), env, nil); err != nil {
		t.Fatal(err)
	}
	res, err := captured.TokenResolver(tokenusage.Window{
		Worktree: hermeticWorktree,
		Start:    mustParseRFC3339Subagent(t, "2026-07-08T11:59:00Z"),
		End:      mustParseRFC3339Subagent(t, "2026-07-08T12:01:00Z"),
	})

	if err != nil || res.Source == tokenusage.SourceTranscript {
		t.Errorf("resolver = (%+v, %v), want the sentinel under HOME unread while CLAUDE_CONFIG_DIR points at an empty root", res, err)
	}
}

func unpinnedRealDispatchers(t *testing.T, filename string, src any) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var dispatches, pinned, stubbed bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "execAdapter" || id.Name == "defaultExecAdapter") {
					dispatches = true
				}
			case *ast.BasicLit:
				if v, err := strconv.Unquote(x.Value); err == nil && v == "CLAUDE_CONFIG_DIR" {
					pinned = true
				}
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok && id.Name == "newAdapterBridge" {
						stubbed = true
					}
				}
			}
			return true
		})
		if dispatches && !pinned && !stubbed {
			bad = append(bad, fn.Name.Name)
		}
	}
	return bad
}

func TestUnpinnedRealDispatchers_FlagsADispatcherWithoutAPinAndAcceptsPinnedOrStubbed(t *testing.T) {
	const unpinned = `package p
func TestA() { defaultExecAdapter(nil, "", map[string]string{"HOME": "x"}) }
func TestB() { execAdapter(nil, nil, nil) }`
	const pinned = `package p
func TestA() { defaultExecAdapter(nil, "", map[string]string{"CLAUDE_CONFIG_DIR": "x"}) }
func TestB() { newAdapterBridge = nil; execAdapter(nil, nil, nil) }`
	const commentOnly = `package p
// CLAUDE_CONFIG_DIR
func TestA() { execAdapter(nil, nil, nil) }`

	if got := unpinnedRealDispatchers(t, "f.go", unpinned); len(got) != 2 {
		t.Errorf("unpinned fixture flagged %v, want TestA and TestB", got)
	}
	if got := unpinnedRealDispatchers(t, "f.go", pinned); len(got) != 0 {
		t.Errorf("pinned fixture flagged %v, want none", got)
	}
	if got := unpinnedRealDispatchers(t, "f.go", commentOnly); len(got) != 1 {
		t.Errorf("comment-only fixture flagged %v, want TestA", got)
	}
}

func TestExecAdapter_EveryRealBridgeTestPinsTheConfigRoot(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob = %v, %v", files, err)
	}
	for _, f := range files {
		if bad := unpinnedRealDispatchers(t, f, nil); len(bad) > 0 {
			t.Errorf("%s: %v reach the real engine without pinning CLAUDE_CONFIG_DIR or stubbing newAdapterBridge; an unpinned run scans the host ~/.claude", f, bad)
		}
	}
}
