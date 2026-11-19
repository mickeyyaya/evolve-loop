package policy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTheRegressionTIAPolicyKeyIsRetired(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "regression_tia") {
		t.Error("the checked-in policy still carries a regression_tia block")
	}
	if _, err := policy.Parse([]byte(`{"regression_tia":{"stage":"shadow"}}`)); err == nil {
		t.Error("Parse accepts a regression_tia block: the key retired, so the strict decoder must refuse it")
	}
}
