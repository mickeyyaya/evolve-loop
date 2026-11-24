package modelquery

import (
	"context"
	"slices"
	"testing"
)

type fakeRouted struct {
	families []string
	listed   []string
}

func (f *fakeRouted) Families() []string { return f.families }

func (f *fakeRouted) List(_ context.Context, cli string) ([]string, error) {
	f.listed = append(f.listed, cli)
	return []string{"m-" + cli}, nil
}

func TestDefaultRouter_RoutesEachFamilyTheListerNamesOffThePicker(t *testing.T) {
	routed := &fakeRouted{families: []string{"agy", "newcli"}}
	r := DefaultRouter(nil, routed)
	for _, cli := range routed.families {
		got, err := r.List(context.Background(), cli)
		if err != nil || !slices.Equal(got, []string{"m-" + cli}) {
			t.Errorf("%s: got %q, %v; want the routed lister", cli, got, err)
		}
	}
	if _, ok := r.ByCLI["codex"]; ok {
		t.Error("a family the lister does not name must not be registered")
	}
	if r.Default == nil {
		t.Error("Default must stay set: the picker still serves the other families")
	}
}

func TestDefaultRouter_ANilRoutedListerRoutesNothing(t *testing.T) {
	r := DefaultRouter(nil, nil)
	if len(r.ByCLI) != 0 || r.Default == nil {
		t.Fatalf("ByCLI = %v, Default = %v; want only the picker", r.ByCLI, r.Default)
	}
}

func TestRoutedLister_ServesOnlyTheFamiliesItNames(t *testing.T) {
	var routed RoutedLister = &fakeRouted{families: []string{"agy"}}
	fake := routed.(*fakeRouted)
	r := DefaultRouter(nil, routed)

	if got, err := r.List(context.Background(), "agy"); err != nil || !slices.Equal(got, []string{"m-agy"}) {
		t.Fatalf("agy: got %q, %v; want the routed lister's answer", got, err)
	}
	if _, err := r.List(context.Background(), "claude"); err == nil {
		t.Error("claude: the picker default has no capturer and must fail, not reach the routed lister")
	}
	if !slices.Equal(fake.listed, []string{"agy"}) {
		t.Errorf("routed lister was asked for %q, want only [agy]", fake.listed)
	}
}
