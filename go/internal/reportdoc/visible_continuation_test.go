package reportdoc

import (
	"reflect"
	"testing"
)

func TestVisibleLines_IndentedLineIsCodeOnlyWhenItCannotContinueAParagraph(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"four-space paragraph continuation", "text\n    more\n", []string{"text", "    more", ""}},
		{"tab paragraph continuation", "text\n\tmore\n", []string{"text", "\tmore", ""}},
		{"list item continuation", "- item\n    more\n", []string{"- item", "    more", ""}},
		{"nested list item continuation", "- item\n  - sub\n      more\n", []string{"- item", "  - sub", "      more", ""}},
		{"code after blank line", "text\n\n    code\n", []string{"text", "", ""}},
		{"code at document start", "    code\ntext\n", []string{"text", ""}},
		{"code after setext underline", "Title\n---\n    code\n", []string{"Title", "---", ""}},
		{"code after thematic break", "text\n\n***\n    code\n", []string{"text", "", "***", ""}},
		{"code after heading", "## H\n    code\n", []string{"## H", ""}},
		{"second indented line of a code block", "text\n\n    a\n    b\n", []string{"text", "", ""}},
		{"indented line after fence", "text\n```\nx\n```\n    code\n", []string{"text", ""}},
		{"fenced code unchanged", "text\n```\n    more\n```\nafter\n", []string{"text", "after", ""}},
		{"indented line after comment-only line", "text\n<!-- c -->\n    code\n", []string{"text", "", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := visibleLines(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("visibleLines(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSection_IndentedContinuationReachesTheSectionBody(t *testing.T) {
	t.Parallel()
	doc := "## Changed Areas\n- `a.go` — splits\n    checkAll into steps.\n\n    hidden example\n\n## Next\nx\n"
	body, ok, err := Section(doc, "Changed Areas")
	if err != nil {
		t.Fatalf("Section: %v", err)
	}
	if !ok {
		t.Fatalf("Section not found")
	}
	want := "- `a.go` — splits\n    checkAll into steps."
	if body != want {
		t.Fatalf("Section body = %q, want %q", body, want)
	}
}
