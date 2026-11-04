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
