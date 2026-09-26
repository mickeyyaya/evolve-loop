package inboxbatch

import (
	"reflect"
	"testing"
)

func TestItem_FiledAt(t *testing.T) {
	for _, tc := range []struct {
		name string
		item Item
		want string
	}{
		{"created_at wins over the filename", Item{CreatedAt: "2026-08-16T19:30:00Z", Path: "2026-01-01T00-00-00Z-x.json"}, "2026-08-16T19:30:00Z"},
		{"a bare-date created_at", Item{CreatedAt: "2026-08-16"}, "2026-08-16T00:00:00Z"},
		{"the filename prefix when created_at is absent", Item{Path: "2026-08-16T19-30-00Z-x.json"}, "2026-08-16T19:30:00Z"},
		{"an unparseable created_at falls to the filename", Item{CreatedAt: "soon", Path: "2026-08-16T19-30-00Z-x.json"}, "2026-08-16T19:30:00Z"},
		{"no date anywhere is zero", Item{Path: "x.json"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.item.FiledAt()
			if tc.want == "" {
				if !got.IsZero() {
					t.Fatalf("FiledAt = %v, want zero", got)
				}
				return
			}
			if s := got.UTC().Format("2006-01-02T15:04:05Z"); s != tc.want {
				t.Fatalf("FiledAt = %s, want %s", s, tc.want)
			}
		})
	}
}

func TestItem_DeclaredPaths(t *testing.T) {
	it := Item{Files: []string{"go/internal/ship/consume.go (gate condition)", "go/internal/x.go:178", "N/A", "role.go", "go/internal/inboxbatch/"}}
	want := []string{"go/internal/ship/consume.go", "go/internal/x.go", "go/internal/inboxbatch/"}
	if got := it.DeclaredPaths(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DeclaredPaths = %q, want %q", got, want)
	}
}

func TestStripControl(t *testing.T) {
	if got := StripControl("fix\nnew bullet\t\x1b[31m\x7fok ✓"); got != "fix new bullet  [31m ok ✓" {
		t.Fatalf("StripControl = %q", got)
	}
}
