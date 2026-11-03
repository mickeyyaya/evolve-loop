package gotestname

import "testing"

func TestHasPrefix_FollowsGoTestDiscovery(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"Test": true, "TestA": true, "Test_x": true, "Test1": true, "Testify": false, "TestÄ": true, "Tes": false, "helper": false, "BenchmarkX": false} {
		if got := HasPrefix(name, "Test"); got != want {
			t.Errorf("HasPrefix(%q, Test) = %v, want %v", name, got, want)
		}
	}
}

func TestIs_AcceptsTestFuzzAndBenchmarkByTheSameRule(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"TestA": true, "FuzzParse": true, "BenchmarkB": true, "Benchmark": true, "Fuzzy": false, "Testify": false, "Benchmarks": false, "ExampleA": false, "helper": false} {
		if got := Is(name); got != want {
			t.Errorf("Is(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIgnoredDir_NamesTheDirectoriesTheGoToolIgnores(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"testdata": true, "vendor": true, ".hidden": true, "_scratch": true, "internal": false, "a": false, "data": false} {
		if got := IgnoredDir(name); got != want {
			t.Errorf("IgnoredDir(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestUnderIgnoredDir_ChecksEachDirectoryOfAFilePath(t *testing.T) {
	t.Parallel()
	for file, want := range map[string]bool{"a.go": false, "./a.go": false, "internal/a.go": false, "/abs/a.go": false, "testdata/a.go": true, "x/vendor/y/a.go": true, ".git/a.go": true, "a/_scratch/a.go": true, "../a.go": true, "testdata.go": false} {
		if got := UnderIgnoredDir(file); got != want {
			t.Errorf("UnderIgnoredDir(%q) = %v, want %v", file, got, want)
		}
	}
}
