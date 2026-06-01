package modelquery

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// stubRunner returns canned output/err and records the last invocation.
type stubRunner struct {
	out      string
	err      error
	lastName string
	lastArgs []string
}

func (s *stubRunner) run(_ context.Context, name string, args []string, _ string) (string, error) {
	s.lastName, s.lastArgs = name, args
	return s.out, s.err
}

func TestParseOllamaList(t *testing.T) {
	out := "NAME                ID              SIZE      MODIFIED\n" +
		"gemma4:latest       c6eb396dbd59    9.6 GB    3 days ago\n" +
		"phi4:latest         ac896e5b8b34    9.1 GB    16 months ago\n" +
		"\n"
	got := parseOllamaList(out)
	want := []string{"gemma4:latest", "phi4:latest"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestOllamaListerUsesRunner(t *testing.T) {
	s := &stubRunner{out: "NAME\nllama3.3:latest  x  y  z\n"}
	l := OllamaLister{Run: s.run}
	ids, err := l.List(context.Background(), "ollama")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "llama3.3:latest" {
		t.Fatalf("ids = %v", ids)
	}
	if s.lastName != "ollama" || strings.Join(s.lastArgs, " ") != "list" {
		t.Fatalf("invocation = %s %v", s.lastName, s.lastArgs)
	}
}

func TestOllamaListerError(t *testing.T) {
	s := &stubRunner{err: errors.New("not found")}
	if _, err := (OllamaLister{Run: s.run}).List(context.Background(), "ollama"); err == nil {
		t.Fatal("expected error")
	}
}

func TestJSONObjects(t *testing.T) {
	tests := []struct {
		name, in string
		want     []string
	}{
		{"plain", `{"fast":"a"}`, []string{`{"fast":"a"}`}},
		{"framed", "header\ncodex\n{\"fast\":\"a\",\"deep\":\"b\"}\ntokens used", []string{`{"fast":"a","deep":"b"}`}},
		{"nested counts as one", `x {"a":{"b":1}} y`, []string{`{"a":{"b":1}}`}},
		{"brace in string", `{"m":"a{b}c"}`, []string{`{"m":"a{b}c"}`}},
		{"escaped quote in string", `{"m":"a\"{x}"}`, []string{`{"m":"a\"{x}"}`}},
		// the live codex case: prompt-echo template first, real answer second.
		{"prompt echo then answer", `{"fast":"<id>"}` + "\ncodex\n" + `{"fast":"phi4"}`, []string{`{"fast":"<id>"}`, `{"fast":"phi4"}`}},
		{"lone close brace before object", `foo } {"fast":"a"}`, []string{`{"fast":"a"}`}},
		{"none", `no json here`, nil},
		{"unbalanced", `{"oops":`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonObjects(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("jsonObjects(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("obj[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestClassifierSkipsPromptEcho is the regression guard for the live bug: codex
// echoes the prompt's literal JSON template before the real answer; the
// classifier must skip the template (models = "<id>", not offered) and use the
// real reply.
func TestClassifierSkipsPromptEcho(t *testing.T) {
	echoed := `{"fast":"<id>","balanced":"<id>","deep":"<id>"}` + "\ncodex\n" +
		`{"fast":"phi4:latest","balanced":"llama3.3:latest","deep":"gemma4:31b-cloud"}` + "\ntokens used\n"
	s := &stubRunner{out: echoed}
	got, err := (CLIClassifier{CLI: "codex", Run: s.run}).Classify(
		context.Background(), "ollama",
		[]string{"gemma4:latest", "gemma4:31b-cloud", "phi4:latest", "llama3.3:latest"})
	if err != nil {
		t.Fatal(err)
	}
	if got["fast"] != "phi4:latest" || got["deep"] != "gemma4:31b-cloud" || got["balanced"] != "llama3.3:latest" {
		t.Fatalf("classifier picked the wrong object: %v", got)
	}
}

func TestSanitizeTierMap(t *testing.T) {
	offered := []string{"m-fast", "m-bal", "m-deep"}
	parsed := map[string]string{
		"fast":     "m-fast",
		"balanced": "m-bal",
		"deep":     "hallucinated", // not offered → dropped
		"ultra":    "m-deep",       // non-canonical tier → dropped
		"weird":    "",
	}
	got := sanitizeTierMap(parsed, offered)
	if len(got) != 2 || got["fast"] != "m-fast" || got["balanced"] != "m-bal" {
		t.Fatalf("sanitize = %v", got)
	}
	if _, ok := got["deep"]; ok {
		t.Fatal("hallucinated deep model should be dropped")
	}
}

func TestClassifierArgv(t *testing.T) {
	tests := []struct {
		cli, wantName, wantArg0 string
	}{
		{"codex", "codex", "exec"},
		{"agy", "agy", "-p"},
		{"claude", "claude", "-p"},
	}
	for _, tt := range tests {
		name, args := classifierArgv(tt.cli, "PROMPT")
		if name != tt.wantName || args[0] != tt.wantArg0 {
			t.Fatalf("argv(%q) = %s %v", tt.cli, name, args)
		}
		if args[len(args)-1] != "PROMPT" {
			t.Fatalf("prompt not last arg for %q: %v", tt.cli, args)
		}
	}
}

func TestCLIClassifierClassify(t *testing.T) {
	s := &stubRunner{out: "OpenAI Codex\ncodex\n{\"fast\":\"gpt-5.4-mini\",\"balanced\":\"gpt-5.4\",\"deep\":\"gpt-5.5\"}\ntokens used\n"}
	c := CLIClassifier{CLI: "codex", Run: s.run}
	got, err := c.Classify(context.Background(), "codex", []string{"gpt-5.4-mini", "gpt-5.4", "gpt-5.5"})
	if err != nil {
		t.Fatal(err)
	}
	if got["deep"] != "gpt-5.5" || got["fast"] != "gpt-5.4-mini" {
		t.Fatalf("classify = %v", got)
	}
	if s.lastName != "codex" || s.lastArgs[0] != "exec" {
		t.Fatalf("expected codex exec, got %s %v", s.lastName, s.lastArgs)
	}
}

func TestCLIClassifierBadReply(t *testing.T) {
	s := &stubRunner{out: "I cannot help with that."}
	if _, err := (CLIClassifier{CLI: "codex", Run: s.run}).Classify(context.Background(), "codex", []string{"x"}); err == nil {
		t.Fatal("expected error when reply has no JSON")
	}
}

// TestDefaultRunner_CapturesOutput exercises the production exec runner end to
// end against a guaranteed-present shell builtin wrapper. Skipped only when the
// chosen binary is genuinely absent from PATH.
func TestDefaultRunner_CapturesOutput(t *testing.T) {
	t.Parallel()
	echo, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not on PATH")
	}
	out, rerr := defaultRunner(context.Background(), echo, []string{"hello-runner"}, "")
	if rerr != nil {
		t.Fatalf("defaultRunner: %v", rerr)
	}
	if !strings.Contains(out, "hello-runner") {
		t.Errorf("output = %q, want it to contain %q", out, "hello-runner")
	}
}

// TestDefaultRunner_PipesStdin covers the stdin != "" branch via `cat`, which
// echoes stdin to stdout. Skipped only when cat is genuinely absent.
func TestDefaultRunner_PipesStdin(t *testing.T) {
	t.Parallel()
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat not on PATH")
	}
	out, rerr := defaultRunner(context.Background(), cat, nil, "piped-in")
	if rerr != nil {
		t.Fatalf("defaultRunner: %v", rerr)
	}
	if strings.TrimSpace(out) != "piped-in" {
		t.Errorf("output = %q, want %q", out, "piped-in")
	}
}

// TestDefaultRunner_NonZeroExitReturnsError covers the error return path of the
// production runner (CombinedOutput err non-nil).
func TestDefaultRunner_NonZeroExitReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not on PATH")
	}
	if _, rerr := defaultRunner(context.Background(), "false", nil, ""); rerr == nil {
		t.Fatal("want error from non-zero exit")
	}
}

// TestErrNoLister_Error pins the error string of the router's no-lister sentinel.
func TestErrNoLister_Error(t *testing.T) {
	t.Parallel()
	got := errNoLister("ollama").Error()
	want := "modelquery: no lister for cli ollama"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestOllamaLister_NilRunDefaultsToExecRunner covers the `run == nil` default
// branch in OllamaLister.List: with no injected Runner it routes through
// defaultRunner and shells out to the real `ollama`. The branch executes
// regardless of whether ollama is installed — so the assertion tolerates both
// outcomes (present → nil err + ids; absent → wrapped "ollama list" error) and
// pins only that the nil-Run path does not panic and stays internally
// consistent (error XOR ids).
func TestOllamaLister_NilRunDefaultsToExecRunner(t *testing.T) {
	t.Parallel()
	ids, err := (OllamaLister{}).List(context.Background(), "ollama")
	if err != nil {
		if !strings.Contains(err.Error(), "ollama list") {
			t.Errorf("want wrapped 'ollama list' error, got %v", err)
		}
		if ids != nil {
			t.Errorf("ids must be nil on error, got %v", ids)
		}
	}
}

// TestCLIClassifier_NilRunDefaultsToExecRunner covers the `run == nil` default
// branch in Classify: with no injected Runner it shells out to the named CLI,
// which is absent in CI, so Classify returns the wrapped classifier error.
func TestCLIClassifier_NilRunDefaultsToExecRunner(t *testing.T) {
	t.Parallel()
	const absentCLI = "evolve-absent-cli-xyz"
	if _, err := exec.LookPath(absentCLI); err == nil {
		t.Skip("sentinel CLI unexpectedly present on PATH")
	}
	_, err := (CLIClassifier{CLI: absentCLI}).Classify(
		context.Background(), absentCLI, []string{"m1"})
	if err == nil {
		t.Fatal("want error when classifier CLI binary is absent")
	}
	if !strings.Contains(err.Error(), "classifier "+absentCLI) {
		t.Errorf("want wrapped classifier error, got %v", err)
	}
}

// TestCLIClassifier_AllObjectsFailToMap covers the loop's continue branch
// (json.Unmarshal failure on a non-object) AND the terminal "no JSON object
// mapped a tier" error: a reply with one malformed-typed object and one valid
// JSON object whose models are all hallucinated → no tier survives sanitize.
func TestCLIClassifier_AllObjectsFailToMap(t *testing.T) {
	t.Parallel()
	// First object has a non-string value → json.Unmarshal into map[string]string
	// fails → continue. Second object is valid JSON but maps tiers to models that
	// were never offered → sanitizeTierMap returns empty → loop continues → the
	// terminal error fires.
	reply := `{"fast":123}` + "\n" + `{"fast":"not-offered","deep":"also-not"}`
	s := &stubRunner{out: reply}
	_, err := (CLIClassifier{CLI: "codex", Run: s.run}).Classify(
		context.Background(), "codex", []string{"real-model"})
	if err == nil {
		t.Fatal("want error when no object maps a tier to an offered model")
	}
	if !strings.Contains(err.Error(), "no JSON object mapped a tier") {
		t.Errorf("want terminal mapping error, got %v", err)
	}
}

// TestTruncate_LongStringTruncates covers truncate's tail branch (len > n):
// it returns the first n runes plus an ellipsis. Multi-byte runes confirm the
// cut is rune-aware, not byte-aware.
func TestTruncate_LongStringTruncates(t *testing.T) {
	t.Parallel()
	got := truncate("ααααα", 3) // 5 two-byte runes, cap 3
	want := "ααα" + "…"
	if got != want {
		t.Errorf("truncate = %q, want %q", got, want)
	}
}

func TestCLIClassifierGuards(t *testing.T) {
	if _, err := (CLIClassifier{Run: (&stubRunner{}).run}).Classify(context.Background(), "codex", []string{"x"}); err == nil {
		t.Fatal("expected error when classifier CLI unset")
	}
	if _, err := (CLIClassifier{CLI: "codex", Run: (&stubRunner{}).run}).Classify(context.Background(), "codex", nil); err == nil {
		t.Fatal("expected error when no model ids")
	}
}
