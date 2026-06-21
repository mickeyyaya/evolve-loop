package render

import (
	"strings"
	"testing"
)

func TestDict_BuildsMap(t *testing.T) {
	m, err := dict("a", 1, "b", "two")
	if err != nil {
		t.Fatalf("dict: %v", err)
	}
	if m["a"] != 1 || m["b"] != "two" {
		t.Errorf("dict = %#v, want {a:1 b:two}", m)
	}
}

func TestDict_OddArgsFailLoudly(t *testing.T) {
	if _, err := dict("a"); err == nil {
		t.Fatal("dict(odd) = nil error, want error")
	}
}

func TestDict_NonStringKeyFailsLoudly(t *testing.T) {
	if _, err := dict(1, "a"); err == nil {
		t.Fatal("dict(non-string key) = nil error, want error")
	}
}

// Rendering must put the data on the page verbatim.
func TestRender_IncludesData(t *testing.T) {
	type model struct {
		Name string
		N    int
	}
	out, err := Render("Hello {{.Name}} v{{.N}}", model{Name: "Evolve", N: 42})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := string(out); !strings.Contains(got, "Hello Evolve v42") {
		t.Errorf("output %q missing expected text", got)
	}
}

// Strict mode: a missing map key must error, never silently emit <no value>.
func TestRender_MissingKeyFailsLoudly(t *testing.T) {
	_, err := Render("{{.missing}}", map[string]any{})
	if err == nil {
		t.Fatal("Render(missing key) = nil error, want strict error")
	}
}

func TestRender_ParseErrorFailsLoudly(t *testing.T) {
	if _, err := Render("{{.unterminated", nil); err == nil {
		t.Fatal("Render(bad template) = nil error, want parse error")
	}
}

// inc supports 1-based numbering in templates.
func TestInc(t *testing.T) {
	if FuncMap()["inc"].(func(int) int)(0) != 1 {
		t.Error("inc(0) != 1")
	}
}
