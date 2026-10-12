package triage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForbiddenFileOf_StripsAParentheticalNoteBeforeTheProtectionCheck(t *testing.T) {
	t.Parallel()
	forbidden := func(p string) bool { return p == "go/internal/guards/role.go" }
	cases := []struct {
		name     string
		line     string
		wantPath string
		wantHit  bool
	}{
		{"annotated path", "- id-1: fix — priority=H, files=go/internal/guards/role.go (see ADR-0073); go/internal/foo/foo.go, source=scout", "go/internal/guards/role.go", true},
		{"annotation only on the safe file", "- id-2: fix — priority=H, files=go/internal/foo/foo.go (the guard one lives elsewhere), source=scout", "", false},
		{"a note with no path before the parenthesis", "- id-3: fix — priority=H, files=(guards role.go); go/internal/guards/role.go, source=scout", "go/internal/guards/role.go", true},
		{"no files field", "- id-4: fix — priority=H, source=scout", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, hit := forbiddenFileOf(tc.line, forbidden)
			if path != tc.wantPath || hit != tc.wantHit {
				t.Fatalf("forbiddenFileOf = (%q,%v), want (%q,%v)", path, hit, tc.wantPath, tc.wantHit)
			}
		})
	}
}

func TestRouteProtectedCards_MalformedArraysAreNamedAndLeaveTheDecisionUntouched(t *testing.T) {
	t.Parallel()
	cards := []protectedCard{{ID: "bad-card", Path: "go/internal/guards/role.go"}}
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"top_n is not an array", `{"top_n":{"id":"bad-card"},"escalate_block":[]}`, "top_n:"},
		{"escalate_block is not an array", `{"top_n":[{"id":"bad-card"}],"escalate_block":"nope"}`, "escalate_block:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeDecision(t, t.TempDir(), tc.body)

			err := routeProtectedCards(path, cards, nil)

			if err == nil || !strings.Contains(err.Error(), "route protected cards: "+tc.wantErr) {
				t.Fatalf("err = %v", err)
			}
			if got, _ := os.ReadFile(path); string(got) != tc.body {
				t.Errorf("a refused routing must not rewrite the decision: %s", got)
			}
		})
	}
}

func TestRouteProtectedCards_UnwritableDecisionDirectoryIsReported(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := writeDecision(t, ws, `{"top_n":[{"id":"bad-card"}],"escalate_block":[]}`)
	if err := os.Chmod(ws, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	if f, err := os.CreateTemp(ws, "probe"); err == nil {
		f.Close()
		t.Skip("directory permissions are not enforced for this user")
	}

	err := routeProtectedCards(path, []protectedCard{{ID: "bad-card", Path: "go/internal/guards/role.go"}}, nil)

	if err == nil || !strings.Contains(err.Error(), "route protected cards:") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), `"top_n":[{"id":"bad-card"}]`) {
		t.Errorf("the original decision must survive a failed write: %s", got)
	}
}

func TestRawField_NonObjectsAndNonStringValuesYieldEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		obj  string
		want string
	}{
		{`{"id":"abc"}`, "abc"},
		{`{"id":7}`, ""},
		{`{"other":"x"}`, ""},
		{`"just a string"`, ""},
		{`[1,2]`, ""},
		{`not json`, ""},
		{``, ""},
	}
	for _, tc := range cases {
		if got := rawField([]byte(tc.obj), "id"); got != tc.want {
			t.Errorf("rawField(%q,id) = %q, want %q", tc.obj, got, tc.want)
		}
	}
}

func TestRankInputs_UnreadablePolicyWarnsOnStderrAndStillRanks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	in := rankInputs(root)
	os.Stderr = orig
	w.Close()
	logged := <-done

	if !strings.Contains(logged, "[triage] WARN inbox rank: policy unreadable") {
		t.Errorf("stderr = %q", logged)
	}
	if in.Now.IsZero() {
		t.Errorf("a broken policy must not stop the rank from getting its clock: %+v", in)
	}
}
