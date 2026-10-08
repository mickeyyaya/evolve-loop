package inboxmover

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
)

type Dispatchability struct {
	Dispatchable bool
	Reason       string
}

func ResolveDispatchability(opts Options, taskID string) Dispatchability {
	ds := ResolveDispatchState(opts, taskID)
	switch ds.State {
	case StatePending:
		return PendingDispatchability(opts, ds.Deps)
	case StateUnknown:
		return Dispatchability{Dispatchable: true}
	default:
		return Dispatchability{Reason: consumedReason(ds)}
	}
}

func PendingDispatchability(opts Options, deps []string) Dispatchability {
	for _, dep := range deps {
		if blocksDependents(ResolveDispatchState(opts, dep).State) {
			return Dispatchability{Reason: "deps unmet: needs " + dep}
		}
	}
	return Dispatchability{Dispatchable: true}
}

func blocksDependents(state string) bool {
	switch state {
	case StatePending, StateProcessing, StateRetry:
		return true
	}
	return false
}

func consumedReason(ds DispatchState) string {
	if ds.Detail == "" {
		return "consumed: " + ds.State
	}
	return "consumed: " + ds.State + " " + ds.Detail
}

type MenuPlace int

const (
	MenuReady MenuPlace = iota
	MenuConsole
	MenuWaiting
)

func PlaceOnLaneMenu(opts Options, it inboxbatch.Item, isProtected func(string) bool) (MenuPlace, string) {
	if routed, reason := inboxbatch.ConsoleRouted(it, isProtected); routed {
		return MenuConsole, reason
	}
	opts.resolveOpts()
	if claimDir, held := claimHolding(opts.InboxDir, it.ID); held {
		return MenuWaiting, "held by the claim of " + claimDir
	}
	if d := PendingDispatchability(opts, it.Deps); !d.Dispatchable {
		return MenuWaiting, d.Reason
	}
	return MenuReady, ""
}

type LaneMenu struct {
	Ready          []inboxbatch.Item
	Console        []inboxbatch.Item
	ConsoleReasons []string
	Waiting        []inboxbatch.Item
	WaitingReasons []string
}

type RankedLaneMenu struct {
	LaneMenu
	Ranked []inboxrank.Ranked
}

func RankLaneMenu(opts Options, queue []inboxbatch.Item, isProtected func(string) bool, rank inboxrank.Inputs) RankedLaneMenu {
	menu := PartitionLaneMenu(opts, queue, isProtected)
	return RankedLaneMenu{LaneMenu: menu, Ranked: rank.Order(menu.Ready, queue)}
}

func PartitionLaneMenu(opts Options, items []inboxbatch.Item, isProtected func(string) bool) LaneMenu {
	var menu LaneMenu
	for _, it := range items {
		switch place, reason := PlaceOnLaneMenu(opts, it, isProtected); place {
		case MenuConsole:
			menu.Console = append(menu.Console, it)
			menu.ConsoleReasons = append(menu.ConsoleReasons, it.ID+": "+reason)
		case MenuWaiting:
			menu.Waiting = append(menu.Waiting, it)
			menu.WaitingReasons = append(menu.WaitingReasons, it.ID+": "+reason)
		default:
			menu.Ready = append(menu.Ready, it)
		}
	}
	return menu
}
