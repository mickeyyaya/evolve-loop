package modelquery

import (
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func TestEnumTokens_NeedsTwoOrMoreEffortWordsAndRejectsAnyOtherToken(t *testing.T) {
	cases := []struct {
		name  string
		inner string
		want  []string
	}{
		{"comma list", "low, medium, high", []string{"low", "medium", "high"}},
		{"pipe list", "low|high", []string{"low", "high"}},
		{"mixed separators", "low, medium|high", []string{"low", "medium", "high"}},
		{"a single word is not an enum", "low", nil},
		{"empty parentheses", "", nil},
		{"only separators", ",|,", nil},
		{"one malformed token spoils the list", "low, 9bad, high", nil},
		{"prose in parentheses", "see the docs for details", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := enumTokens(tc.inner); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("enumTokens(%q) = %v, want %v", tc.inner, got, tc.want)
			}
		})
	}
}

func TestFirstEnumIn_SkipsParentheticalsThatAreNotEnumsAndFindsALaterOne(t *testing.T) {
	got := firstEnumIn("(see docs) (not, A Level) (low, high) (max, min)")
	if !reflect.DeepEqual(got, []string{"low", "high"}) {
		t.Fatalf("got %v", got)
	}
	for _, s := range []string{"", "no parens", "(unclosed low, high", "(a) (b"} {
		if got := firstEnumIn(s); got != nil {
			t.Errorf("firstEnumIn(%q) = %v, want nil", s, got)
		}
	}
}

func TestIncumbentFirst_MovesThePickToTheFrontWithoutMutatingOrReturnsTheSameLineage(t *testing.T) {
	lineage := []string{"a", "b", "c", "d"}

	moved := incumbentFirst(lineage, "c")
	if !reflect.DeepEqual(moved, []string{"c", "a", "b", "d"}) {
		t.Fatalf("moved = %v", moved)
	}
	if !reflect.DeepEqual(lineage, []string{"a", "b", "c", "d"}) {
		t.Errorf("input mutated: %v", lineage)
	}
	if front := incumbentFirst(lineage, "a"); !reflect.DeepEqual(front, lineage) {
		t.Errorf("front = %v", front)
	}
	absent := incumbentFirst(lineage, "zzz")
	if !reflect.DeepEqual(absent, lineage) || &absent[0] != &lineage[0] {
		t.Errorf("an absent pick returns the lineage itself: %v", absent)
	}
	if got := incumbentFirst(nil, "a"); got != nil {
		t.Errorf("nil lineage: %v", got)
	}
}

func TestParseDate_EachComponentThatFailsToParseVoidsTheWholeDate(t *testing.T) {
	boom := errors.New("not a number")
	cases := []struct {
		name   string
		failOn string
	}{
		{"year", "2025"},
		{"month", "07"},
		{"day", "31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := parseDateAtoi
			t.Cleanup(func() { parseDateAtoi = orig })
			parseDateAtoi = func(s string) (int, error) {
				if s == tc.failOn {
					return 0, boom
				}
				return strconv.Atoi(s)
			}
			if got := parseDate("claude-opus-2025-07-31"); got.ok || got.value != 0 {
				t.Fatalf("parseDate with a failing %s = %+v, want the zero dateVersion", tc.name, got)
			}
		})
	}
}

func TestParseDate_ComponentsComposeIntoAComparableNumber(t *testing.T) {
	cases := map[string]dateVersion{
		"m-2025-07-31": {ok: true, value: 20250731},
		"m-2025-07":    {ok: true, value: 20250700},
		"m-2025":       {ok: true, value: 20250000},
		"m-latest":     {},
		"":             {},
	}
	for id, want := range cases {
		if got := parseDate(id); got != want {
			t.Errorf("parseDate(%q) = %+v, want %+v", id, got, want)
		}
	}
}
