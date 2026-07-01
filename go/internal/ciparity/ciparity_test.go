package ciparity

import (
	"reflect"
	"testing"
)

func TestIntersectEnforced(t *testing.T) {
	enforce := []byte("# comment line\n" +
		"./internal/config\n" +
		"\n" +
		"./internal/router\n" +
		"  ./internal/policy  \n" + // leading/trailing space tolerated
		"./internal/bridge/panestream\n")

	cases := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			name:    "touched enforced packages are returned (strip /... and dedupe+sort)",
			changed: []string{"./internal/router/...", "./internal/config/...", "./internal/router/..."},
			want:    []string{"./internal/config", "./internal/router"},
		},
		{
			name:    "a touched package NOT in the enforce list is dropped",
			changed: []string{"./internal/router/...", "./internal/notenforced/..."},
			want:    []string{"./internal/router"},
		},
		{
			name:    "whitespace-padded enforce line still matches",
			changed: []string{"./internal/policy/..."},
			want:    []string{"./internal/policy"},
		},
		{
			name:    "no touched package is enforced => nil (caller skips apicover)",
			changed: []string{"./internal/notenforced/...", "./cmd/evolve/..."},
			want:    nil,
		},
		{
			name:    "empty changed set => nil (best-effort locator returned nothing)",
			changed: nil,
			want:    nil,
		},
		{
			name:    "comment and blank lines in the enforce file are never matched",
			changed: []string{"./comment/...", "./..."},
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IntersectEnforced(tc.changed, enforce)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("IntersectEnforced(%v) = %v, want %v", tc.changed, got, tc.want)
			}
		})
	}
}
