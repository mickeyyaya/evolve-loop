package modelquery

import (
	"strings"
	"testing"
)

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

func TestSanitizeTierMap(t *testing.T) {
	offered := []string{"m-fast", "m-bal", "m-deep"}
	parsed := map[string]string{
		"fast":     "m-fast",
		"balanced": "m-bal",
		"deep":     "hallucinated",
		"ultra":    "m-deep",
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

func TestErrNoLister_Error(t *testing.T) {
	t.Parallel()
	got := errNoLister("ollama").Error()
	want := "modelquery: no lister for cli ollama"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestClipOutput_TrimsAndClipsToTheNamedLimit(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("α", OutputClip+5)
	if got, want := ClipOutput("  "+long+"\n"), strings.Repeat("α", OutputClip)+"…"; got != want {
		t.Errorf("long output not clipped to %d runes: %d runes", OutputClip, len([]rune(got)))
	}
	if got := ClipOutput(" short \n"); got != "short" {
		t.Errorf("short output = %q, want it trimmed and whole", got)
	}
}
