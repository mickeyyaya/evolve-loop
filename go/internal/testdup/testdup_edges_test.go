package testdup

import (
	"os"
	"path/filepath"
	"testing"
)

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("root reads a file with mode 000")
	}
}

func TestScan_FailsOnADirectoryItCannotRead(t *testing.T) {
	skipIfRoot(t)
	root := t.TempDir()
	plant(t, root, map[string]string{"internal/a/a_test.go": source(helperSource, "a", "makeFile")})
	locked := filepath.Join(root, "internal", "a")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })

	if _, err := Scan(root, baselineDay); err == nil {
		t.Fatal("Scan of an unreadable directory returned no error; a skipped directory would hide copies")
	}
}

func TestScan_FailsOnATestFileItCannotRead(t *testing.T) {
	skipIfRoot(t)
	root := t.TempDir()
	plant(t, root, map[string]string{"internal/a/a_test.go": source(helperSource, "a", "makeFile")})
	locked := filepath.Join(root, "internal", "a", "a_test.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })

	if _, err := Scan(root, baselineDay); err == nil {
		t.Fatal("Scan of an unreadable test file returned no error")
	}
}

func TestScan_IgnoresAnUnreadableDirectoryInsideAnUntrackedAcsDirectory(t *testing.T) {
	skipIfRoot(t)
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go":              source(helperSource, "a", "makeFile"),
		"acs/cycle9/predicates_test.go":     source(helperSource, "cycle9", "makeFile"),
		"acs/cycle9/inner/inner_test.go.in": "x",
	})
	ageTree(t, filepath.Join(root, "acs"), justNow)
	locked := filepath.Join(root, "acs", "cycle9", "inner")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })

	report, err := dirAgeOnly(root)

	if err != nil {
		t.Fatal(err)
	}
	if report {
		t.Error("acs/cycle9 was touched after the baseline date and must not be old")
	}
}

func dirAgeOnly(root string) (bool, error) {
	return loadAcsAges(root, baselineDay).isOld("acs/cycle9"), nil
}

func TestAcsDir_ReturnsNothingForAFileDirectlyUnderAcs(t *testing.T) {
	if got := acsDir("acs/top_test.go"); got != "" {
		t.Errorf("acsDir = %q, want empty for a file that is in no cycle directory", got)
	}
	if got := acsDir("acs/cycle7/p_test.go"); got != "acs/cycle7" {
		t.Errorf("acsDir = %q, want acs/cycle7", got)
	}
}

func TestScan_OrdersSeveralGroupsByTheirFirstSite(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/z/z_test.go": source(helperSource, "z", "makeFile"),
		"internal/y/y_test.go": source(helperSource, "y", "newPath"),
		"internal/b/b_test.go": source(otherHelperSource, "b", "readAll"),
		"internal/a/a_test.go": source(otherHelperSource, "a", "slurp"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if len(report.Groups) != 2 || report.Groups[0].Sites[0] > report.Groups[1].Sites[0] {
		t.Errorf("Groups = %v, want two groups ordered by first site", report.Groups)
	}
}
