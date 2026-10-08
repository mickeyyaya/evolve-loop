package stelint_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

var houseWords = []stelint.Substitution{
	{Phrase: "utilize", Replacement: "use"},
	{Phrase: "via", Replacement: "through, with, by"},
	{Phrase: "in order to", Replacement: "to"},
	{Phrase: "e.g.", Replacement: "for example"},
	{Phrase: "etc.", Replacement: "(list all the items)"},
	{Phrase: "vs.", Replacement: "against, compared with"},
	{Phrase: "et al.", Replacement: "and others"},
	{Phrase: "incl.", Replacement: "with"},
}

func words(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "word"
	}
	return strings.Join(parts, " ")
}

func sentence(n int) string {
	return words(n) + "."
}

func check(doc string) []stelint.Finding {
	return stelint.Check([]byte(doc), stelint.Options{Words: houseWords})
}

func rulesAt(fs []stelint.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%d %s", f.Line, f.Rule))
	}
	return out
}

func assertRules(t *testing.T, doc string, want ...string) []stelint.Finding {
	t.Helper()
	got := check(doc)
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(rulesAt(got), want) {
		t.Fatalf("findings = %v, want %v\ndoc:\n%s\nfull: %+v", rulesAt(got), want, doc, got)
	}
	return got
}

func TestCheck_SentenceLengthIsTwentyFiveWordsInText(t *testing.T) {
	for name, tc := range map[string]struct {
		doc  string
		want []string
	}{
		"exactly 25 words pass":                    {sentence(25) + "\n", nil},
		"26 words fail":                            {sentence(26) + "\n", []string{"1 STE-SENTENCE"}},
		"a line with no full stop is one sentence": {words(26) + "\n", []string{"1 STE-SENTENCE"}},
		"two short sentences on one line pass":     {sentence(20) + " " + sentence(20) + "\n", nil},
		"a question mark splits":                   {words(20) + "? " + sentence(20) + "\n", nil},
		"an exclamation mark splits":               {words(20) + "! " + sentence(20) + "\n", nil},
		"a bullet item keeps the 25-word limit":    {"- " + sentence(25) + "\n", nil},
		"a checked box is not a word":              {"- [x] " + sentence(25) + "\n", nil},
		"a sentence over two lines starts on its first line": {
			"Intro.\n\n" + words(10) + "\n" + sentence(16) + "\n",
			[]string{"3 STE-SENTENCE"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertRules(t, tc.doc, tc.want...)
		})
	}
}

func TestCheck_SentenceMessageNamesTheCountAndTheLimit(t *testing.T) {
	got := assertRules(t, sentence(31)+"\n", "1 STE-SENTENCE")
	if !strings.Contains(got[0].Message, "31") || !strings.Contains(got[0].Message, "25") {
		t.Errorf("message %q must name 31 words and the limit 25", got[0].Message)
	}
	if !strings.HasPrefix(got[0].Excerpt, "word word") {
		t.Errorf("excerpt %q must quote the start of the sentence", got[0].Excerpt)
	}
}

func TestCheck_ANumberedStepHasTwentyWords(t *testing.T) {
	for name, tc := range map[string]struct {
		doc  string
		want []string
	}{
		"exactly 20 words pass":          {"1. " + sentence(20) + "\n", nil},
		"21 words fail":                  {"1. " + sentence(21) + "\n", []string{"1 STE-SENTENCE"}},
		"a parenthesis marker is a step": {"2) " + sentence(21) + "\n", []string{"1 STE-SENTENCE"}},
		"the second step is checked on its own line": {
			"1. " + sentence(5) + "\n2. " + sentence(21) + "\n",
			[]string{"2 STE-SENTENCE"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := assertRules(t, tc.doc, tc.want...)
			if len(got) == 1 && got[0].Message != "a numbered item has 21 words; the limit is 20" {
				t.Errorf("message = %q, want %q", got[0].Message, "a numbered item has 21 words; the limit is 20")
			}
		})
	}
}

func TestCheck_SentenceSplitting(t *testing.T) {
	half := words(12)
	for name, tc := range map[string]struct {
		doc  string
		want []string
	}{
		"a decimal number does not split":                          {half + " 1.5 " + half + " end.\n", []string{"1 STE-SENTENCE"}},
		"a house abbreviation does not split":                      {half + " e.g. " + half + " end.\n", []string{"1 STE-SENTENCE", "1 STE-WORD"}},
		"a code span with a full stop does not split":              {half + " `a. b` " + half + " end.\n", []string{"1 STE-SENTENCE"}},
		"a path does not split":                                    {half + " docs/a.md " + half + " end.\n", []string{"1 STE-SENTENCE"}},
		"bold text that ends a sentence splits":                    {"**" + words(20) + ".** " + sentence(20) + "\n", nil},
		"a closing parenthesis after a full stop splits":           {"(" + words(20) + ".) " + sentence(20) + "\n", nil},
		"vs. does not split":                                       {half + " vs. " + half + " end.\n", []string{"1 STE-SENTENCE", "1 STE-WORD"}},
		"et al. does not split":                                    {half + " et al. " + half + " end.\n", []string{"1 STE-SENTENCE", "1 STE-WORD"}},
		"incl. does not split":                                     {half + " incl. " + half + " end.\n", []string{"1 STE-SENTENCE", "1 STE-WORD"}},
		"a quotation that ends with a full stop ends the sentence": {`He said "` + words(20) + `." ` + sentence(23) + "\n", nil},
	} {
		t.Run(name, func(t *testing.T) {
			assertRules(t, tc.doc, tc.want...)
		})
	}
}

func TestCheck_ACodeSpanIsOneWord(t *testing.T) {
	span := "`one two three four five`"
	assertRules(t, words(24)+" "+span+".\n")
	assertRules(t, words(25)+" "+span+".\n", "1 STE-SENTENCE")
}

func TestCheck_ParagraphHasSixSentences(t *testing.T) {
	para := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = sentence(3)
		}
		return strings.Join(s, " ")
	}
	for name, tc := range map[string]struct {
		doc  string
		want []string
	}{
		"exactly 6 sentences pass":                 {para(6) + "\n", nil},
		"7 sentences fail":                         {para(7) + "\n", []string{"1 STE-PARAGRAPH"}},
		"the finding names the first line":         {"Intro.\n\n" + para(4) + "\n" + para(3) + "\n", []string{"3 STE-PARAGRAPH"}},
		"a blank line ends a paragraph":            {para(4) + "\n\n" + para(4) + "\n", nil},
		"a list item is its own paragraph":         {"- " + para(4) + "\n- " + para(4) + "\n", nil},
		"a list item over its limit fails":         {"- " + para(2) + "\n- " + para(7) + "\n", []string{"2 STE-PARAGRAPH"}},
		"a table cell is its own paragraph":        {"| A | B |\n|---|---|\n| " + para(4) + " | " + para(4) + " |\n", nil},
		"a table cell over its limit fails":        {"| A | B |\n|---|---|\n| x | " + para(7) + " |\n", []string{"3 STE-PARAGRAPH"}},
		"a heading ends a paragraph":               {para(4) + "\n## Next\n" + para(4) + "\n", nil},
		"a piece without a word is not a sentence": {para(6) + " —\n", nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := assertRules(t, tc.doc, tc.want...)
			if len(got) == 1 && (!strings.Contains(got[0].Message, "7") || !strings.Contains(got[0].Message, "6")) {
				t.Errorf("message %q must name 7 sentences and the limit 6", got[0].Message)
			}
		})
	}
}

func TestCheck_HouseWords(t *testing.T) {
	for name, tc := range map[string]struct {
		doc  string
		want []string
	}{
		"a house word is found":                         {"We utilize it.\n", []string{"1 STE-WORD"}},
		"the match ignores case":                        {"Utilize it.\n", []string{"1 STE-WORD"}},
		"punctuation does not hide a word":              {"Send it (via the bridge).\n", []string{"1 STE-WORD"}},
		"a part of a longer word is not found":          {"A viable trivia question.\n", nil},
		"a phrase is found":                             {"Do it in order to win.\n", []string{"1 STE-WORD"}},
		"a part of a phrase is not found":               {"Put it in order now.\n", nil},
		"a phrase over a line break is found":           {"Do it in order\nto win.\n", []string{"1 STE-WORD"}},
		"an abbreviation is found":                      {"Use a tool, e.g. a hammer.\n", []string{"1 STE-WORD"}},
		"each use is found on its line":                 {"Go via one.\nGo via two.\n", []string{"1 STE-WORD", "2 STE-WORD"}},
		"a part of a hyphenated word is found":          {"A via-bridge path.\n", []string{"1 STE-WORD"}},
		"an UPPER_SNAKE name is technical":              {"Read VIA_CODE now.\n", nil},
		"a key=value token is technical":                {"Set mode=via now.\n", nil},
		"a bare URL is technical":                       {"See https://example.org/via now.\n", nil},
		"a bracket tag is technical":                    {"The [via] tag.\n", nil},
		"an unpaired straight quote is not a quotation": {"The pane is 80\" wide. We utilize the bridge.\n", []string{"1 STE-WORD"}},
		"an unpaired curly quote is not a quotation":    {"The label “draft is set. We utilize the bridge.\n", []string{"1 STE-WORD"}},
		"text after a curly quotation is checked":       {"He said “x” and we utilize it.\n", []string{"1 STE-WORD"}},
	} {
		t.Run(name, func(t *testing.T) {
			assertRules(t, tc.doc, tc.want...)
		})
	}
}

func TestCheck_WordMessageNamesTheApprovedAlternative(t *testing.T) {
	got := assertRules(t, "Go via the bridge.\n", "1 STE-WORD")
	if !strings.Contains(got[0].Message, `"via"`) || !strings.Contains(got[0].Message, "through, with, by") {
		t.Errorf("message %q must name the word and its alternative", got[0].Message)
	}
	if got[0].Excerpt != "via" {
		t.Errorf("excerpt = %q, want the word as written", got[0].Excerpt)
	}
}

func TestCheck_Exemptions(t *testing.T) {
	long := sentence(40)
	for name, tc := range map[string]struct {
		doc  string
		want []string
	}{
		"a backtick fence":                          {"```\n" + long + " utilize\n```\n", nil},
		"a tilde fence":                             {"~~~go\n" + long + " utilize\n~~~\n", nil},
		"an indented fence in a list":               {"- Step:\n\n  ```\n  " + long + " utilize\n  ```\n", nil},
		"text after a fence is checked":             {"```\nx\n```\nWe utilize it.\n", []string{"4 STE-WORD"}},
		"a code span":                               {"Run `utilize via` now.\n", nil},
		"a link URL":                                {"See [the guide](https://example.org/utilize/via) now.\n", nil},
		"a link URL with parentheses":               {"See [the guide](https://example.org/a_(utilize)) now.\n", nil},
		"a reference link label":                    {"See [the guide][utilize] now.\n", nil},
		"an autolink":                               {"See <https://example.org/utilize> now.\n", nil},
		"link text":                                 {"See [utilize this](https://example.org) now.\n", nil},
		"text after a link":                         {"See [the guide](https://e.org) and we utilize it.\n", []string{"1 STE-WORD"}},
		"a straight quotation":                      {`He said "utilize via" once.` + "\n", nil},
		"a curly quotation":                         {"He said “utilize via” once.\n", nil},
		"text after a quotation":                    {`He said "x" and utilize.` + "\n", []string{"1 STE-WORD"}},
		"a blockquote that starts with a quotation": {"> \"" + long + " utilize\n> more utilize text\"\n", nil},

		"YAML frontmatter":                              {"---\ntitle: " + long + " utilize\n---\nText.\n", nil},
		"YAML frontmatter closed with dots":             {"---\ntitle: we utilize it\n...\nText.\n", nil},
		"an HTML comment line":                          {"<!-- " + long + " utilize -->\n", nil},
		"an HTML comment block":                         {"<!--\n" + long + " utilize\n-->\nText.\n", nil},
		"an inline HTML comment":                        {"Text <!-- utilize --> here.\n", nil},
		"an HTML block":                                 {"<details>\n<summary>utilize via</summary>\n" + long + "\n\nWe utilize it.\n", []string{"5 STE-WORD"}},
		"an inline tag at the start of a line is prose": {"<b>Note:</b> we utilize it.\n", []string{"1 STE-WORD"}},
		"an inline HTML tag is not a word":              {words(24) + " <br> end.\n", nil},
	} {
		t.Run(name, func(t *testing.T) {
			assertRules(t, tc.doc, tc.want...)
		})
	}
}

func TestCheck_AQuotationIsOneWord(t *testing.T) {
	assertRules(t, words(24)+` "a b c d e" end.`+"\n", "1 STE-SENTENCE")
	assertRules(t, words(23)+` "a b c d e" end.`+"\n")
	assertRules(t, words(23)+" “a b c d e” end.\n")
}

func TestCheck_ABlockquoteIsProse(t *testing.T) {
	assertRules(t, "> "+sentence(26)+"\n", "1 STE-SENTENCE")
	assertRules(t, "> We utilize it.\n>\n> Go via it.\n", "1 STE-WORD", "3 STE-WORD")
	assertRules(t, "> - "+sentence(25)+"\n> 1. "+sentence(21)+"\n", "2 STE-SENTENCE")
	assertRules(t, "> “A quoted passage, we utilize it.”\n\nWe utilize it.\n", "3 STE-WORD")
	assertRules(t, "> \"We utilize the first paragraph of the passage.\n>\n> We utilize the second paragraph.\"\n\nGo via it.\n", "5 STE-WORD")
}

func TestCheck_AHeadingIsCheckedForWordsOnly(t *testing.T) {
	long := sentence(40)
	assertRules(t, "# "+long+" utilize\n", "1 STE-WORD")
	assertRules(t, "### Send it via the bridge ###\n", "1 STE-WORD")
	assertRules(t, long+" utilize\n===\n\nText.\n", "1 STE-WORD")
	assertRules(t, "## "+long+"\n")
}

func TestCheck_LinkTextCountsAndTheURLDoesNot(t *testing.T) {
	assertRules(t, words(23)+" [two words](https://example.org/a/b/c/d) end.\n", "1 STE-SENTENCE")
	assertRules(t, words(22)+" [two words](https://example.org/a b c d e) end.\n")
}

func TestCheck_OnlyTheStandardExemptsItsOwnWordTable(t *testing.T) {
	table := "| Do not write | Write |\n|---|---|\n| utilize | use |\n| via | through |\n"
	standard := func(doc string) []string {
		return rulesAt(stelint.Check([]byte(doc), stelint.Options{Words: houseWords, IsStandard: true}))
	}
	if got := standard("## " + stelint.WordTableHeading + "\n\n" + table); len(got) != 0 {
		t.Errorf("the standard's own table = %v, want no findings", got)
	}
	if got := standard("## " + stelint.WordTableHeading + "\n\n" + table + "\n## Next\n\n" + table); !reflect.DeepEqual(got, []string{"12 STE-WORD", "13 STE-WORD"}) {
		t.Errorf("a table under another heading of the standard = %v, want rows 12 and 13", got)
	}
	if got := standard("## Other words\n\n" + table); !reflect.DeepEqual(got, []string{"5 STE-WORD", "6 STE-WORD"}) {
		t.Errorf("another table of the standard = %v, want rows 5 and 6", got)
	}
	assertRules(t, "## "+stelint.WordTableHeading+"\n\n"+table, "5 STE-WORD", "6 STE-WORD")
}

func TestCheck_GeneratedText(t *testing.T) {
	long := sentence(40) + " utilize\n"
	t.Run("a generated file is reported once and not checked", func(t *testing.T) {
		got := assertRules(t, "<!-- Code generated by a tool. DO NOT EDIT. -->\n\n"+long+long, "1 "+stelint.RuleSkippedGenerated)
		if !got[0].IsSkipNotice() {
			t.Errorf("the generated notice must be a skip notice: %+v", got[0])
		}
	})
	t.Run("a header after frontmatter marks the file", func(t *testing.T) {
		assertRules(t, "---\nt: x\n---\n<!-- GENERATED by a tool -->\n"+long, "4 "+stelint.RuleSkippedGenerated)
	})
	t.Run("a marker after the first text does not mark the file", func(t *testing.T) {
		assertRules(t, "We utilize it.\n<!-- DO NOT EDIT -->\n", "1 STE-WORD")
	})
	t.Run("a marked region is skipped and reported once", func(t *testing.T) {
		doc := "We utilize it.\n\n<!-- GENERATED:a BEGIN — do not edit by hand -->\n" + long +
			"<!-- GENERATED:a END -->\n\n<!-- GENERATED:b BEGIN -->\n" + long + "<!-- GENERATED:b END -->\n\nGo via it.\n"
		got := assertRules(t, doc, "1 STE-WORD", "3 "+stelint.RuleSkippedGenerated, "11 STE-WORD")
		if !strings.Contains(got[1].Message, "2") {
			t.Errorf("the region notice %q must count the 2 regions", got[1].Message)
		}
	})
	t.Run("a region at the top does not mark the whole file", func(t *testing.T) {
		assertRules(t, "<!-- GENERATED:a BEGIN -->\nx\n<!-- GENERATED:a END -->\n\nWe utilize it.\n", "1 "+stelint.RuleSkippedGenerated, "5 STE-WORD")
	})
	t.Run("a marker in a code span is prose", func(t *testing.T) {
		assertRules(t, "The header `// Code generated … DO NOT EDIT.` stays. We utilize it.\n", "1 STE-WORD")
	})
}

func TestFinding_IsSkipNoticeOnlyForGeneratedText(t *testing.T) {
	for _, rule := range []string{stelint.RuleSentence, stelint.RuleParagraph, stelint.RuleWord} {
		if (stelint.Finding{Rule: rule}).IsSkipNotice() {
			t.Errorf("%s must count as a finding", rule)
		}
	}
	if !(stelint.Finding{Rule: stelint.RuleSkippedGenerated}).IsSkipNotice() {
		t.Error("STE-SKIPPED-GENERATED must be a skip notice")
	}
}

func TestCheck_FindingsAreSortedByLineThenRule(t *testing.T) {
	doc := "Go via it.\n\n" + strings.Repeat(sentence(3)+" ", 6) + "We utilize " + words(30) + ".\n"
	assertRules(t, doc, "1 STE-WORD", "3 STE-PARAGRAPH", "3 STE-SENTENCE", "3 STE-WORD")
}

func TestCheck_IsDeterministic(t *testing.T) {
	doc := []byte("Go via it.\n\n1. " + sentence(22) + "\n- utilize " + sentence(30) + "\n\n| a | " + strings.Repeat(sentence(2)+" ", 8) + "|\n")
	first := stelint.Check(doc, stelint.Options{Words: houseWords})
	if len(first) < 4 {
		t.Fatalf("the fixture must produce several findings, got %+v", first)
	}
	for i := range 20 {
		if got := stelint.Check(doc, stelint.Options{Words: houseWords}); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d differs:\n%+v\nwant\n%+v", i, got, first)
		}
	}
}
