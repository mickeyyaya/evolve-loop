package phaseidentity

import "testing"

func TestAPICoverNamedExports(t *testing.T) {
	var f Facts = facts1707
	if got := Block(f); len(got) == 0 {
		t.Fatal("Block")
	}
	var h string = Heading
	if h == "" {
		t.Fatal("Heading")
	}
}
