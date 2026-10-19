package request_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
)

const cycle1950Fixture = "testdata/cycle1950/predicates.go.fixture"

func materializeCycle1950(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(cycle1950Fixture)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "cycle1950")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func predicateAsk(name string) handoff.Ask {
	return handoff.Ask{
		ID:           name,
		Text:         name,
		Source:       "acs/cycle1950/predicates_test.go:" + name,
		EvidenceKind: "green_run",
		Required:     true,
	}
}

func writePackage(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cycle9001")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTDDBuild_Cycle1950FixtureListsEachPredicate(t *testing.T) {
	got, err := request.TDDBuild(materializeCycle1950(t))
	if err != nil {
		t.Fatalf("TDDBuild: %v", err)
	}
	want := handoff.Request{
		Pair: "tdd-build",
		Asks: []handoff.Ask{
			predicateAsk("TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage"),
			predicateAsk("TestC1950_002_BatchAddsNoProductionEdits"),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestTDDBuild_ListsOnlyPredicatesAndKeepsTheSourceOrder(t *testing.T) {
	dir := writePackage(t, map[string]string{
		"b_test.go": "//go:build acs\n\npackage cycle9001\n\nimport \"testing\"\n\nfunc TestC9001_003_Third(t *testing.T) {}\n",
		"a_test.go": "//go:build acs\n\npackage cycle9001\n\nimport \"testing\"\n\nfunc helper(t *testing.T) {}\n\nfunc TestC9001_002_Second(t *testing.T) {}\n\nfunc TestC9001_001_First(t *testing.T) {}\n\nfunc TestMain(m *testing.M) {}\n\nfunc TestWrongParam(n int) {}\n\nfunc TestNoParam() {}\n\nfunc TestTwoParams(t *testing.T, n int) {}\n\nfunc TestOtherPointer(t *other.T) {}\n\nfunc TestValueParam(t testing.T) {}\n\nfunc TestPlainStar(t *T) {}\n\nfunc BenchmarkSkip(b *testing.B) {}\n\ntype holder struct{}\n\nfunc (holder) TestMethod(t *testing.T) {}\n",
		"util.go":   "package cycle9001\n\nfunc TestNotInATestFile(t *testing.T) {}\n",
	})
	got, err := request.TDDBuild(dir)
	if err != nil {
		t.Fatalf("TDDBuild: %v", err)
	}
	var ids []string
	for _, ask := range got.Asks {
		ids = append(ids, ask.ID)
	}
	want := []string{"TestC9001_002_Second", "TestC9001_001_First", "TestC9001_003_Third"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want files in name order and functions in source order %v", ids, want)
	}
	if got.Asks[0].Source != "acs/cycle9001/a_test.go:TestC9001_002_Second" {
		t.Errorf("source = %q, want it free of the machine path", got.Asks[0].Source)
	}
}

func TestTDDBuild_DigestDoesNotDependOnTheCheckoutPath(t *testing.T) {
	files := map[string]string{"p_test.go": "package cycle9001\n\nimport \"testing\"\n\nfunc TestC9001_001_Only(t *testing.T) {}\n"}
	first, err := request.TDDBuild(writePackage(t, files))
	if err != nil {
		t.Fatal(err)
	}
	second, err := request.TDDBuild(writePackage(t, files))
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != second.Digest() {
		t.Errorf("two checkouts of one package gave two digests: %s vs %s", first.Digest(), second.Digest())
	}
}

func TestTDDBuild_RefusesAPackageItCannotListWhole(t *testing.T) {
	cases := []struct {
		name    string
		dir     func(t *testing.T) string
		wantErr error
	}{
		{
			name: "a missing directory",
			dir:  func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent") },
		},
		{
			name: "a package with no predicate",
			dir: func(t *testing.T) string {
				return writePackage(t, map[string]string{"x_test.go": "package cycle9001\n\nfunc helper() {}\n"})
			},
			wantErr: request.ErrNoPredicates,
		},
		{
			name:    "a directory with no test file",
			dir:     func(t *testing.T) string { return writePackage(t, nil) },
			wantErr: request.ErrNoPredicates,
		},
		{
			name: "a test file that does not parse",
			dir: func(t *testing.T) string {
				return writePackage(t, map[string]string{"bad_test.go": "package cycle9001\n\nfunc (\n"})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := request.TDDBuild(tc.dir(t))
			if err == nil {
				t.Fatalf("want an error, got %+v", got)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if len(got.Asks) != 0 {
				t.Errorf("a refused build returned asks: %+v", got.Asks)
			}
		})
	}
}

func TestTDDBuild_RequestBindsALedgerThatValidates(t *testing.T) {
	req, err := request.TDDBuild(materializeCycle1950(t))
	if err != nil {
		t.Fatal(err)
	}
	requireMetLedgerValidates(t, req)
}
