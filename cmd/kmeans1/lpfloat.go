package main

import "fmt"
import "sync/atomic"
import "math"

type LPFloat struct {
	Value  float64 // the actual value
	Digits int     // the number of digits used in json
}

func (l LPFloat) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%.*f", l.Digits, l.Value)
	return []byte(s), nil
}

var badFloatDetected atomic.Bool

func verifyFloat(value float64) float64 {
	if badFloatDetected.Load() {
		return value
	}
	badFloatDetected.Store(true)
	if math.IsNaN(value) {
		println("\nbadFloatDetected: NaN")
	}
	if math.IsInf(value, 1) {
		println("\nbadFloatDetected: +Inf")
	}
	if math.IsInf(value, -1) {
		println("\nbadFloatDetected: -Inf")
	}
	return value
}
