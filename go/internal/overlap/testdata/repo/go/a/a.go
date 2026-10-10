package a

import (
	_ "embed"

	"example.com/fix/b"
)

//go:embed data.txt
var Data string

func Name() string { return b.Name() }
