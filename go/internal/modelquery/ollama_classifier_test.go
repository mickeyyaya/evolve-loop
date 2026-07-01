package modelquery

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// stubRunner returns canned output/err and records the last invocation,
// including stdin — TestOllamaListerReachesNoModel below asserts stdin is
// always empty for `ollama list` (the metadata-only, no-prompt guarantee).
type stubRunner struct {
	out       string
	err       error
	lastName  string
	lastArgs  []string
	lastStdin string
}

func (s *stubRunner) run(_ context.Context, name string, args []string, stdin string) (string, error) {
	s.lastName, s.lastArgs, s.lastStdin = name, args, stdin
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

// TestOllamaListerReachesNoModel is GAP 2's decision-(b) predicate (scout
// finding #3): `ollama list` is a non-interactive metadata enumeration, not a
// model-reaching dispatch, so C1 (every LLM-CLI control that reaches a model
// must go through the bridge) does not apply to it. This asserts the call
// site invokes ONLY `ollama list` — no prompt argument, no stdin — which is
// the structural difference between "enumerate what's installed" and
// "dispatch a prompt to a model".
func TestOllamaListerReachesNoModel(t *testing.T) {
	s := &stubRunner{out: "NAME\nllama3.3:latest  x  y  z\n"}
	if _, err := (OllamaLister{Run: s.run}).List(context.Background(), "ollama"); err != nil {
		t.Fatal(err)
	}
	if len(s.lastArgs) != 1 || s.lastArgs[0] != "list" {
		t.Fatalf("args = %v, want exactly [\"list\"] (no prompt argument)", s.lastArgs)
	}
	if s.lastStdin != "" {
		t.Fatalf("stdin = %q, want empty (no prompt piped to ollama)", s.lastStdin)
	}
}

// TestOllamaListMetadataExceptionDocumented pins GAP 2's required call-site
// comment (scout mailbox → Auditor: "ollama list is an ALLOWED metadata
// exception (must be commented + tested as no-model-reached)"). A grep-gamed
// magic string alone would be a degenerate predicate (cycle-85 lesson), so
// this test also runs alongside TestOllamaListerReachesNoModel, which
// exercises the actual no-prompt behavior the comment documents.
func TestOllamaListMetadataExceptionDocumented(t *testing.T) {
	src, err := os.ReadFile("ollama.go")
	if err != nil {
		t.Fatalf("read ollama.go: %v", err)
	}
	matched, err := regexp.MatchString(`(?i)metadata|no model|not model-reaching`, string(src))
	if err != nil {
		t.Fatalf("regexp: %v", err)
	}
	if !matched {
		t.Error("ollama.go must document its List call site as a metadata-only / non-model-reaching C1 exception")
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

// (The CLIClassifier prompt-echo regression guard lives in classifier_test.go
// now, exercised through the PromptDispatcher seam.)

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

// (classifierArgv and the exec-argv-based CLIClassifier tests are retired:
// GAP 1's fix removes classifierArgv from classifier.go entirely — the bridge's
// headless drivers (driver_codex.go/driver_claudep.go/driver_agy.go) already
// own the exact same per-CLI invocation shape it duplicated. See
// TestGuard_ClassifierHasNoDirectModelExec in classifier_test.go.)

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

// (TestCLIClassifier_AllObjectsFailToMap moved to classifier_test.go, rewired
// through the fakeDispatcher seam. The nil-Run-defaults-to-exec-runner
// scenario no longer applies: CLIClassifier has no exec fallback at all now —
// see TestCLIClassifierClassify_NilDispatcherErrorsNeverShellsOut.)

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

// (TestCLIClassifierGuards moved to classifier_test.go, rewired through the
// fakeDispatcher seam, and additionally asserts the dispatcher is never
// called when the pre-flight guards reject.)
