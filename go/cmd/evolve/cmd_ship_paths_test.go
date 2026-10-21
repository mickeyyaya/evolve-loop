package main

import (
	"reflect"
	"testing"
)

func TestSplitShipPaths(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"a.go", []string{"a.go"}},
		{" a.go , pkg/b.go,,", []string{"a.go", "pkg/b.go"}},
	}
	for _, tc := range cases {
		if got := splitShipPaths(tc.raw); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitShipPaths(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
