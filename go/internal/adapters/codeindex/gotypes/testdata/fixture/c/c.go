package c

import (
	"example.com/fixture/a"
	"example.com/fixture/b"
)

func Main() int {
	return a.Digest() + b.Digest() + b.Drive(a.Engine{}) + b.Apply(b.Digest)
}

func OnlyB() int { return b.Digest() }
