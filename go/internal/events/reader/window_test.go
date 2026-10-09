package reader

import (
	"strconv"
	"testing"
)

func TestWindow_RemembersTheLast4096IDs(t *testing.T) {
	t.Parallel()
	w := newWindow(windowSize)
	if w.seen("a") {
		t.Fatal("a first id was seen")
	}
	if !w.seen("a") {
		t.Fatal("a repeated id was not seen")
	}
	for i := range windowSize - 1 {
		w.seen(strconv.Itoa(i))
	}
	if !w.seen("a") {
		t.Fatal("the window forgot an id before 4096 others came")
	}
	for i := range windowSize {
		w.seen("x" + strconv.Itoa(i))
	}

	if w.seen("a") {
		t.Errorf("the window still holds an id after 4096 others came")
	}
}

func TestWindow_SizeIs4096(t *testing.T) {
	t.Parallel()
	if windowSize != 4096 {
		t.Errorf("windowSize = %d, want 4096 (D26)", windowSize)
	}
}
