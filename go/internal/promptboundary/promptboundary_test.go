package promptboundary

import "testing"

func TestPlace(t *testing.T) {
	cases := []struct{ name, prompt, block, want string }{
		{"after boundary", "BODY" + Boundary + "- cycle: 1\n", "BLK\n", "BODY" + Boundary + "BLK\n- cycle: 1\n"},
		{"no boundary appends one", "BODY", "BLK\n", "BODY" + Boundary + "BLK\n"},
		{"empty block without boundary adds none", "BODY", "", "BODY"},
		{"empty block is identity", "BODY" + Boundary + "x", "", "BODY" + Boundary + "x"},
		{"empty prompt", "", "BLK\n", Boundary + "BLK\n"},
		{"first boundary wins", "A" + Boundary + "B" + Boundary + "C", "X", "A" + Boundary + "XB" + Boundary + "C"},
	}
	for _, c := range cases {
		if got := Place(c.prompt, c.block); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestStaticPrefix(t *testing.T) {
	cases := []struct{ name, prompt, want string }{
		{"cut at boundary", "BODY" + Boundary + "- cycle: 1\n", "BODY"},
		{"no boundary", "BODY", "BODY"},
		{"empty", "", ""},
		{"first boundary wins", "A" + Boundary + "B" + Boundary + "C", "A"},
	}
	for _, c := range cases {
		if got := StaticPrefix(c.prompt); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestPlaceThenStaticPrefixIgnoresBlock(t *testing.T) {
	base := "BODY" + Boundary + "- cycle: 1\n"
	if StaticPrefix(Place(base, "one")) != StaticPrefix(Place(base, "two")) {
		t.Fatal("block content must never reach the static prefix")
	}
}
