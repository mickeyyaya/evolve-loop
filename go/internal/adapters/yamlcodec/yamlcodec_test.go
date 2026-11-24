package yamlcodec_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
	"gopkg.in/yaml.v3"
)

type doc struct {
	Name  string   `yaml:"name"`
	Count int      `yaml:"count,omitempty"`
	Tags  []string `yaml:"tags"`
}

var errBoom = errors.New("boom")

type failing struct{}

func (failing) MarshalYAML() (any, error) { return nil, errBoom }

func TestMarshal_EmitsTheLibraryBytesForATaggedStruct(t *testing.T) {
	t.Parallel()
	in := []doc{{Name: "a", Count: 2, Tags: []string{"x", "y"}}, {Name: "b"}}

	got, err := yamlcodec.Marshal(in)
	want, wantErr := yaml.Marshal(in)

	if err != nil || wantErr != nil || string(got) != string(want) {
		t.Fatalf("Marshal = %q, %v; want %q, %v", got, err, want, wantErr)
	}
	if !strings.Contains(string(got), "- name: a\n  count: 2\n") {
		t.Errorf("Marshal = %q, want block-style entries honoring the yaml tags", got)
	}
}

func TestMarshal_ReportsTheLibraryErrorForAnUnsupportedValue(t *testing.T) {
	t.Parallel()
	_, err := yamlcodec.Marshal(failing{})

	if err == nil || err.Error() != errBoom.Error() {
		t.Fatalf("Marshal error = %v, want %v", err, errBoom)
	}
}

func TestUnmarshal_DecodesIntoTheTarget(t *testing.T) {
	t.Parallel()
	var got []doc

	err := yamlcodec.Unmarshal([]byte("- name: a\n  count: 2\n  tags: [x, y]\n"), &got)

	if err != nil || len(got) != 1 || got[0].Name != "a" || got[0].Count != 2 || len(got[0].Tags) != 2 {
		t.Fatalf("Unmarshal = %+v, %v; want one decoded doc", got, err)
	}
}

func TestUnmarshal_ReturnsTheLibraryErrorUnchanged(t *testing.T) {
	t.Parallel()
	var got []doc
	var want []doc
	input := []byte("- name: [unclosed\n")

	err := yamlcodec.Unmarshal(input, &got)
	wantErr := yaml.Unmarshal(input, &want)

	if err == nil || wantErr == nil || err.Error() != wantErr.Error() {
		t.Fatalf("Unmarshal error = %v, want %v", err, wantErr)
	}
}

func TestUnmarshalStrict_RefusesAnUnknownField(t *testing.T) {
	t.Parallel()
	var got doc

	err := yamlcodec.UnmarshalStrict([]byte("name: a\ncolour: red\n"), &got)

	if err == nil || !strings.Contains(err.Error(), "colour") {
		t.Fatalf("UnmarshalStrict error = %v, want a refusal that names the field colour", err)
	}
}

func TestUnmarshalStrict_DecodesAKnownDocument(t *testing.T) {
	t.Parallel()
	var got doc

	err := yamlcodec.UnmarshalStrict([]byte("name: a\ncount: 2\ntags: [x]\n"), &got)

	if err != nil || got.Name != "a" || got.Count != 2 || len(got.Tags) != 1 {
		t.Fatalf("UnmarshalStrict = %+v, %v; want the decoded doc", got, err)
	}
}

func TestUnmarshalStrict_RefusesAnEmptyDocument(t *testing.T) {
	t.Parallel()
	var got doc

	err := yamlcodec.UnmarshalStrict([]byte(""), &got)

	if err == nil {
		t.Fatal("UnmarshalStrict of an empty document = nil error, want a refusal")
	}
}

func TestDecodeAll_ReturnsEveryDocumentWithItsLines(t *testing.T) {
	t.Parallel()
	input := []byte("a: 1\nb:\n  - x\n  - y\n---\nc: 2\n---\n- one\n")

	docs, err := yamlcodec.DecodeAll(input)

	if err != nil || len(docs) != 3 {
		t.Fatalf("DecodeAll = %d docs, %v; want 3 docs", len(docs), err)
	}
	if docs[0].Kind() != yamlcodec.KindMapping || docs[0].Len() != 2 || docs[0].Line() != 1 {
		t.Errorf("doc 0 = kind %v len %d line %d; want a 2-key mapping on line 1", docs[0].Kind(), docs[0].Len(), docs[0].Line())
	}
	b, ok := docs[0].Get("b")
	if !ok || b.Kind() != yamlcodec.KindSequence || b.Len() != 2 || b.Line() != 3 {
		t.Errorf("Get(b) = kind %v len %d line %d ok %v; want a 2-item sequence on line 3", b.Kind(), b.Len(), b.Line(), ok)
	}
	if docs[1].Line() != 6 {
		t.Errorf("doc 1 line = %d, want 6", docs[1].Line())
	}
	if docs[2].Kind() != yamlcodec.KindSequence {
		t.Errorf("doc 2 kind = %v, want a sequence", docs[2].Kind())
	}
}

func TestDecodeAll_AnEmptyFileHasNoDocument(t *testing.T) {
	t.Parallel()

	docs, err := yamlcodec.DecodeAll([]byte(""))

	if err != nil || len(docs) != 0 {
		t.Fatalf("DecodeAll(empty) = %d docs, %v; want none", len(docs), err)
	}
}

func TestDecodeAll_ASyntaxErrorCarriesItsLine(t *testing.T) {
	t.Parallel()

	docs, err := yamlcodec.DecodeAll([]byte("ok: 1\n---\nkey: value: more\nnext: 2\n"))

	if err == nil || docs != nil {
		t.Fatalf("DecodeAll = %v, %v; want no docs and a syntax error", docs, err)
	}
	if err.Line != 3 || err.Msg == "" {
		t.Fatalf("error = %#v, want a *SyntaxError on line 3 with a message", err)
	}
	if got := err.Error(); got != "line 3: "+err.Msg {
		t.Errorf("Error() = %q, want line 3 then the message", got)
	}
}

func TestDecodeAll_AnErrorWithoutALineKeepsItsMessage(t *testing.T) {
	t.Parallel()

	_, err := yamlcodec.DecodeAll([]byte("a: \x01\n"))

	if err == nil || err.Line != 0 || err.Msg == "" {
		t.Fatalf("DecodeAll = %#v, want a *SyntaxError with line 0 and a message", err)
	}
	if got := err.Error(); got != err.Msg {
		t.Errorf("Error() = %q, want the bare message %q when no line is known", got, err.Msg)
	}
}

func TestNode_AScalarHasNoKeysOrLength(t *testing.T) {
	t.Parallel()
	docs, err := yamlcodec.DecodeAll([]byte("a: &x 1\nb: *x\nc: plain\n"))
	if err != nil {
		t.Fatal(err)
	}

	c, ok := docs[0].Get("c")
	_, missing := docs[0].Get("absent")
	_, onScalar := c.Get("anything")

	if !ok || c.Kind() != yamlcodec.KindScalar || c.Len() != 0 || c.Keys() != nil || missing || onScalar {
		t.Errorf("scalar c = kind %v len %d keys %v; missing=%v onScalar=%v", c.Kind(), c.Len(), c.Keys(), missing, onScalar)
	}
	if got := docs[0].Keys(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("Keys = %v, want [a b c] in file order", got)
	}
	if b, _ := docs[0].Get("b"); b.Kind() != yamlcodec.KindScalar {
		t.Errorf("an alias value has kind %v, want scalar", b.Kind())
	}
}

func TestNode_AnAliasResolvesToItsTarget(t *testing.T) {
	t.Parallel()
	docs, err := yamlcodec.DecodeAll([]byte("base: &x\n  runs-on: linux\n  n: 1\nseq: &s [a, b]\njob: *x\nlist: *s\n"))
	if err != nil {
		t.Fatal(err)
	}

	job, _ := docs[0].Get("job")
	list, _ := docs[0].Get("list")
	runner, ok := job.Get("runs-on")

	if job.Kind() != yamlcodec.KindMapping || job.Len() != 2 || !ok || runner.Kind() != yamlcodec.KindScalar {
		t.Errorf("alias job = kind %v len %d runs-on %v; want the 2-key mapping of its anchor", job.Kind(), job.Len(), ok)
	}
	if !slices.Equal(job.Keys(), []string{"runs-on", "n"}) || list.Kind() != yamlcodec.KindSequence || list.Len() != 2 {
		t.Errorf("alias keys = %v, list kind %v len %d; want the anchors' content", job.Keys(), list.Kind(), list.Len())
	}
}

func TestNode_AMergeKeyInheritsKeysAndExplicitKeysWin(t *testing.T) {
	t.Parallel()
	input := "defaults: &d {runs-on: linux, shell: bash}\nextra: &e {shell: zsh, timeout: 5}\n" +
		"single:\n  <<: *d\n  shell: sh\n" +
		"many:\n  <<: [*d, *e]\n  name: m\n" +
		"junk:\n  <<: [*d, plain, 3]\n" +
		"scalar:\n  <<: 7\n  own: 1\n"
	docs, err := yamlcodec.DecodeAll([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		keys []string
	}{
		{"single", []string{"shell", "runs-on"}},
		{"many", []string{"name", "runs-on", "shell", "timeout"}},
		{"junk", []string{"runs-on", "shell"}},
		{"scalar", []string{"own"}},
	}
	for _, tc := range cases {
		n, _ := docs[0].Get(tc.name)
		if !slices.Equal(n.Keys(), tc.keys) || n.Len() != len(tc.keys) {
			t.Errorf("%s keys = %v (len %d), want %v", tc.name, n.Keys(), n.Len(), tc.keys)
		}
	}
	single, _ := docs[0].Get("single")
	shell, _ := single.Get("shell")
	many, _ := docs[0].Get("many")
	manyShell, _ := many.Get("shell")
	if shell.Line() != 5 || manyShell.Line() != 1 {
		t.Errorf("shell lines = %d, %d; want the explicit key on 5 and the first merge source (line 1, from defaults) beating the later one", shell.Line(), manyShell.Line())
	}
}

func TestNode_ASelfMergeStopsAtTheDepthLimit(t *testing.T) {
	t.Parallel()
	docs, err := yamlcodec.DecodeAll([]byte("a: &a\n  <<: *a\n  k: 1\n"))
	if err != nil {
		t.Fatalf("DecodeAll of a self merge = %v, want it parsed", err)
	}

	a, _ := docs[0].Get("a")

	if !slices.Equal(a.Keys(), []string{"k"}) {
		t.Errorf("self merge keys = %v, want [k]", a.Keys())
	}
}

func TestNode_AMergeKeyInheritsThroughANestedMerge(t *testing.T) {
	t.Parallel()
	docs, err := yamlcodec.DecodeAll([]byte("a: &a {runs-on: linux}\nb: &b {<<: *a, name: b}\njob:\n  <<: *b\n"))
	if err != nil {
		t.Fatal(err)
	}

	job, _ := docs[0].Get("job")
	_, ok := job.Get("runs-on")

	if !ok || !slices.Equal(job.Keys(), []string{"name", "runs-on"}) {
		t.Errorf("job keys = %v, runs-on found = %v; want the key inherited through two merge levels", job.Keys(), ok)
	}
}

func TestSyntaxError_IsAnErrorWhoseTextNamesItsLineOnlyWhenKnown(t *testing.T) {
	t.Parallel()
	withLine := &yamlcodec.SyntaxError{Line: 7, Msg: "bad"}
	withoutLine := &yamlcodec.SyntaxError{Msg: "bad"}
	var asError error = withLine

	var target *yamlcodec.SyntaxError
	if !errors.As(asError, &target) || target.Line != 7 || target.Msg != "bad" {
		t.Errorf("errors.As = %#v, want the SyntaxError on line 7", target)
	}
	if len(asError.Error()) <= len(withoutLine.Error()) {
		t.Errorf("Error() = %q and %q, want the line to lengthen only the first", asError.Error(), withoutLine.Error())
	}
}

func TestNode_GetOfAMissingKeyReturnsTheZeroNodeAndFalse(t *testing.T) {
	t.Parallel()
	docs, syntaxErr := yamlcodec.DecodeAll([]byte("a: 1\n"))
	if syntaxErr != nil || len(docs) != 1 {
		t.Fatalf("DecodeAll = %d docs, %v", len(docs), syntaxErr)
	}
	var doc yamlcodec.Node = docs[0]

	got, ok := doc.Get("missing")

	if ok || got != (yamlcodec.Node{}) {
		t.Errorf("Get(missing) = %v, %v; want the zero Node and false", got, ok)
	}
	if _, ok := doc.Get("a"); !ok {
		t.Error("Get(a) = false, want the present key found")
	}
}
