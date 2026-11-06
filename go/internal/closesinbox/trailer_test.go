package closesinbox

import (
	"reflect"
	"testing"
)

func TestIDs_NoneIsAnExplicitEmptySet(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"alone":        "Closes-Inbox: none\n",
		"another case": "Closes-Inbox: NONE\n",
		"backticked":   "Closes-Inbox: `none`\n",
	} {
		if got := IDs([]byte(body)); got != nil {
			t.Errorf("%s: got %v, want no id", name, got)
		}
	}
	if got, want := IDs([]byte("Closes-Inbox: none, real-item\n")), []string{"real-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v: none is never an id, also in a list", got, want)
	}
}

func TestRefIDs_ReadsItsOwnMarkerOnly(t *testing.T) {
	t.Parallel()
	body := []byte("Refs-Inbox: partial-item, `other-item`\nCloses-Inbox: closed-item\n```\nRefs-Inbox: fenced-item\n```\n  Refs-Inbox: indented-item\n")
	if got, want := RefIDs(body), []string{"partial-item", "other-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("RefIDs = %v, want %v", got, want)
	}
	if got, want := IDs(body), []string{"closed-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs = %v, want %v: a Refs-Inbox line never closes", got, want)
	}
}

func TestDeclares_NoneCountsAndAbsentDoesNot(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		body string
		want bool
	}{
		"closes none":        {"Closes-Inbox: none\n", true},
		"refs line":          {"- Refs-Inbox: partial-item\n", true},
		"no trailer":         {"# Report\n\nDid the work.\n", false},
		"only a fenced line": {"```\nCloses-Inbox: fenced\n```\n", false},
	} {
		if got := Declares([]byte(row.body)); got != row.want {
			t.Errorf("%s: got %v, want %v", name, got, row.want)
		}
	}
}

func TestDeclares_NeedsAnIDOrNone(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		body string
		want bool
	}{
		"blank":          {"Closes-Inbox:\n", false},
		"placeholder":    {"Closes-Inbox: <inbox-item-id>\n", false},
		"prose":          {"Closes-Inbox: the salvage layer item\n", false},
		"none":           {"Closes-Inbox: none\n", true},
		"an id":          {"Closes-Inbox: item-a\n", true},
		"a ref":          {"Refs-Inbox: item-a\n", true},
		"a blank ref":    {"Refs-Inbox:\n", false},
		"a ref in prose": {"Refs-Inbox: the salvage layer item\n", false},
		"refs none":      {"Refs-Inbox: none\n", true},
	} {
		if got := Declares([]byte(row.body)); got != row.want {
			t.Errorf("%s: got %v, want %v", name, got, row.want)
		}
	}
}

func TestIDs_AnIDThatStartsWithNoneStillCloses(t *testing.T) {
	t.Parallel()
	if got, want := IDs([]byte("Closes-Inbox: none-such-item, nonesuch\n")), []string{"none-such-item", "nonesuch"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRefIDs_NoneIsNotAnID(t *testing.T) {
	t.Parallel()
	if got := RefIDs([]byte("Refs-Inbox: None\n")); got != nil {
		t.Errorf("got %v, want no id", got)
	}
}

func TestDeclares_AnyValidLineCountsInAnyOrder(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		body string
		want bool
	}{
		"valid then blank": {"Closes-Inbox: item-a\nCloses-Inbox:\n", true},
		"refs placeholder": {"Refs-Inbox: <id>\n", false},
	} {
		if got := Declares([]byte(row.body)); got != row.want {
			t.Errorf("%s: got %v, want %v", name, got, row.want)
		}
	}
}
