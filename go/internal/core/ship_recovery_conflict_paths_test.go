package core

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRebaseWithDerivedRegen_NamesEveryNonDerivedConflictAndNoDerivedOne(t *testing.T) {
	g := &scriptedGit{}
	g.respond = func(args []string) (string, int, error) {
		j := joined(args)
		switch {
		case j == "rebase main":
			return "", 1, nil
		case strings.HasPrefix(j, "diff --name-only --diff-filter=U"):
			return "go/.apicover-enforce\x00" + cflags + "\x00docs/architecture/adr/0105-identity-preserving-fleet-rebase.md\x00", 0, nil
		case strings.Contains(j, "rebase --abort"):
			return "", 0, nil
		}
		t.Fatalf("unexpected git call %q", j)
		return "", 0, nil
	}
	regen, _ := recordingRegen("")

	ok, conflicts := rebaseWithDerivedRegen(context.Background(), "/wt", g.capture, regen, derivedEntryOf)

	want := []string{"go/.apicover-enforce", "docs/architecture/adr/0105-identity-preserving-fleet-rebase.md"}
	if ok || !reflect.DeepEqual(conflicts, want) {
		t.Fatalf("rebase = (%v, %v), want (false, %v): the debugger may write exactly the non-derived conflicts", ok, conflicts, want)
	}
}

func TestLatchShippedState_ForgetsTheConflictsOfTheRecoveryThatLanded(t *testing.T) {
	cs := CycleState{ShipRecoveryCode: string(CodeGitFleetRebaseNeeded), ShipRecoveryConflicts: []string{"go/.apicover-enforce"}}

	if !latchShippedState(&cs, PhaseShip, VerdictPASS) || cs.ShipRecoveryConflicts != nil {
		t.Fatalf("after the ship latched, conflicts = %v, want none", cs.ShipRecoveryConflicts)
	}
}
