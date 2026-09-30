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

func TestCLIClassifierClassify_NilDispatcherErrorsNeverShellsOut(t *testing.T) {
	c := CLIClassifier{CLI: "codex"}
	_, err := c.Classify(context.Background(), "codex", []string{"m1"})
	if err == nil {
		t.Fatal("expected error when Dispatcher is nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "dispatcher") {
		t.Errorf("error should name the missing dispatcher, got %v", err)
	}
}

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

func TestCLIClassifierClassify_BadReply(t *testing.T) {
	d := &fakeDispatcher{out: "I cannot help with that."}
	if _, err := (CLIClassifier{CLI: "codex", Dispatcher: d}).Classify(context.Background(), "codex", []string{"x"}); err == nil {
		t.Fatal("expected error when reply has no JSON")
	}
}

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
