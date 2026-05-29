package keyspec

import (
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		token string
		want  Class
	}{
		{"Enter", ClassNamed},
		{"escape", ClassNamed}, // case-insensitive
		{"Esc", ClassNamed},
		{"Tab", ClassNamed},
		{"Up", ClassNamed},
		{"PgUp", ClassNamed},
		{"F12", ClassNamed},
		{"C-c", ClassModifier},
		{"M-x", ClassModifier},
		{"S-Tab", ClassModifier},     // modifier + named remainder
		{"C-M-Enter", ClassModifier}, // stacked modifiers
		{"C-Excape", ClassSuspect},   // modifier + mistyped remainder
		{"y", ClassLiteral},
		{"hello", ClassLiteral}, // lowercase literal text
		{"3", ClassLiteral},
		{"Excape", ClassSuspect}, // looks like a key name, isn't
		{"Etner", ClassSuspect},
		{"", ClassLiteral},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			if got := Classify(tc.token); got != tc.want {
				t.Errorf("Classify(%q)=%d want %d", tc.token, got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"all valid named seq", "Escape Enter", nil},
		{"literal + named", "y Enter", nil},
		{"modifier", "C-c", nil},
		{"one typo", "Excape", []string{"Excape"}},
		{"typo amid valid", "y Etner Enter", []string{"Etner"}},
		{"multiple typos", "Excape Etner", []string{"Excape", "Etner"}},
		{"empty body", "", nil},
		{"whitespace only", "   ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Validate(tc.body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate(%q)=%v want %v", tc.body, got, tc.want)
			}
		})
	}
}
