package inboxstamps

import (
	"context"
	"testing"
)

func TestPlanAgainst_NamesEachFieldOriginsValueReplacedAsDropped(t *testing.T) {
	w := newWorld(t)
	w.consoleLands(t)
	w.plane.Git("fetch", "-q", "origin")
	write(t, w.plane, contestedItem, `{"id":"contested","weight":0.5,"failure_count":9}`)
	partition, err := Classify(context.Background(), git(w.plane))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

	want := Dropped{Path: contestedItem, Key: "failure_count", Value: []byte("9")}
	if err != nil || len(plan.Dropped) != 1 || plan.Dropped[0].Path != want.Path || plan.Dropped[0].Key != want.Key || string(plan.Dropped[0].Value) != "9" {
		t.Errorf("dropped = %+v (err %v), want only %+v: origin changed failure_count on that item", plan.Dropped, err, want)
	}
}
