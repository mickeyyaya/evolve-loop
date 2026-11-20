package testaudit

import (
	"reflect"
	"testing"
)

func TestAuditScope_MapsEachChangedPathToItsPackageOrTheWholeTree(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		changed []string
		want    Scope
	}{
		{"go file", []string{"go/internal/core/a.go"}, Scope{Packages: []string{"internal/core"}}},
		{"test file", []string{"go/internal/core/a_test.go"}, Scope{Packages: []string{"internal/core"}}},
		{"two files of one package", []string{"go/internal/core/a.go", "go/internal/core/b_test.go"}, Scope{Packages: []string{"internal/core"}}},
		{"packages sorted", []string{"go/z/a.go", "go/b/a.go"}, Scope{Packages: []string{"b", "z"}}},
		{"module root file", []string{"go/main.go"}, Scope{Packages: []string{"."}}},
		{"local shard", []string{"go/internal/core/" + shardRelPath}, Scope{Packages: []string{"internal/core"}}},
		{"root shard", []string{"go/" + shardRelPath}, Scope{Packages: []string{"."}}},
		{"deleted file keeps its package", []string{"go/old/gone_test.go"}, Scope{Packages: []string{"old"}}},
		{"central baselines", []string{"go/" + centralBaselinesRelPath}, Scope{Whole: true}},
		{"guards", []string{"go/internal/testaudit/guards.json"}, Scope{Whole: true}},
		{"fakes", []string{"go/internal/testaudit/fakes.json"}, Scope{Whole: true}},
		{"audit rule source", []string{"go/internal/testaudit/rules.go"}, Scope{Packages: []string{"internal/testaudit"}, Whole: true}},
		{"audit test source", []string{"go/internal/testaudit/rules_test.go"}, Scope{Packages: []string{"internal/testaudit"}}},
		{"outside go", []string{"docs/a.md", "agents/x.go"}, Scope{}},
		{"non go file in go", []string{"go/internal/core/data.json", "go/go.mod"}, Scope{}},
		{"empty", nil, Scope{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := AuditScope(tc.changed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("AuditScope(%v) = %+v, want %+v", tc.changed, got, tc.want)
			}
		})
	}
}
