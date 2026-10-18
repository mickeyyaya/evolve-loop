package c

import "example.com/fixture/b"

var total = b.Apply(Seed)

var hook = func() int { return Seed() }

func Seed() int { return 3 }
