package practicewatch

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalize_PageDropsMarkupScriptsStylesAndDates(t *testing.T) {
	html := `<html><head><style>p{color:red}</style><script>var x = "<b>noise</b>";</script></head>
<body><!-- hidden --><h1>Changelog</h1><p>Released on 2026-10-12T09:30:00Z</p><p>Updated Oct 12, 2026 and 12 October 2026</p>
<p>Use   short   prompts &amp; clear goals.</p></body></html>`

	got := Normalize(KindPage, html)

	want := "Changelog\nReleased on\nUpdated and\nUse short prompts & clear goals."
	if got != want {
		t.Errorf("Normalize(page) = %q, want %q", got, want)
	}
}

func TestNormalize_PageIgnoresADateThatMoves(t *testing.T) {
	first := Normalize(KindPage, "<p>Last updated 2026-10-01</p><p>Same advice</p>")
	second := Normalize(KindPage, "<p>Last updated 2026-10-09</p><p>Same advice</p>")

	if first != second {
		t.Errorf("a moved date changed the page: %q vs %q", first, second)
	}
}

func TestNormalize_PageSeesATextChange(t *testing.T) {
	first := Normalize(KindPage, "<p>Use goals</p>")
	second := Normalize(KindPage, "<p>Use goals and limits</p>")

	if first == second {
		t.Errorf("a changed sentence left the normalized page unchanged: %q", first)
	}
}

func TestNormalize_ReleaseFeedKeepsTitlesAndDropsTimestamps(t *testing.T) {
	atom := `<feed><entry><updated>2026-10-12T00:00:00Z</updated><title>v2.1.300</title><content type="html">&lt;p&gt;Fix the prompt cache&lt;/p&gt;</content></entry></feed>`

	got := Normalize(KindGithubReleases, atom)

	if got != "v2.1.300\nFix the prompt cache" {
		t.Errorf("Normalize(releases) = %q", got)
	}
}

func TestNormalize_FileAndCommandKeepDatesAndDropBlankLinesAndCarriageReturns(t *testing.T) {
	text := "## 2.1.3 (2026-10-12)\r\n\r\n   - fixed   a bug  \r\n\r\n"

	for _, kind := range []Kind{KindGithubFile, KindCommand} {
		if got := Normalize(kind, text); got != "## 2.1.3 (2026-10-12)\n- fixed a bug" {
			t.Errorf("Normalize(%s) = %q", kind, got)
		}
	}
}

func FuzzNormalize(f *testing.F) {
	for _, seed := range []string{"", "<p>a</p>", "<script>", "<!--", "&amp;&lt;", "2026-10-12", "a\r\n\r\nb", "<<>>", "\xff\xfe"} {
		f.Add(seed)
	}
	kinds := []Kind{KindGithubFile, KindGithubReleases, KindPage, KindCommand}
	f.Fuzz(func(t *testing.T, text string) {
		for _, kind := range kinds {
			got := Normalize(kind, text)
			if got != Normalize(kind, text) {
				t.Fatalf("Normalize(%s) is not deterministic for %q", kind, text)
			}
			if strings.ContainsRune(got, '\r') {
				t.Fatalf("Normalize(%s) kept a carriage return: %q", kind, got)
			}
			if got == "" {
				continue
			}
			for _, line := range strings.Split(got, "\n") {
				if line == "" || line != strings.TrimSpace(line) {
					t.Fatalf("Normalize(%s) left a blank or untrimmed line %q in %q", kind, line, got)
				}
			}
			if utf8.ValidString(text) && !utf8.ValidString(got) {
				t.Fatalf("Normalize(%s) broke valid UTF-8 for %q", kind, text)
			}
		}
	})
}
