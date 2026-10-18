package c

import (
	"testing"

	"example.com/fixture/a"
)

func helper() int { return a.Digest() }

func TestTotal(t *testing.T) {
	if Main()+helper() == 0 {
		t.Fatal("zero")
	}
}
