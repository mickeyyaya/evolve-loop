//go:build acs

package cycle2658

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	inboxmoverPkg  = "./internal/inboxmover"
	cmdPkg         = "./cmd/evolve"
	promoterSource = "internal/inboxmover/promote_delivered.go"
	cardPath       = "docs/architecture/integration-driver-enforce.md"
)

func runGoTest(t *testing.T, pkg, pattern string, wantPass ...string) {
	t.Helper()
	mod := acsassert.GoModuleDir(t)
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", "test", "-count=1", "-v", "-run", pattern, pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", pattern, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped or not run):\n%s", name, stdout)
		}
	}
}

func TestC2658_001_a_merged_member_moves_delivered_to_processed_and_a_deferred_id_stays_open(t *testing.T) {
	runGoTest(t, inboxmoverPkg, "^TestPromoter_Promote_(MovesTheDeliveredItemOfAMergedMemberToProcessed|ADeferredIdThatNeverLeftTheInboxStaysOpen|AnIdListedTwiceMovesOnceAndASecondCallIsANoOp)$",
		"TestPromoter_Promote_MovesTheDeliveredItemOfAMergedMemberToProcessed",
		"TestPromoter_Promote_ADeferredIdThatNeverLeftTheInboxStaysOpen",
		"TestPromoter_Promote_AnIdListedTwiceMovesOnceAndASecondCallIsANoOp")
}

func TestC2658_002_a_failed_move_keeps_the_item_delivered_and_a_no_op_input_moves_nothing(t *testing.T) {
	runGoTest(t, inboxmoverPkg, "^TestPromoter_Promote_(NoOpsAndErrors|AFailedMoveKeepsTheItemDeliveredAndReturnsErrMvFailed|AFailedLedgerAppendWarnsAndStillPromotes)$",
		"TestPromoter_Promote_NoOpsAndErrors",
		"TestPromoter_Promote_AFailedMoveKeepsTheItemDeliveredAndReturnsErrMvFailed",
		"TestPromoter_Promote_AFailedLedgerAppendWarnsAndStillPromotes")
}

func TestC2658_003_the_production_enforce_actor_holds_a_promoter_so_missing_names_no_promote(t *testing.T) {
	runGoTest(t, cmdPkg, "^TestProductionEnforceActor_APromoterIsWiredSoMissingNamesNoPromote$",
		"TestProductionEnforceActor_APromoterIsWiredSoMissingNamesNoPromote")
}

func TestC2658_004_the_production_actor_flush_of_a_merged_member_promotes_through_the_production_port(t *testing.T) {
	runGoTest(t, cmdPkg, "^TestProductionEnforceActor_AMergedMemberMovesItsDeliveredItemToProcessedAndAnOpenDeferredIdStays$",
		"TestProductionEnforceActor_AMergedMemberMovesItsDeliveredItemToProcessedAndAnOpenDeferredIdStays")
}

func TestC2658_005_every_function_of_the_promoter_source_has_full_statement_coverage(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", "test", "-count=1", "-run", "^TestPromoter_", "-coverprofile="+profile, inboxmoverPkg)
	if code != 0 || err != nil {
		t.Fatalf("coverage run exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	funcs, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", "tool", "cover", "-func="+profile)
	if code != 0 || err != nil {
		t.Fatalf("go tool cover exited %d (err=%v)\n%s", code, err, stderr)
	}
	rows := 0
	for _, line := range strings.Split(funcs, "\n") {
		if !strings.Contains(line, "promote_delivered.go:") {
			continue
		}
		rows++
		if !strings.HasSuffix(strings.TrimSpace(line), "100.0%") {
			t.Errorf("function below 100%% statement coverage: %s", line)
		}
	}
	if rows == 0 {
		t.Errorf("no function of %s in the coverage report: the promoter source is missing or not exercised", promoterSource)
	}
}

func TestC2658_006_the_promoter_source_carries_zero_comments_and_is_not_gitignored(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	path := filepath.Join(mod, promoterSource)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s is missing: %v", promoterSource, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", promoterSource, err)
	}
	for _, group := range file.Comments {
		t.Errorf("comment in %s: %q", promoterSource, group.Text())
	}
	if _, _, code, _ := acsassert.SubprocessOutputIn(mod, "git", "check-ignore", "-q", promoterSource); code == 0 {
		t.Errorf("%s is gitignored and would be dropped at ship", promoterSource)
	}
}

// acs-predicate: config-check
func TestC2658_007_card_section_7_names_the_delivered_promoter_and_passes_ste_lint(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	root := filepath.Dir(mod)
	body, err := os.ReadFile(filepath.Join(root, cardPath))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "## 7. ")
	end := strings.Index(text, "## 8. ")
	if start < 0 || end < start {
		t.Fatalf("section 7 not found in %s", cardPath)
	}
	section := text[start:end]
	for _, want := range []string{"inboxmover.Promoter", "delivered", "processed"} {
		if !strings.Contains(section, want) {
			t.Errorf("section 7 does not name %q", want)
		}
	}
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", "run", "./cmd/evolve", "docs", "ste-lint", "--strict", "--project-root", root, filepath.Join(root, cardPath))
	if code != 0 || err != nil {
		t.Errorf("ste-lint --strict exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC2658_008_the_change_adds_no_test_helper_that_is_an_equal_copy_of_an_existing_one(t *testing.T) {
	runGoTest(t, "./internal/repocontract", "^TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline$",
		"TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline")
}
