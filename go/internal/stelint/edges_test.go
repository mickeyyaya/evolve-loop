package stelint_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

type ruleCase struct {
	doc  string
	want []string
}

func runRuleCases(t *testing.T, cases map[string]ruleCase) {
	t.Helper()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertRules(t, tc.doc, tc.want...)
		})
	}
}

func TestCheck_RuleLinesAndSetextUnderlines(t *testing.T) {
	runRuleCases(t, map[string]ruleCase{
		"a star rule ends a paragraph":                  {words(13) + "\n***\n" + sentence(13) + "\n", nil},
		"an underscore rule ends a paragraph":           {words(13) + "\n___\n" + sentence(13) + "\n", nil},
		"a spaced star rule ends a paragraph":           {words(13) + "\n* * *\n" + sentence(13) + "\n", nil},
		"a rule with no paragraph before is skipped":    {"***\n" + sentence(25) + "\n", nil},
		"three dashes make the text a heading":          {words(30) + "\n---\n" + sentence(13) + "\n", nil},
		"three equals signs make the text a heading":    {words(30) + "\n===\n" + sentence(13) + "\n", nil},
		"three dashes after a bullet item are a rule":   {"- " + sentence(30) + "\n---\n", []string{"1 STE-SENTENCE"}},
		"three dashes after a numbered step are a rule": {"1. " + sentence(21) + "\n---\n", []string{"1 STE-SENTENCE"}},
		"three dashes after a checkbox item are a rule": {"- [ ] " + sentence(26) + "\n---\n" + sentence(13) + "\n", []string{"1 STE-SENTENCE"}},
		"two dashes are text in the paragraph":          {words(13) + "\n--\n" + sentence(13) + "\n", []string{"1 STE-SENTENCE"}},
		"two stars are text in the paragraph":           {words(13) + "\n**\n" + sentence(13) + "\n", []string{"1 STE-SENTENCE"}},
		"a lone pair of stars does not end a sentence":  {words(13) + " ** " + sentence(13) + "\n", []string{"1 STE-SENTENCE"}},
	})
}

func TestCheck_Frontmatter(t *testing.T) {
	runRuleCases(t, map[string]ruleCase{
		"a closed frontmatter is skipped":            {"---\ntitle: " + words(30) + "\n---\nText.\n", nil},
		"a frontmatter closed by dots is skipped":    {"---\ntitle: " + words(30) + "\n...\nText.\n", nil},
		"a CRLF frontmatter is skipped":              {"---\r\ntitle: " + words(30) + "\r\n---\r\nText.\r\n", nil},
		"an unclosed frontmatter is checked as text": {"---\ntitle: " + words(30) + "\n", []string{"2 STE-SENTENCE"}},
		"a rule after line one is not a frontmatter": {"\n---\ntitle: " + words(30) + "\n...\n", []string{"3 STE-SENTENCE"}},
		"an empty document has no finding":           {"", nil},
		"a lone frontmatter fence has no finding":    {"---", nil},
	})
}

func TestCheck_HTMLBlocks(t *testing.T) {
	runRuleCases(t, map[string]ruleCase{
		"an open block tag skips to the blank line":   {"<div\n" + sentence(30) + "\n\n" + sentence(26) + "\n", []string{"4 STE-SENTENCE"}},
		"a block tag with attributes skips its lines": {"<div class=\"x\">\nWe utilize it.\n", nil},
		"an uppercase block tag skips its lines":      {"<DIV>\nWe utilize it.\n", nil},
		"an inline tag at the line start is text":     {"<span\n" + sentence(30) + "\n", []string{"1 STE-SENTENCE"}},
		"a line that is only a bracket is text":       {"<\nWe utilize it.\n", []string{"2 STE-WORD"}},
		"a block comment skips to its close":          {"<!--\nWe utilize it.\n-->\nGo via it.\n", []string{"4 STE-WORD"}},
	})
}

func TestCheck_InlineSyntax(t *testing.T) {
	runRuleCases(t, map[string]ruleCase{
		"a closed code span is exempt":                 {"`We utilize it.`\n", nil},
		"a lone unclosed backtick is not a word":       {words(25) + " `\n", nil},
		"a tag at the line end is not a word":          {words(25) + " <br>\n", nil},
		"a bracket before text is not a link":          {"We [we utilize it] now.\n", []string{"1 STE-WORD"}},
		"an escaped full stop still ends a sentence":   {words(13) + " end\\. " + sentence(12) + "\n", nil},
		"an unclosed backtick is text":                 {"`We utilize it.\n", []string{"1 STE-WORD"}},
		"a double span keeps a single backtick inside": {"``code ` utilize`` then we utilize it.\n", []string{"1 STE-WORD"}},
		"an unclosed double backtick is text":          {"``utilize` text.\n", []string{"1 STE-WORD"}},
		"an inline comment is exempt":                  {"Text <!-- we utilize --> and we utilize it.\n", []string{"1 STE-WORD"}},
		"an unclosed inline comment hides the rest":    {"Text <!-- we utilize it.\n", nil},
		"a broken comment opener is text":              {"A <!-x utilize> b.\n", []string{"1 STE-WORD"}},
		"a less-than sign with no close is text":       {"If a < b we utilize it.\n", []string{"1 STE-WORD"}},
		"an autolink is exempt":                        {"See <https://example.org/utilize> now.\n", nil},
		"a mail link is exempt":                        {"Mail <mailto:utilize@example.org> now.\n", nil},
		"a tag with attributes is exempt":              {"<span data utilize>Text.</span>\n", nil},
		"text inside inline tags is checked":           {"<em>We utilize it.</em>\n", []string{"1 STE-WORD"}},
		"a self-closing tag is not a word":             {words(12) + " <br/> " + sentence(13) + "\n", nil},
		"an uppercase tag is not a word":               {words(12) + " <BR> " + sentence(13) + "\n", nil},
		"an unknown tag is a word":                     {words(12) + " <foo> " + sentence(13) + "\n", []string{"1 STE-SENTENCE"}},
		"link text is exempt":                          {"[We utilize it](x.md).\n", nil},
		"reference link text is exempt":                {"[We utilize it][ref].\n", nil},
		"nested brackets stay in the link text":        {"[a [we utilize it] b](x.md) now.\n", nil},
		"a bracket at the line end is not a link":      {"We [we utilize it]\n", []string{"1 STE-WORD"}},
		"an unclosed link target is not a link":        {"We [we utilize it](x.md now.\n", []string{"1 STE-WORD"}},
		"an unclosed reference is not a link":          {"We [we utilize it][ref now.\n", []string{"1 STE-WORD"}},
		"an unclosed bracket is not a link":            {"We [utilize it now.\n", []string{"1 STE-WORD"}},
		"text after a link is checked":                 {"[a](x.md) we utilize it.\n", []string{"1 STE-WORD"}},
		"an escaped bracket is not a link":             {"\\[We utilize it](x.md).\n", []string{"1 STE-WORD"}},
		"an escaped backtick is not a code span":       {"\\`We utilize it\\`.\n", []string{"1 STE-WORD"}},
		"an escaped symbol is not a tag":               {"\\<span data utilize>.\n", []string{"1 STE-WORD"}},
		"a backslash before a letter is text":          {"We \\utilize it.\n", []string{"1 STE-WORD"}},
		"a backslash at the line end is text":          {"We utilize it.\\\n", []string{"1 STE-WORD"}},
		"an escaped pipe stays in its table cell":      {"| A | B |\n|---|---|\n| " + words(13) + " \\| " + sentence(13) + " | x |\n", []string{"3 STE-SENTENCE"}},
		"a code span keeps its pipe in the table cell": {"| A | B |\n|---|---|\n| " + words(13) + " `|` " + sentence(12) + " | x |\n", []string{"3 STE-SENTENCE"}},
		"a long unicode sentence counts its words":     {strings.Repeat("café ", 26) + "fin.\n", []string{"1 STE-SENTENCE"}},
	})
}

func TestCheck_AnEmptyPhraseInTheWordListIsIgnored(t *testing.T) {
	opt := stelint.Options{Words: []stelint.Substitution{{Phrase: "  ", Replacement: "x"}, {Phrase: "", Replacement: "y"}, {Phrase: "utilize", Replacement: "use"}}}

	got := stelint.Check([]byte("We utilize it.\n"), opt)

	if len(got) != 1 || got[0].Rule != stelint.RuleWord || got[0].Message != `replace "utilize" with "use"` {
		t.Fatalf("findings = %+v, want only the utilize finding", got)
	}
}

func TestCheck_AnAbbreviationOnlyEndsASentenceWhenNotListed(t *testing.T) {
	doc := words(10) + " e.g. " + sentence(15) + "\n"
	withList := stelint.Check([]byte(doc), stelint.Options{Words: houseWords})
	withoutList := stelint.Check([]byte(doc), stelint.Options{})

	if got := rulesAt(withList); len(got) != 2 || got[0] != "1 STE-SENTENCE" || got[1] != "1 STE-WORD" {
		t.Errorf("with the house list, e.g. must not end the sentence: %v", got)
	}
	if len(withoutList) != 0 {
		t.Errorf("without the list, e.g. ends the sentence: %+v", withoutList)
	}
}

func TestCheckGo_ReceiversAndArguments(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"a log method on a field is checked":          {`x.l.Infof("we utilize %s", s)`, []string{"13 STE-WORD"}},
		"a log method on a call result is checked":    {`newLogger().Warnf("we utilize %s", s)`, []string{"13 STE-WORD"}},
		"a New on a field is not checked":             {`_ = x.errors.New("we utilize it")`, nil},
		"fmt.Fprintf to a pointer is not checked":     {`fmt.Fprintf(&b, "we utilize %s", s)`, nil},
		"fmt.Fprintf to a call result is not checked": {`fmt.Fprintf(out(), "we utilize %s", s)`, nil},
		"fmt.Fprintln to stdout is checked":           {`fmt.Fprintln(os.Stdout, "we utilize it")`, []string{"13 STE-WORD"}},
		"fmt.Fprint to a local stderr is checked":     {`fmt.Fprint(stderr, "we utilize it")`, []string{"13 STE-WORD"}},
		"fmt.Fprintf with only a writer is skipped":   {`fmt.Fprintf(os.Stderr)`, nil},
		"a call with no argument is skipped":          {`log.Printf()`, nil},
		"a parenthesized literal is checked":          {`_ = errors.New(("we utilize it"))`, []string{"13 STE-WORD"}},
		"a product of strings is not text":            {`_ = errors.New("we utilize " * "it")`, nil},
		"a number is not text":                        {`log.Printf(7)`, nil},
		"a raw string is checked":                     {"_ = errors.New(`we utilize it`)", []string{"13 STE-WORD"}},
		"an indexed verb is one code word":            {`_ = fmt.Errorf("go %[1]via it", s)`, nil},
		"an unclosed verb index is text":              {`_ = fmt.Errorf("go %[via it", s)`, []string{"13 STE-WORD"}},
		"an empty verb index is part of the verb":     {`_ = fmt.Errorf("go %[]via it", s)`, nil},
		"flags, width and precision make one verb":    {`_ = fmt.Errorf("go %-[1]*.2[3]via it", s)`, nil},
		"a percent sign at the end is text":           {`_ = fmt.Errorf("we utilize 100%")`, []string{"13 STE-WORD"}},
		"a percent sign before a space is text":       {`_ = fmt.Errorf("50% via it")`, []string{"13 STE-WORD"}},
		"a percent sign before a letter is a verb":    {`_ = fmt.Errorf("go %via it")`, nil},
		"a doubled percent sign is text":              {`_ = fmt.Errorf("go %%via it")`, []string{"13 STE-WORD"}},
		"a backslash is not an escape in Go text":     {`_ = fmt.Errorf("go \\[via](x)")`, []string{"13 STE-WORD"}},
	} {
		t.Run(name, func(t *testing.T) {
			assertGoRules(t, "\t"+tc.body, tc.want...)
		})
	}
}

func TestCheckGo_AnAliasedImportIsResolvedByItsPath(t *testing.T) {
	imports := "\te \"errors\"\n\tf \"fmt\"\n\t\"io\"\n\t\"os\""
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"an aliased errors.New is checked":         {`_ = e.New("we utilize it")`, []string{"11 STE-WORD"}},
		"an aliased fmt.Fprintln is checked":       {`f.Fprintln(os.Stderr, "we utilize it")`, []string{"11 STE-WORD"}},
		"the package name of an alias is not used": {`_ = errors.New("we utilize it")`, nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := rulesAt(checkGo(t, goFile(imports, "\t"+tc.body)))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings = %v, want %v", got, tc.want)
			}
		})
	}
}

func constChain(hops int) string {
	var b strings.Builder
	for i := 0; i < hops; i++ {
		fmt.Fprintf(&b, "const c%02d = c%02d\n", i, i+1)
	}
	fmt.Fprintf(&b, "const c%02d = \"we utilize it\"\n", hops)
	return b.String()
}

func TestCheckGo_ConstantChainsStopAtTheDepthLimit(t *testing.T) {
	for name, tc := range map[string]struct {
		hops int
		want []string
	}{
		"no hop resolves":    {0, []string{"13 STE-WORD"}},
		"seven hops resolve": {7, []string{"13 STE-WORD"}},
		"eight hops stop":    {8, nil},
		"twenty hops stop":   {20, nil},
	} {
		t.Run(name, func(t *testing.T) {
			src := goFile(goImports, "\t_ = errors.New(c00)") + "\n" + constChain(tc.hops)

			got := rulesAt(checkGo(t, src))

			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings = %v, want %v\n%s", got, tc.want, src)
			}
		})
	}
}

func TestCheckGo_CyclicConstantsTerminate(t *testing.T) {
	src := goFile(goImports, "\t_ = errors.New(a + \"we utilize it\")") + "\nconst a = b\nconst b = a\n"

	if got := checkGo(t, src); len(got) != 0 {
		t.Fatalf("a cyclic constant cannot resolve, got %+v", got)
	}
}

func TestLintFile_AGoParseErrorNamesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.go")
	writeFile(t, path, "package a\nfunc {")

	got, err := stelint.LintFile(path, stelint.Options{Words: houseWords})

	if err == nil || got != nil || !strings.HasPrefix(err.Error(), path+": parse Go source:") {
		t.Fatalf("LintFile = %+v, %v; want no findings and an error naming %s", got, err, path)
	}
}
