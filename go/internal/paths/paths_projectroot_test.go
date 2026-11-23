package paths

import "testing"

func TestProjectRootOf_InvertsEvolveDirOf(t *testing.T) {
	for _, root := range []string{"/plane", "/a/b/c", "/"} {
		got, ok := ProjectRootOf(EvolveDirOf(root))
		if !ok || got != root {
			t.Errorf("ProjectRootOf(EvolveDirOf(%q)) = (%q, %v), want (%q, true)", root, got, ok, root)
		}
	}
}

func TestProjectRootOf_RefusesAPathThatIsNotAnEvolveDir(t *testing.T) {
	for _, dir := range []string{"", "/plane", "/plane/.evolve/inbox", "/plane/evolve", "/plane/.evolve-x"} {
		if got, ok := ProjectRootOf(dir); ok || got != "" {
			t.Errorf("ProjectRootOf(%q) = (%q, %v), want (\"\", false)", dir, got, ok)
		}
	}
}

func TestProjectRootOf_CleansATrailingSeparator(t *testing.T) {
	if got, ok := ProjectRootOf("/plane/.evolve/"); !ok || got != "/plane" {
		t.Errorf("ProjectRootOf with a trailing slash = (%q, %v), want (/plane, true)", got, ok)
	}
}
