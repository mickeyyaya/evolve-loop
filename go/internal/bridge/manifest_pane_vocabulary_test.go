package bridge

import (
	"io"
	"strings"
	"testing"
)

func paneVocabularyManifest(field, pattern string) []byte {
	return []byte(`{"cli":"x-tmux","binary":"x","` + field + `":` + jsonString(pattern) + `}`)
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

func TestParseManifest_RejectsABrokenPaneVocabularyPattern(t *testing.T) {
	cases := []struct {
		field, pattern, why string
	}{
		{"busy_line_regex", `^\s*([⣾⣽]`, "does not compile"},
		{"busy_line_regex", `^\s*[⣾⣽]\s+\S`, "captures no spinner frame"},
		{"token_line_regex", `Thought for \S+, ([0-9]+) tokens`, "names no count group"},
		{"token_line_regex", `Thought (?P<count>[0-9]+`, "does not compile"},
		{"model_label_regex", `\s{2,}(\S+ · \S+)$`, "names no model group"},
		{"model_label_regex", `(?P<model>`, "does not compile"},
	}
	for _, c := range cases {
		_, err := parseManifestWithStderr("x-tmux", paneVocabularyManifest(c.field, c.pattern), io.Discard)
		if err == nil || !strings.Contains(err.Error(), c.field) {
			t.Errorf("%s %q (%s): err = %v, want a load error naming the field", c.field, c.pattern, c.why, err)
		}
	}
}

func TestParseManifest_AcceptsWellFormedPaneVocabulary(t *testing.T) {
	for _, field := range []struct{ name, pattern string }{
		{"busy_line_regex", `^\s*([⣾⣽])\s+\S`},
		{"token_line_regex", `Thought for \S+, (?P<count>[0-9]+)(?P<scale>k?) tokens`},
		{"model_label_regex", `\s{2,}(?P<model>\S+ · \S+)$`},
	} {
		if _, err := parseManifestWithStderr("x-tmux", paneVocabularyManifest(field.name, field.pattern), io.Discard); err != nil {
			t.Errorf("%s: %v", field.name, err)
		}
	}
	if _, err := parseManifestWithStderr("x-tmux", []byte(`{"cli":"x-tmux","binary":"x"}`), io.Discard); err != nil {
		t.Errorf("a manifest that declares no pane vocabulary must still load: %v", err)
	}
}
