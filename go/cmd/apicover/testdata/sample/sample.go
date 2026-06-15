// Package sample is a fixture for apicover's enumeration tests. It deliberately
// mixes exported and unexported declarations of every kind.
package sample

// ExportedFunc is an exported top-level function.
func ExportedFunc() {}

func unexportedFunc() {}

// ExportedType is an exported type with both exported and unexported methods.
type ExportedType struct{}

// ExportedMethod is an exported method (keyed "ExportedType.ExportedMethod").
func (ExportedType) ExportedMethod() {}

func (ExportedType) unexportedMethod() {}

// ExportedVar is an exported package-level var.
var ExportedVar = 1

var unexportedVar = 2

// ExportedConst is an exported package-level const.
const ExportedConst = 3
