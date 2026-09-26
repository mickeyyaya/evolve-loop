package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestPresentCLIs(t *testing.T) {
	orig := installLookPath
	t.Cleanup(func() { installLookPath = orig })

	cases := []struct {
		name    string
		present map[string]bool
		want    []string
	}{
		{"none", map[string]bool{}, nil},
		{"claude only", map[string]bool{"claude": true}, []string{"claude"}},
		{"codex + agy", map[string]bool{"codex": true, "agy": true}, []string{"codex", "agy"}},
		{"all four", map[string]bool{"claude": true, "codex": true, "agy": true, "gemini": true}, []string{"claude", "codex", "agy", "gemini"}},
		{"keeps install order, not PATH order", map[string]bool{"gemini": true, "claude": true}, []string{"claude", "gemini"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installLookPath = func(bin string) (string, error) {
				if tc.present[bin] {
					return "/usr/bin/" + bin, nil
				}
				return "", errors.New("not found")
			}
			if got := presentCLIs(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("presentCLIs() = %v, want %v", got, tc.want)
			}
		})
	}
}
