package main

import "fmt"

type LPFloat struct {
	Value  float64 // the actual value
	Digits int     // the number of digits used in json
}

func (l LPFloat) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%.*f", l.Digits, l.Value)
	for len(s) > 2 {
		if s[len(s)-1] == '0' && s[len(s)-2] != '.' {
			s = s[:len(s)-1]
		} else {
			break
		}
	}
	return []byte(s), nil
}
