package inboxmover

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
)

const (
	companionLane    = "retro-minting-files-through-inbox-add-lane"
	companionAcFive  = "retro-minting-files-through-inbox-add-lane-ac-5-console"
	companionReason  = companionLane + ": console_companion_pending: " + companionAcFive
	companionUnclear = "other-console"
)

func companionQueue() []inboxbatch.Item {
	return []inboxbatch.Item{{ID: companionLane, Route: "lane"}, {ID: companionAcFive, Route: "console-manual"}, {ID: companionUnclear, Route: "console-manual"}, {ID: "free", Route: "lane"}}
}

func TestRankLaneMenu_EnforceDefersALaneItemWithAPendingConsoleCompanion(t *testing.T) {
	t.Parallel()
	menu := RankLaneMenu(Options{InboxDir: t.TempDir()}, companionQueue(), nil, inboxrank.Inputs{}, config.StageEnforce)

	if got := idsOf(menu.Waiting); !reflect.DeepEqual(got, []string{companionLane}) {
		t.Errorf("waiting = %v, want the lane item", got)
	}
	if !reflect.DeepEqual(menu.WaitingReasons, []string{companionReason}) {
		t.Errorf("reasons = %v, want %q", menu.WaitingReasons, companionReason)
	}
	if got := idsOf(menu.Ready); !reflect.DeepEqual(got, []string{"free"}) || len(menu.Ranked) != 1 || len(menu.WouldDefer) != 0 {
		t.Errorf("ready = %v, ranked %d, would %v; want only the free item ranked", got, len(menu.Ranked), menu.WouldDefer)
	}
}

func TestRankLaneMenu_ShadowKeepsTheItemAndRecordsWouldDefer(t *testing.T) {
	t.Parallel()
	menu := RankLaneMenu(Options{InboxDir: t.TempDir()}, companionQueue(), nil, inboxrank.Inputs{}, config.StageShadow)

	if got := idsOf(menu.Ready); !reflect.DeepEqual(got, []string{companionLane, "free"}) || len(menu.Waiting) != 0 {
		t.Errorf("ready = %v, waiting %v; want the lane item still ready at shadow", got, idsOf(menu.Waiting))
	}
	if !reflect.DeepEqual(menu.WouldDefer, []string{companionReason}) {
		t.Errorf("would defer = %v, want %q", menu.WouldDefer, companionReason)
	}
}

func TestRankLaneMenu_OffChecksNothing(t *testing.T) {
	t.Parallel()
	menu := RankLaneMenu(Options{InboxDir: t.TempDir()}, companionQueue(), nil, inboxrank.Inputs{}, config.StageOff)
	if len(menu.Ready) != 2 || len(menu.WouldDefer) != 0 {
		t.Errorf("menu = %+v, want no check at off", menu.LaneMenu)
	}
}

func TestRankLaneMenu_AConsumedCompanionLeavesTheLaneItemReady(t *testing.T) {
	t.Parallel()
	queue := []inboxbatch.Item{{ID: companionLane, Route: "lane"}, {ID: companionUnclear, Route: "console-manual"}}
	menu := RankLaneMenu(Options{InboxDir: t.TempDir()}, queue, nil, inboxrank.Inputs{}, config.StageEnforce)
	if got := idsOf(menu.Ready); !reflect.DeepEqual(got, []string{companionLane}) || len(menu.Waiting) != 0 {
		t.Errorf("ready = %v, want the lane item once its companion left the inbox", got)
	}
}
