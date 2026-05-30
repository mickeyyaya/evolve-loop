package recipe

import (
	"errors"
	"testing"
)

func TestStepsFor(t *testing.T) {
	r := Recipe{
		Name:  "demo",
		Steps: []Step{{Name: "agnostic"}},
		PerCLI: map[string][]Step{
			"claude-tmux": {{Name: "claude-specific"}},
		},
	}
	t.Run("per-cli arm wins", func(t *testing.T) {
		steps, err := r.stepsFor("claude-tmux")
		if err != nil || len(steps) != 1 || steps[0].Name != "claude-specific" {
			t.Fatalf("got %v err=%v", steps, err)
		}
	})
	t.Run("falls back to agnostic steps", func(t *testing.T) {
		steps, err := r.stepsFor("agy-tmux")
		if err != nil || steps[0].Name != "agnostic" {
			t.Fatalf("got %v err=%v", steps, err)
		}
	})
	t.Run("no arm and no agnostic → ErrUnsupportedCLI", func(t *testing.T) {
		only := Recipe{Name: "x", PerCLI: map[string][]Step{"claude-tmux": {{Name: "a"}}}}
		_, err := only.stepsFor("ollama-tmux")
		if !errors.Is(err, ErrUnsupportedCLI) {
			t.Fatalf("err=%v want ErrUnsupportedCLI", err)
		}
	})
}

func TestMergeParams(t *testing.T) {
	r := Recipe{Params: []ParamDecl{
		{Name: "marketplace", Required: true},
		{Name: "scope", Default: "user"},
		{Name: "opt"},
	}}
	t.Run("caller value used; default applied; optional-unset omitted", func(t *testing.T) {
		out, err := r.mergeParams(Params{"marketplace": "ecc"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if out["marketplace"] != "ecc" || out["scope"] != "user" {
			t.Fatalf("merge=%v", out)
		}
		if _, ok := out["opt"]; ok {
			t.Errorf("unset optional should be absent, got %v", out["opt"])
		}
	})
	t.Run("caller overrides default", func(t *testing.T) {
		out, _ := r.mergeParams(Params{"marketplace": "ecc", "scope": "project"})
		if out["scope"] != "project" {
			t.Fatalf("scope=%q", out["scope"])
		}
	})
	t.Run("missing required → ErrMissingParam", func(t *testing.T) {
		_, err := r.mergeParams(Params{})
		if !errors.Is(err, ErrMissingParam) {
			t.Fatalf("err=%v want ErrMissingParam", err)
		}
	})
	t.Run("undeclared caller key ignored", func(t *testing.T) {
		out, _ := r.mergeParams(Params{"marketplace": "ecc", "bogus": "x"})
		if _, ok := out["bogus"]; ok {
			t.Errorf("undeclared key should be dropped")
		}
	})
}

func TestSubstitute(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		params  Params
		want    string
		wantErr bool
	}{
		{"single", "/install {{plugin}}", Params{"plugin": "ecc@ecc"}, "/install ecc@ecc", false},
		{"multiple", "{{a}}-{{b}}", Params{"a": "x", "b": "y"}, "x-y", false},
		{"repeat token", "{{p}} and {{p}}", Params{"p": "z"}, "z and z", false},
		{"whitespace in braces", "{{ plugin }}", Params{"plugin": "ecc"}, "ecc", false},
		{"no placeholders", "/reload-plugins", Params{}, "/reload-plugins", false},
		{"unresolved → error", "/install {{missing}}", Params{}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := substitute(tc.body, tc.params)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil {
				if !errors.Is(err, ErrUnknownParam) {
					t.Errorf("err not ErrUnknownParam: %v", err)
				}
				return
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
