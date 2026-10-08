package stelint_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

func checkGo(t *testing.T, src string) []stelint.Finding {
	t.Helper()
	got, err := stelint.CheckGo([]byte(src), stelint.Options{Words: houseWords})
	if err != nil {
		t.Fatalf("CheckGo: %v\n%s", err, src)
	}
	return got
}

func goFile(imports, body string) string {
	return "package p\n\nimport (\n" + imports + "\n)\n\nfunc f(x Logger, s string, w io.Writer) {\n" + body + "\n}\n"
}

const goImports = "\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n\t\"log\"\n\t\"os\"\n\tstdlog \"log\""

func assertGoRules(t *testing.T, body string, want ...string) []stelint.Finding {
	t.Helper()
	got := checkGo(t, goFile(goImports, body))
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(rulesAt(got), want) {
		t.Fatalf("findings = %v, want %v\nbody:\n%s\nfull: %+v", rulesAt(got), want, body, got)
	}
	return got
}

func TestCheckGo_EachCallFormIsChecked(t *testing.T) {
	for name, call := range map[string]string{
		"fmt.Errorf":                        `_ = fmt.Errorf("we utilize %s", s)`,
		"errors.New":                        `_ = errors.New("we utilize it")`,
		"log.Printf":                        `log.Printf("we utilize %s", s)`,
		"log.Println":                       `log.Println("we utilize it")`,
		"log.Fatalf":                        `log.Fatalf("we utilize %v", s)`,
		"an aliased log import":             `stdlog.Printf("we utilize it")`,
		"fmt.Fprintf to stderr":             `fmt.Fprintf(os.Stderr, "we utilize %s\n", s)`,
		"fmt.Fprintf to stdout":             `fmt.Fprintf(os.Stdout, "we utilize %s\n", s)`,
		"fmt.Fprintln to stderr":            `fmt.Fprintln(os.Stderr, "we utilize it")`,
		"fmt.Fprintf to an injected stderr": `fmt.Fprintf(stderr, "we utilize %s", s)`,
		"fmt.Fprintln to a Stdout field":    `fmt.Fprintln(x.Stdout, "we utilize it")`,
		"fmt.Fprintf to an injected stdout": `fmt.Fprintf(a.stdout, "we utilize %s", s)`,
		"fmt.Printf":                        `fmt.Printf("we utilize %s\n", s)`,
		"a method Infof":                    `x.Infof("we utilize %s", s)`,
		"a method Warnf":                    `x.Warnf("we utilize %s", s)`,
		"a method Errorf":                   `x.Errorf("we utilize %s", s)`,
		"a method Printf":                   `x.Printf("we utilize %s", s)`,
		"a method Logf":                     `x.Logf("we utilize %s", s)`,
	} {
		t.Run(name, func(t *testing.T) {
			got := assertGoRules(t, "\t"+call, "13 STE-WORD")
			if !strings.Contains(got[0].Message, `"utilize"`) {
				t.Errorf("message %q must name the word", got[0].Message)
			}
		})
	}
}

func TestCheckGo_OtherCallsAreNotChecked(t *testing.T) {
	for name, call := range map[string]string{
		"fmt.Fprintf to a writer":    `fmt.Fprintf(w, "we utilize %s", s)`,
		"fmt.Sprintf":                `_ = fmt.Sprintf("we utilize %s", s)`,
		"a method not in the list":   `x.Debugf("we utilize %s", s)`,
		"a plain function":           `Errorf("we utilize %s", s)`,
		"a later argument":           `_ = fmt.Errorf("%s", "we utilize it")`,
		"a non-literal argument":     `_ = errors.New(s)`,
		"a concatenation with a var": `_ = errors.New("we utilize " + s)`,
		"a string outside a call":    `s = "we utilize it"`,
		"a New outside errors":       `_ = template.New("we utilize it")`,
		"fmt.Fprintf to os.Stdin":    `fmt.Fprintf(os.Stdin, "we utilize %s", s)`,
	} {
		t.Run(name, func(t *testing.T) {
			assertGoRules(t, "\t"+call)
		})
	}
}

func TestCheckGo_ConstantsAndConcatenationsResolve(t *testing.T) {
	t.Run("a package constant", func(t *testing.T) {
		src := goFile(goImports, "\t_ = errors.New(msg)") + "\nconst msg = \"we utilize it\"\n"
		if got := rulesAt(checkGo(t, src)); !reflect.DeepEqual(got, []string{"13 STE-WORD"}) {
			t.Fatalf("findings = %v, want the constant checked at the call", got)
		}
	})
	t.Run("a local constant format", func(t *testing.T) {
		assertGoRules(t, "\tconst format = \"we utilize %s\"\n\t_ = fmt.Errorf(format, s)", "14 STE-WORD")
	})
	t.Run("a concatenation of literals", func(t *testing.T) {
		assertGoRules(t, "\t_ = errors.New(\"first part, \" +\n\t\t\"we utilize it\")", "13 STE-WORD")
	})
	t.Run("a constant built from a concatenation", func(t *testing.T) {
		src := goFile(goImports, "\t_ = errors.New(msg)") + "\nconst prefix = \"first: \"\nconst msg = prefix + \"we utilize it\"\n"
		if got := rulesAt(checkGo(t, src)); !reflect.DeepEqual(got, []string{"13 STE-WORD"}) {
			t.Fatalf("findings = %v", got)
		}
	})
	t.Run("a name with two constants is skipped", func(t *testing.T) {
		src := goFile(goImports, "\tconst msg = \"fine\"\n\t_ = errors.New(msg)") + "\nfunc g() {\n\tconst msg = \"we utilize it\"\n\t_ = msg\n}\n"
		if got := checkGo(t, src); len(got) != 0 {
			t.Fatalf("an ambiguous constant must be skipped, got %+v", got)
		}
	})
}

func TestCheckGo_SentenceLengthCountsFormatVerbsAsWords(t *testing.T) {
	verbs := "%s %v %d %q %w %-10s %+v %#v %5.2f %x"
	assertGoRules(t, `_ = fmt.Errorf("`+words(15)+" "+verbs+`")`)
	got := assertGoRules(t, `_ = fmt.Errorf("`+words(16)+" "+verbs+`")`, "13 STE-SENTENCE")
	if !strings.Contains(got[0].Message, "26") {
		t.Errorf("message %q must count 26 words", got[0].Message)
	}
	assertGoRules(t, `_ = fmt.Errorf("`+words(24)+` %% done")`)
}

func TestCheckGo_TechnicalTokens(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"a key=value token is one technical word":   {`_ = fmt.Errorf("via=%s mode=via", s)`, nil},
		"a key=value token counts once":             {`_ = fmt.Errorf("` + words(24) + ` cycle=%d", 1)`, nil},
		"a bracket tag is technical":                {`log.Printf("[via] start")`, nil},
		"text after a bracket tag is checked":       {`log.Printf("[runner] we utilize it")`, []string{"13 STE-WORD"}},
		"an UPPER_SNAKE code is technical":          {`log.Printf("VIA_CODE raised")`, nil},
		"a code span is technical":                  {"log.Printf(\"run `via` now\")", nil},
		"a quotation is exempt":                     {`log.Printf("the operator said \"via\"")`, nil},
		"a long message over lines is one sentence": {`_ = errors.New("` + words(13) + `\n` + words(13) + `")`, []string{"13 STE-SENTENCE"}},
		"a full stop splits a message":              {`_ = errors.New("` + words(20) + `. ` + words(20) + `")`, nil},
	} {
		t.Run(name, func(t *testing.T) {
			assertGoRules(t, "\t"+tc.body, tc.want...)
		})
	}
}

func TestCheckGo_GeneratedFileIsReportedOnce(t *testing.T) {
	src := "// Code generated by a tool. DO NOT EDIT.\n\n" + goFile(goImports, "\t_ = errors.New(\"we utilize it\")")
	got := checkGo(t, src)
	if len(got) != 1 || got[0].Rule != stelint.RuleSkippedGenerated || !got[0].IsSkipNotice() {
		t.Fatalf("findings = %+v, want one generated notice", got)
	}
}

func TestCheckGo_AParseErrorIsAnError(t *testing.T) {
	if _, err := stelint.CheckGo([]byte("package p\nfunc {"), stelint.Options{}); err == nil {
		t.Fatal("CheckGo(broken source) must return an error")
	}
}

func TestCheckGo_IsDeterministic(t *testing.T) {
	src := []byte(goFile(goImports, "\t_ = errors.New(\"we utilize it via x\")\n\tlog.Printf(\""+words(30)+" via\")\n\tx.Logf(\"in order to win\")"))
	first, err := stelint.CheckGo(src, stelint.Options{Words: houseWords})
	if err != nil || len(first) < 4 {
		t.Fatalf("the fixture must produce several findings, got %+v, %v", first, err)
	}
	for i := range 20 {
		if got, _ := stelint.CheckGo(src, stelint.Options{Words: houseWords}); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d differs:\n%+v\nwant\n%+v", i, got, first)
		}
	}
}
