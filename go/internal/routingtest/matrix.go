package routingtest

import "strings"

type variant struct {
	label  string
	bricks []Brick
}

func V(label string, bricks ...Brick) variant { return variant{label: label, bricks: bricks} }

type Dimension struct {
	name     string
	variants []variant
}

func Dim(name string, vs ...variant) Dimension { return Dimension{name: name, variants: vs} }

func Matrix(base []Brick, dims ...Dimension) []ScenarioSpec {
	combos := []([]variant){{}}
	for _, d := range dims {
		var next []([]variant)
		for _, prefix := range combos {
			for _, v := range d.variants {
				next = append(next, append(append([]variant{}, prefix...), v))
			}
		}
		combos = next
	}

	specs := make([]ScenarioSpec, 0, len(combos))
	for _, combo := range combos {
		bricks := append([]Brick{}, base...)
		labels := make([]string, 0, len(combo))
		for i, v := range combo {
			bricks = append(bricks, v.bricks...)
			labels = append(labels, dims[i].name+"="+v.label)
		}
		specs = append(specs, Scenario(strings.Join(labels, "/"), bricks...))
	}
	return specs
}
