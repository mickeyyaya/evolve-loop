package inboxmover

import (
	"reflect"
	"testing"
)

func TestClosesInboxIDsIsTheParserOfTheLeafPackage(t *testing.T) {
	t.Parallel()
	got := ClosesInboxIDs([]byte("Closes-Inbox: alpha, beta\n- Closes-Inbox: `gamma`\n"))
	if want := []string{"alpha", "beta", "gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ClosesInboxIDs = %#v, want %#v", got, want)
	}
}

func TestClosesInboxIDs_NoneIsAnExplicitEmptySet(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"alone":      "Closes-Inbox: none\n",
		"upper case": "CLOSES-INBOX: NONE\n",
		"backticked": "- Closes-Inbox: `none`\n",
	} {
		if got := ClosesInboxIDs([]byte(body)); got != nil {
			t.Errorf("%s: ClosesInboxIDs = %#v, want nil", name, got)
		}
	}
	got := ClosesInboxIDs([]byte("Closes-Inbox: none, real-item\n"))
	if want := []string{"real-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("none in a list: ClosesInboxIDs = %#v, want %#v", got, want)
	}
	got = ClosesInboxIDs([]byte("Closes-Inbox: none-such-item\n"))
	if want := []string{"none-such-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("an id that starts with none: ClosesInboxIDs = %#v, want %#v", got, want)
	}
}

func TestRefsInboxIDs_ReadsItsOwnMarkerOnly(t *testing.T) {
	t.Parallel()
	body := []byte("Closes-Inbox: closed-item\nRefs-Inbox: partial-item, other-item\n- refs-inbox: `partial-item`\n  Refs-Inbox: indented-item\nA mention Refs-Inbox: mid-sentence\n")
	if got, want := RefsInboxIDs(body), []string{"partial-item", "other-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("RefsInboxIDs = %#v, want %#v", got, want)
	}
	if got, want := ClosesInboxIDs(body), []string{"closed-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ClosesInboxIDs = %#v, want %#v: a Refs-Inbox line closes nothing", got, want)
	}
}

func TestRefsInboxIDs_NoneIsNotAnID(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"none":       "Refs-Inbox: none\n",
		"upper none": "Refs-Inbox: None\n",
		"empty body": "",
		"no marker":  "# Build Report\n",
	} {
		if got := RefsInboxIDs([]byte(body)); got != nil {
			t.Errorf("%s: RefsInboxIDs = %#v, want nil", name, got)
		}
	}
	if got, want := RefsInboxIDs([]byte("Refs-Inbox: none, real-item\n")), []string{"real-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("none in a list: RefsInboxIDs = %#v, want %#v", got, want)
	}
}
