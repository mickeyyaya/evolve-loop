package evidence

import (
	"reflect"
	"testing"
)

func TestTrailerValues_ReadsOnlyTheClosingTrailerBlock(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want []string
	}{
		{"closing block", "fix: a\n\nbody\n\nFix-Journey: j1\nCloses-Inbox: x\n", []string{"j1"}},
		{"repeated key", "fix: a\n\nFix-Journey: j1\nFix-Journey: j2", []string{"j1", "j2"}},
		{"key in the body is not a trailer", "fix: a\n\nFix-Journey: j1\n\nclosing prose here", nil},
		{"other keys only", "fix: a\n\nCloses-Inbox: x", nil},
		{"value is trimmed", "fix: a\n\nFix-Journey:   j1  ", []string{"j1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := TrailerValues(tc.msg, "Fix-Journey"); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("TrailerValues = %v, want %v", got, tc.want)
			}
		})
	}
}
