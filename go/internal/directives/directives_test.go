package directives_test

// Black-box, environment-agnostic coverage of the runtime operator-directives
// loader. Driven ONLY through the public API (Load/Resolve) and explicit file
// paths — never t.Setenv/os.Getenv. Per the Flag→Parameter Conversion Standard.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/directives"
)

// writeTempFile writes body to a temp file and returns its path.
func writeTempFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

const lane = "campaign"

func TestLoad_Resolution(t *testing.T) {
	cases := []struct {
		name          string
		global        string // file body; "" written as a real empty file
		perLoop       string
		globalAbsent  bool // true ⇒ pass "" path (no file)
		perLoopAbsent bool
		wantMerged    bool // expect a non-empty Merged block + Version
		wantContains  []string
	}{
		{name: "both-absent", globalAbsent: true, perLoopAbsent: true, wantMerged: false},
		{name: "empty-files-are-absent", global: "   \n\t ", perLoop: "", wantMerged: false},
		{name: "global-only", global: "Always env-agnostic.", perLoopAbsent: true, wantMerged: true,
			wantContains: []string{"## Operator Directives", "### Global (all loops)", "Always env-agnostic."}},
		{name: "per-loop-only", globalAbsent: true, perLoop: "Focus cluster-10.", wantMerged: true,
			wantContains: []string{"### This loop (" + lane + ")", "Focus cluster-10."}},
		{name: "both-present", global: "Global rule.", perLoop: "Loop rule.", wantMerged: true,
			wantContains: []string{"### Global (all loops)", "Global rule.", "### This loop (" + lane + ")", "Loop rule."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gPath := ""
			if !tc.globalAbsent {
				gPath = writeTempFile(t, "directives.md", tc.global)
			}
			lPath := ""
			if !tc.perLoopAbsent {
				lPath = writeTempFile(t, "loop.md", tc.perLoop)
			}
			var set directives.Set = directives.Load(gPath, lPath, lane)
			if tc.wantMerged {
				if set.Merged == "" || set.Version == "" {
					t.Fatalf("expected non-empty Merged+Version, got Merged=%q Version=%q", set.Merged, set.Version)
				}
				for _, sub := range tc.wantContains {
					if !strings.Contains(set.Merged, sub) {
						t.Errorf("Merged missing %q; got:\n%s", sub, set.Merged)
					}
				}
			} else if set.Merged != "" || set.Version != "" {
				t.Errorf("expected empty Set, got Merged=%q Version=%q", set.Merged, set.Version)
			}
		})
	}
}

func TestLoad_SafetyPreambleAlwaysPresentWhenRendered(t *testing.T) {
	g := writeTempFile(t, "directives.md", "Be env-agnostic.")
	set := directives.Load(g, "", lane)
	// The gates-are-authoritative preamble must accompany any injected directives.
	if !strings.Contains(set.Merged, "Binary gates") || !strings.Contains(set.Merged, "guidance does not override a gate") {
		t.Errorf("rendered block missing the gates-authoritative safety preamble; got:\n%s", set.Merged)
	}
}

func TestLoad_FailOpenOnUnreadable(t *testing.T) {
	// A path that does not exist must fail open to an absent layer, never panic.
	missing := filepath.Join(t.TempDir(), "nope.md")
	var got directives.Set = directives.Load(missing, missing, lane)
	var lay directives.Layer = got.Global
	if lay.Body != "" {
		t.Errorf("unreadable path must yield absent layer, got Body=%q", lay.Body)
	}
	if got.Merged != "" || got.Version != "" {
		t.Errorf("both unreadable ⇒ empty Set, got Merged=%q Version=%q", got.Merged, got.Version)
	}
}

func TestLoad_VersionDeterminismAndSensitivity(t *testing.T) {
	g1 := writeTempFile(t, "a.md", "Rule A.")
	g2 := writeTempFile(t, "b.md", "Rule A.") // identical content, different file
	g3 := writeTempFile(t, "c.md", "Rule B.") // different content

	v1 := directives.Load(g1, "", lane).Version
	v2 := directives.Load(g2, "", lane).Version
	v3 := directives.Load(g3, "", lane).Version

	if v1 == "" {
		t.Fatal("version must be set when directives render")
	}
	if v1 != v2 {
		t.Errorf("identical content must yield identical Version: %q vs %q", v1, v2)
	}
	if v1 == v3 {
		t.Errorf("different content must yield different Version, both = %q", v1)
	}
	// Global-only directives are lane-independent (lane is rendered only in the
	// per-loop section), so the version must NOT change with the lane here.
	if diff := directives.Load(g1, "", "other-lane").Version; diff != v1 {
		t.Errorf("global-only version must be lane-independent: %q vs %q", v1, diff)
	}
	// With a per-loop layer present, the lane IS rendered and must affect version.
	lPath := writeTempFile(t, "loop.md", "Loop rule.")
	if vA, vB := directives.Load("", lPath, "lane-a").Version, directives.Load("", lPath, "lane-b").Version; vA == vB {
		t.Errorf("different lane with a per-loop layer must change Version, both = %q", vA)
	}
}

func TestLoad_LaneSanitizedInHeading(t *testing.T) {
	// A lane carrying line breaks must not inject extra lines / fake headings into
	// the rendered block — the per-loop heading stays on one line.
	lPath := writeTempFile(t, "loop.md", "Loop rule.")
	set := directives.Load("", lPath, "evil\n## Injected\nlane")
	// The danger is a NEWLINE-prefixed fake heading; sanitization removes the line
	// breaks (the literal "## Injected" survives mid-line, which markdown ignores).
	if strings.Contains(set.Merged, "\n## Injected") {
		t.Errorf("lane line breaks leaked a line-start heading into the block; got:\n%s", set.Merged)
	}
	if !strings.Contains(set.Merged, "### This loop (evil## Injectedlane)") {
		t.Errorf("expected sanitized single-line heading; got:\n%s", set.Merged)
	}
}

func TestResolve_Paths(t *testing.T) {
	home := filepath.Join("/tmp", "home")
	gPath, lPath := directives.Resolve(home, lane)
	wantG := filepath.Join(home, ".claude", "evolve", "directives.md")
	wantL := filepath.Join(home, ".claude", "evolve", "loops", lane+".md")
	if gPath != wantG {
		t.Errorf("global path = %q, want %q", gPath, wantG)
	}
	if lPath != wantL {
		t.Errorf("per-loop path = %q, want %q", lPath, wantL)
	}
}
