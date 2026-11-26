package inboxbatch

import (
	"encoding/json"
	"testing"
)

func TestItem_ReadsTheSourceThatAFilerStamps(t *testing.T) {
	var item Item
	raw := []byte(`{"id":"a","` + SourceField + `":"request:7/inbox-add-1"}`)

	if err := json.Unmarshal(raw, &item); err != nil || item.Source != "request:7/inbox-add-1" {
		t.Fatalf("item = %+v, err = %v, want the source of the file", item, err)
	}
}
