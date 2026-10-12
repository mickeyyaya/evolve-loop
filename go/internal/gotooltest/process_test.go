package gotooltest

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLimitProcess_NestedGoRunsInheritTheLimitsButTheTestBinaryKeepsItsOwn(t *testing.T) {
	t.Setenv("GOFLAGS", "-count=1 -p=32")
	t.Setenv("GOMAXPROCS", "16")

	if err := LimitProcess(); err != nil {
		t.Fatalf("LimitProcess: %v", err)
	}

	out, err := exec.Command("sh", "-c", `printf '%s|%s' "$GOFLAGS" "$GOMAXPROCS"`).Output()
	if err != nil || string(out) != "-count=1 -p=2|2" {
		t.Errorf("child env GOFLAGS|GOMAXPROCS = %q (%v), want \"-count=1 -p=2|2\"", out, err)
	}
	if got := os.Getenv("GOFLAGS"); !strings.HasSuffix(got, "-p=2") {
		t.Errorf("GOFLAGS = %q, want a trailing -p=2", got)
	}
}

func TestApplyLimits_SetsOnlyTheTwoLimitsAndReportsASetFailure(t *testing.T) {
	var set []string
	record := func(key, value string) error {
		set = append(set, key+"="+value)
		return nil
	}

	if err := applyLimits(record, []string{"PATH=/bin", "GOFLAGS=-race"}); err != nil {
		t.Fatalf("applyLimits: %v", err)
	}

	if strings.Join(set, ",") != "GOFLAGS=-race -p=2,GOMAXPROCS=2" {
		t.Errorf("set = %v, want only GOFLAGS and GOMAXPROCS", set)
	}
	boom := errors.New("read-only environment")
	err := applyLimits(func(string, string) error { return boom }, nil)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "GOFLAGS") {
		t.Errorf("applyLimits with a failing setenv = %v, want the error naming GOFLAGS", err)
	}
}
