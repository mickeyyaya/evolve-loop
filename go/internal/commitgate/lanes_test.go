package commitgate

// lanes_test.go — RED contract for cycle-549's cli-command-layer-test-coverage
// task (triage-report.md top_n item, fleet_scope
// cli-command-layer-test-coverage-worktree-swarm's "commitgate (incl. non-Go
// lane fixtures/removal)" clause). lanePython, isPyTest, laneNode, and
// laneRust — the non-Go commit-gate lanes — had ZERO direct test coverage
// (0.0% per `go tool cover -func`) even though laneGo (their sibling) is
// already well covered in commitgate_test.go, whose baseOpts/scriptRunner
// fixture harness this file reuses verbatim.

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsPyTest(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"test_foo.py", true},
		{"foo_test.py", true},
		{"dir/test_bar.py", true},
		{"nested/dir/baz_test.py", true},
		{"foo.py", false},
		{"test_foo.txt", false},
		{"testfoo.py", false}, // no separator after "test"
		{"foo.py.bak", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isPyTest(tc.path); got != tc.want {
				t.Errorf("isPyTest(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestLanePython_NoFiles_ExitPassNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := baseOpts(root, "ruff", "pytest")
	sr := &scriptRunner{}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.lanePython(context.Background(), []string{"README.md"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass", code)
	}
	if len(sr.calls) != 0 {
		t.Errorf("no .py files present, want zero subprocess calls, got %v", sr.calls)
	}
}

func TestLanePython_RuffOnly_NoTestFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.py"), "x = 1\n")
	o := baseOpts(root, "ruff", "pytest")
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "ruff check", exit: 0}}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.lanePython(context.Background(), []string{"a.py"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass (%v)", code, res.Logs)
	}
	if !reflect.DeepEqual(res.ChecksPassed, []string{"python:ruff"}) {
		t.Errorf("ChecksPassed = %v, want [python:ruff] (no test files → no pytest call)", res.ChecksPassed)
	}
}

func TestLanePython_RuffAndPytest_BothPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.py"), "x = 1\n")
	mustWrite(t, filepath.Join(root, "test_a.py"), "def test_x(): pass\n")
	o := baseOpts(root, "ruff", "pytest")
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "ruff check", exit: 0},
		{matchPrefix: "pytest -q", exit: 0},
	}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.lanePython(context.Background(), []string{"a.py", "test_a.py"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass (%v)", code, res.Logs)
	}
	if !reflect.DeepEqual(res.ChecksPassed, []string{"python:ruff", "python:pytest"}) {
		t.Errorf("ChecksPassed = %v, want [python:ruff python:pytest]", res.ChecksPassed)
	}
}

func TestLanePython_RuffFails_ExitFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.py"), "x=1\n")
	o := baseOpts(root, "ruff")
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "ruff check", exit: 1, stdout: "E1 bad\n"}}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.lanePython(context.Background(), []string{"a.py"}, res); code != ExitFail {
		t.Fatalf("code = %d, want ExitFail", code)
	}
}

func TestLanePython_RuffMissing_ExitToolMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.py"), "x=1\n")
	o := baseOpts(root) // no tools present
	o.NoInstall = true
	res := &Result{}

	if code := o.lanePython(context.Background(), []string{"a.py"}, res); code != ExitToolMissing {
		t.Fatalf("code = %d, want ExitToolMissing", code)
	}
}

func TestLanePython_PytestFails_ExitFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "test_a.py"), "def test_x(): assert False\n")
	o := baseOpts(root, "ruff", "pytest")
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "ruff check", exit: 0},
		{matchPrefix: "pytest -q", exit: 1, stdout: "1 failed\n"},
	}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.lanePython(context.Background(), []string{"test_a.py"}, res); code != ExitFail {
		t.Fatalf("code = %d, want ExitFail", code)
	}
	if !reflect.DeepEqual(res.ChecksPassed, []string{"python:ruff"}) {
		t.Errorf("ChecksPassed = %v, want [python:ruff] only (pytest failed, not recorded as passed)", res.ChecksPassed)
	}
}

func TestLaneNode_NoMatchingFiles_ExitPassNoOp(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "eslint")
	sr := &scriptRunner{}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneNode(context.Background(), []string{"README.md", "a.go"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass", code)
	}
	if len(sr.calls) != 0 {
		t.Errorf("no JS/TS files, want zero calls, got %v", sr.calls)
	}
}

func TestLaneNode_EslintPresent_Pass(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "eslint")
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "eslint", exit: 0}}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneNode(context.Background(), []string{"a.ts", "b.jsx"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass (%v)", code, res.Logs)
	}
	if !reflect.DeepEqual(res.ChecksPassed, []string{"node:eslint"}) {
		t.Errorf("ChecksPassed = %v, want [node:eslint]", res.ChecksPassed)
	}
	if len(sr.calls) != 1 || sr.calls[0] != "eslint a.ts b.jsx" {
		t.Errorf("calls = %v, want a single direct eslint invocation", sr.calls)
	}
}

func TestLaneNode_EslintAbsentNpxPresent_UsesNpx(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "npx")
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "npx eslint", exit: 0}}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneNode(context.Background(), []string{"a.mjs"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass (%v)", code, res.Logs)
	}
	if len(sr.calls) != 1 || sr.calls[0] != "npx eslint a.mjs" {
		t.Errorf("calls = %v, want npx eslint fallback invocation", sr.calls)
	}
}

func TestLaneNode_NeitherToolPresent_ExitToolMissing(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir()) // neither eslint nor npx
	res := &Result{}

	if code := o.laneNode(context.Background(), []string{"a.js"}, res); code != ExitToolMissing {
		t.Fatalf("code = %d, want ExitToolMissing", code)
	}
}

func TestLaneNode_EslintFails_ExitFail(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "eslint")
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "eslint", exit: 1, stdout: "1 problem\n"}}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneNode(context.Background(), []string{"a.jsx"}, res); code != ExitFail {
		t.Fatalf("code = %d, want ExitFail", code)
	}
}

func TestLaneRust_NoFiles_ExitPassNoOp(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "cargo")
	sr := &scriptRunner{}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneRust(context.Background(), []string{"a.go"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass", code)
	}
	if len(sr.calls) != 0 {
		t.Errorf("no .rs files, want zero calls, got %v", sr.calls)
	}
}

func TestLaneRust_SingleCrate_AllPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "Cargo.toml"), "[package]\nname=\"x\"\n")
	mustWrite(t, filepath.Join(root, "src", "main.rs"), "fn main() {}\n")
	o := baseOpts(root, "cargo")
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "cargo fmt --check", exit: 0},
		{matchPrefix: "cargo clippy", exit: 0},
		{matchPrefix: "cargo test", exit: 0},
	}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneRust(context.Background(), []string{"src/main.rs"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass (%v)", code, res.Logs)
	}
	if !reflect.DeepEqual(res.ChecksPassed, []string{"rust:fmt", "rust:clippy", "rust:test"}) {
		t.Errorf("ChecksPassed = %v, want [rust:fmt rust:clippy rust:test]", res.ChecksPassed)
	}
}

func TestLaneRust_CargoMissing_ExitToolMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "Cargo.toml"), "[package]\n")
	mustWrite(t, filepath.Join(root, "src", "main.rs"), "fn main() {}\n")
	o := baseOpts(root) // cargo absent
	o.NoInstall = true
	res := &Result{}

	if code := o.laneRust(context.Background(), []string{"src/main.rs"}, res); code != ExitToolMissing {
		t.Fatalf("code = %d, want ExitToolMissing", code)
	}
}

func TestLaneRust_ClippyFails_ExitFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "Cargo.toml"), "[package]\n")
	mustWrite(t, filepath.Join(root, "src", "main.rs"), "fn main() {}\n")
	o := baseOpts(root, "cargo")
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "cargo fmt --check", exit: 0},
		{matchPrefix: "cargo clippy", exit: 1, stdout: "warning\n"},
	}}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneRust(context.Background(), []string{"src/main.rs"}, res); code != ExitFail {
		t.Fatalf("code = %d, want ExitFail", code)
	}
	if len(res.ChecksPassed) != 0 {
		t.Errorf("ChecksPassed = %v, want empty (clippy failed before any rust:* check recorded)", res.ChecksPassed)
	}
}

// TestLaneRust_NoCargoTomlAbove_NoCrateFound: a changed .rs file with no
// Cargo.toml anywhere above it contributes no crate — the lane must not crash
// or invoke cargo, and must still pass (mirrors laneGo's "no go.mod" being a
// hard failure being the ONE asymmetry worth pinning: laneRust silently
// skips an un-rooted file rather than failing the whole gate).
func TestLaneRust_NoCargoTomlAbove_NoCrateFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "orphan.rs"), "fn f() {}\n")
	o := baseOpts(root, "cargo")
	sr := &scriptRunner{}
	o.Runner = sr.run()
	res := &Result{}

	if code := o.laneRust(context.Background(), []string{"orphan.rs"}, res); code != ExitPass {
		t.Fatalf("code = %d, want ExitPass (%v)", code, res.Logs)
	}
	if len(sr.calls) != 0 {
		t.Errorf("no Cargo.toml above orphan.rs, want zero cargo calls, got %v", sr.calls)
	}
	if len(res.ChecksPassed) != 0 {
		t.Errorf("ChecksPassed = %v, want empty (no crate found)", res.ChecksPassed)
	}
}
