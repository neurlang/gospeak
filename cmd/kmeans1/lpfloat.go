package main

import "fmt"

type LPFloat struct {
	Value  float64 // the actual value
	Digits int     // the number of digits used in json
}

func (l LPFloat) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%.*f", l.Digits, l.Value)
	return []byte(s), nil
}
