package yamlcodec_test

import (
	"errors"
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
