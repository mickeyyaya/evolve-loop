package routingtest

import "strings"

// variant is one labeled choice along a Matrix dimension.
type variant struct {
	label  string
	bricks []Brick
}

// V builds a labeled dimension variant (the label appears in the generated
// scenario name, so failures are debuggable).
func V(label string, bricks ...Brick) variant { return variant{label: label, bricks: bricks} }

// Dimension is one axis of the cross-product.
type Dimension struct {
	name     string
	variants []variant
}

// Dim builds a named dimension from labeled variants.
func Dim(name string, vs ...variant) Dimension { return Dimension{name: name, variants: vs} }

// Matrix expands base bricks × every combination of dimension variants into a
// flat []ScenarioSpec. This is the literal "send a set of configs to trigger
// different phase combinations": one Matrix call yields product(len(dim)) specs,
// each named "<dim>=<label>/<dim>=<label>...". The total count equals the
// product of dimension sizes (asserted in matrix_test.go).
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
