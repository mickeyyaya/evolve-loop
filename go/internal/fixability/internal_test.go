package fixability

import (
	"reflect"
	"testing"
)

func TestIsHandoffFinding(t *testing.T) {
	if !isHandoffFinding(Finding{ConsoleItems: []string{"ci"}}, nil) {
		t.Fatal("expected handoff for console items")
	}
	if !isHandoffFinding(Finding{}, []string{"go/internal/guards/role.go"}) {
		t.Fatal("expected handoff for protected surface")
	}
	if isHandoffFinding(Finding{}, []string{"go/internal/unprotected/foo.go"}) {
		t.Fatal("did not expect handoff for unprotected surface")
	}
	if isHandoffFinding(Finding{}, nil) {
		t.Fatal("did not expect handoff for empty paths")
	}
}

func TestIsBlockerFinding(t *testing.T) {
	if !isBlockerFinding(Finding{Severity: "CRITICAL"}, nil, nil) {
		t.Fatal("expected blocker for critical")
	}
	if !isBlockerFinding(Finding{Severity: "HIGH"}, nil, nil) {
		t.Fatal("expected blocker for high")
	}
	if !isBlockerFinding(Finding{Severity: "MEDIUM"}, []string{"pkg/sub/file.go"}, []string{"pkg/sub"}) {
		t.Fatal("expected blocker for matching red path")
	}
	if isBlockerFinding(Finding{Severity: "MEDIUM"}, []string{"pkg/sub"}, []string{"other/sub"}) {
		t.Fatal("did not expect blocker for mismatched path")
	}
	if isBlockerFinding(Finding{Severity: "MEDIUM"}, nil, nil) {
		t.Fatal("did not expect blocker for empty paths")
	}
}

func TestMatchPathSegment(t *testing.T) {
	if !matchPathSegment("/a/b/c", "a/b/c/") {
		t.Fatal("expected exact match")
	}
	if !matchPathSegment("a/b/c", "a/b") {
		t.Fatal("expected prefix match a in b")
	}
	if !matchPathSegment("a/b", "a/b/c") {
		t.Fatal("expected prefix match b in a")
	}
	if matchPathSegment("a/bc", "a/b") {
		t.Fatal("did not expect match across non-segment boundary")
	}
}

func TestAppendUnique(t *testing.T) {
	got := appendUnique([]string{"a"}, "", "b", "a", "")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
