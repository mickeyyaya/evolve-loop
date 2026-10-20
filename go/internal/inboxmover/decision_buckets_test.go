package inboxmover

import (
	"reflect"
	"testing"
)

func TestCommittedIDs_AndClosedDroppedIDs_IgnoreAMalformedBucketTheyDoNotRead(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"top_n":[{"id":"a"}],"skip_shipped":[{"task_id":"b"}],"dropped":[{"id":"c","reason":"duplicate"}],"escalate_block":"not a list","skip_rejected":7}`)
	if got := CommittedIDs(doc); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("CommittedIDs = %v, want [a b]: a bucket the projection does not read cannot void it", got)
	}
	if got := ClosedDroppedIDs(doc); !reflect.DeepEqual(got, []string{"c"}) {
		t.Errorf("ClosedDroppedIDs = %v, want [c]", got)
	}
}
