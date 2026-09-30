//go:build acs

package cycle1070

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func writeRawTestReport(t *testing.T, ws, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write raw test-report.md: %v", err)
	}
}

func TestC1070_006_EmptyTopNNoAuthoredFilesApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)
	writeTestReport(t, ws, "some-slug")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("empty top_n + zero authored files is the compliant no-op and must be APPROVED; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_007_MissingTestReportFailsOpen(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-a", "committed-b")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("missing test-report.md (nothing to bind a claimed slug from) must fail OPEN; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_008_UnparseableTaskHeaderFailsOpen(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-a", "committed-b")
	raw := "# TDD Report — Cycle 1070\n\n" +
		"## Handoff to Builder\n```json\n{\n  \"testFiles\": [\"go/acs/cycle1070/x_test.go\"]\n}\n```\n"
	writeRawTestReport(t, ws, raw)

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("authored files but no parseable ## Task: header must fail OPEN (nothing to bind against); got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_009_NonJSONFenceIgnoredNotFalsePositive(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "some-other-committed-slug")
	raw := "# TDD Report — Cycle 1070\n\n## Task: unrelated-slug\n\n" +
		"## RED Run Output\n```\n" +
		"$ go test -tags acs -count=1 -v ./acs/cycle1070\n" +
		"    predicates_test.go:114: rejected go/acs/cycle660/predicates_test.go\n" +
		"FAIL\n```\n\n" +
		"## Handoff to Builder\n```json\n{\n  \"testFiles\": []\n}\n```\n"
	writeRawTestReport(t, ws, raw)

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("the authoritative handoff JSON declares zero authored files; a non-JSON prose fence mentioning a file path must NOT cause a false block; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_010_MalformedJSONFenceDoesNotPanic(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)
	raw := "# TDD Report — Cycle 1070\n\n## Task: some-slug\n\n" +
		"## Handoff to Builder\n```json\n{\n  \"testFiles\": [\"unterminated.go\"\n```\n"
	writeRawTestReport(t, ws, raw)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("malformed handoff JSON must not panic the reviewer; recovered: %v", r)
		}
	}()
	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("unparseable handoff JSON is an ambiguity and this package fails open on every ambiguity; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_011_BlankStringTestFileEntryStillCountsAsAuthored(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)
	writeTestReport(t, ws, "declined-slug", "")

	res := reviewTDD(t, config.StageEnforce, ws)
	if res.Approve {
		t.Errorf("a non-empty testFiles[] array (even a single blank-string element) under an empty top_n is authored-files-present and must be REJECTED; got Approve=true")
	}
	if !res.Approve && res.Reason == "" {
		t.Errorf("a blocked review must carry a non-empty Reason (core.ReviewResult contract); got empty Reason")
	}
}

func TestC1070_012_OutOfLaneSlugWithNoAuthoredFilesApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-slug-a", "committed-slug-b")
	writeTestReport(t, ws, "totally-other-slug")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("an out-of-lane claimed slug with zero authored testFiles[] has nothing to reject; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_013_LargeScaleTopNAndTestFilesApproves(t *testing.T) {
	ws := t.TempDir()
	slugs := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		slugs = append(slugs, fmt.Sprintf("slug-%04d", i))
	}
	writeTriage(t, ws, slugs...)

	files := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		files = append(files, fmt.Sprintf("go/acs/cycle1070/gen_%04d_test.go", i))
	}
	writeTestReport(t, ws, "slug-0499", files...)

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("in-lane claim against a large (500-entry) committed top_n with 200 authored files must still be APPROVED; got Approve=false reason=%q", res.Reason)
	}
}

func TestC1070_014_DuplicateSlugInTopNStillApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "tdd-topn-scope-gate", "tdd-topn-scope-gate", "other-slug")
	writeTestReport(t, ws, "tdd-topn-scope-gate", "go/acs/cycle1070/predicates_test.go")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("a duplicate committed-slug bullet must not break membership parsing; in-lane claim must be APPROVED; got Approve=false reason=%q", res.Reason)
	}
}
