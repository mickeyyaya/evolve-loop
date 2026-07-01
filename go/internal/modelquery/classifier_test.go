package modelquery

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// fakeDispatcher is a test double for PromptDispatcher: it records the (cli,
// prompt) it was invoked with and returns canned output/err, so tests can
// prove Classify goes THROUGH the seam instead of shelling out directly.
type fakeDispatcher struct {
	out        string
	err        error
	calls      int
	lastCLI    string
	lastPrompt string
}

func (f *fakeDispatcher) DispatchPrompt(_ context.Context, cli, prompt string) (string, error) {
	f.calls++
	f.lastCLI, f.lastPrompt = cli, prompt
	return f.out, f.err
}

// TestPromptDispatcher_InterfaceContract names the PromptDispatcher interface
// and pins its single-method contract, mirroring ModelCapturer's contract test
// (apicover_named_test.go): a DispatchPrompt implementation is usable through
// the interface. The production implementation lives in cmd/evolve, routed
// through the bridge — this seam is what keeps that fragile, live-only launch
// out of the unit-tested classifier package (mirrors ModelCapturer/recipe.go).
func TestPromptDispatcher_InterfaceContract(t *testing.T) {
	t.Parallel()
	var d PromptDispatcher = &fakeDispatcher{out: `{"fast":"a","balanced":"b","deep":"c"}`}
	out, err := d.DispatchPrompt(context.Background(), "codex", "hello")
	if err != nil {
		t.Fatalf("DispatchPrompt: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

// TestCLIClassifierClassify_DispatchesThroughPromptDispatcher is the positive
// C1 fix assertion (GAP 1): Classify must deliver the built classification
// prompt through the injected PromptDispatcher — never a raw exec.Command or
// an injected Runner — and use the dispatcher's reply to build the tier map.
func TestCLIClassifierClassify_DispatchesThroughPromptDispatcher(t *testing.T) {
	d := &fakeDispatcher{out: "OpenAI Codex\ncodex\n{\"fast\":\"gpt-5.4-mini\",\"balanced\":\"gpt-5.4\",\"deep\":\"gpt-5.5\"}\ntokens used\n"}
	c := CLIClassifier{CLI: "codex", Dispatcher: d}
	got, err := c.Classify(context.Background(), "codex", []string{"gpt-5.4-mini", "gpt-5.4", "gpt-5.5"})
	if err != nil {
		t.Fatal(err)
	}
	if got["deep"] != "gpt-5.5" || got["fast"] != "gpt-5.4-mini" {
		t.Fatalf("classify = %v", got)
	}
	if d.calls != 1 {
		t.Fatalf("expected exactly one DispatchPrompt call, got %d", d.calls)
	}
	if d.lastCLI != "codex" {
		t.Fatalf("dispatcher cli = %q, want codex", d.lastCLI)
	}
	if !strings.Contains(d.lastPrompt, "gpt-5.4-mini") {
		t.Fatalf("dispatcher prompt missing an offered model id: %q", d.lastPrompt)
	}
}

// TestCLIClassifierClassify_NilDispatcherErrorsNeverShellsOut is the negative
// C1 fix assertion: a CLIClassifier with no Dispatcher must error, not fall
// back to a raw-exec default (the old `run == nil -> defaultRunner` behavior
// is the exact bypass this cycle removes).
func TestCLIClassifierClassify_NilDispatcherErrorsNeverShellsOut(t *testing.T) {
	c := CLIClassifier{CLI: "codex"} // Dispatcher intentionally nil
	_, err := c.Classify(context.Background(), "codex", []string{"m1"})
	if err == nil {
		t.Fatal("expected error when Dispatcher is nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "dispatcher") {
		t.Errorf("error should name the missing dispatcher, got %v", err)
	}
}

// TestCLIClassifierClassify_DispatcherErrorPropagates covers the edge case of
// an external dispatch failure (bridge launch error, CLI exhaustion, etc.): the
// error must propagate through Classify, wrapped with context, not be silently
// swallowed or retried via a fallback exec path.
func TestCLIClassifierClassify_DispatcherErrorPropagates(t *testing.T) {
	d := &fakeDispatcher{err: errors.New("bridge launch failed")}
	_, err := (CLIClassifier{CLI: "codex", Dispatcher: d}).Classify(context.Background(), "codex", []string{"x"})
	if err == nil {
		t.Fatal("expected error to propagate from the dispatcher")
	}
	if !strings.Contains(err.Error(), "bridge launch failed") {
		t.Fatalf("expected wrapped dispatcher error, got %v", err)
	}
}

// TestCLIClassifierClassify_BadReply pins that a dispatcher reply with no JSON
// object still errors cleanly through the new seam (regression: this behavior
// pre-dates the seam and must survive the refactor).
func TestCLIClassifierClassify_BadReply(t *testing.T) {
	d := &fakeDispatcher{out: "I cannot help with that."}
	if _, err := (CLIClassifier{CLI: "codex", Dispatcher: d}).Classify(context.Background(), "codex", []string{"x"}); err == nil {
		t.Fatal("expected error when reply has no JSON")
	}
}

// TestCLIClassifierClassify_SkipsPromptEcho is the regression guard for the
// live bug ported to the new seam: codex echoes the prompt's literal JSON
// template before the real answer; the classifier must skip the template
// (models = "<id>", not offered) and use the real reply — proving the JSON
// object selection logic is untouched by routing the reply through
// PromptDispatcher instead of a raw Runner.
func TestCLIClassifierClassify_SkipsPromptEcho(t *testing.T) {
	echoed := `{"fast":"<id>","balanced":"<id>","deep":"<id>"}` + "\ncodex\n" +
		`{"fast":"phi4:latest","balanced":"llama3.3:latest","deep":"gemma4:31b-cloud"}` + "\ntokens used\n"
	d := &fakeDispatcher{out: echoed}
	got, err := (CLIClassifier{CLI: "codex", Dispatcher: d}).Classify(
		context.Background(), "ollama",
		[]string{"gemma4:latest", "gemma4:31b-cloud", "phi4:latest", "llama3.3:latest"})
	if err != nil {
		t.Fatal(err)
	}
	if got["fast"] != "phi4:latest" || got["deep"] != "gemma4:31b-cloud" || got["balanced"] != "llama3.3:latest" {
		t.Fatalf("classifier picked the wrong object: %v", got)
	}
}

// TestCLIClassifierClassify_AllObjectsFailToMap covers the loop's continue
// branch (json.Unmarshal failure on a non-object) AND the terminal "no JSON
// object mapped a tier" error: a reply with one malformed-typed object and one
// valid JSON object whose models are all hallucinated → no tier survives
// sanitize. Edge/OOD diversity axis: malformed dispatcher reply.
func TestCLIClassifierClassify_AllObjectsFailToMap(t *testing.T) {
	t.Parallel()
	reply := `{"fast":123}` + "\n" + `{"fast":"not-offered","deep":"also-not"}`
	d := &fakeDispatcher{out: reply}
	_, err := (CLIClassifier{CLI: "codex", Dispatcher: d}).Classify(
		context.Background(), "codex", []string{"real-model"})
	if err == nil {
		t.Fatal("want error when no object maps a tier to an offered model")
	}
	if !strings.Contains(err.Error(), "no JSON object mapped a tier") {
		t.Errorf("want terminal mapping error, got %v", err)
	}
}

// TestCLIClassifierGuards pins the pre-dispatch validation order (empty CLI /
// no model ids), which must still fire before the dispatcher is even
// consulted — regression floor carried over from the pre-seam behavior.
func TestCLIClassifierGuards(t *testing.T) {
	d := &fakeDispatcher{}
	if _, err := (CLIClassifier{Dispatcher: d}).Classify(context.Background(), "codex", []string{"x"}); err == nil {
		t.Fatal("expected error when classifier CLI unset")
	}
	if _, err := (CLIClassifier{CLI: "codex", Dispatcher: d}).Classify(context.Background(), "codex", nil); err == nil {
		t.Fatal("expected error when no model ids")
	}
	if d.calls != 0 {
		t.Fatalf("guards must reject before ever calling the dispatcher, got %d calls", d.calls)
	}
}

// TestGuard_ClassifierHasNoDirectModelExec is the C1 self-enforcing invariant
// (BA2 in scout-report.md): classifier.go must not reference the raw-exec seam
// (defaultRunner), the exec-argv builder that used to route a prompt around
// the bridge (classifierArgv), or the Runner type at all — every prompt must
// go through PromptDispatcher. AST-based (not substring/grep) so a renamed
// wrapper around the same call still trips the identifier check, and comments
// mentioning these names in passing don't false-positive.
func TestGuard_ClassifierHasNoDirectModelExec(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "classifier.go", nil, 0)
	if err != nil {
		t.Fatalf("parse classifier.go: %v", err)
	}
	banned := map[string]string{
		"defaultRunner":  "raw-exec runner",
		"classifierArgv": "exec-argv builder",
		"Runner":         "Runner type",
	}
	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if what, isBanned := banned[id.Name]; isBanned {
			t.Errorf("classifier.go references %q (%s) at %s — prompts must route through PromptDispatcher, not raw exec",
				id.Name, what, fset.Position(id.Pos()))
		}
		return true
	})
}
