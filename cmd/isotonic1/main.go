package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	data, err := os.ReadFile("../../dict/glados/study.json")
	if err != nil {
		panic(err)
	}

	var input map[string][]uint32
	err = json.Unmarshal(data, &input)
	if err != nil {
		panic(err)
	}

	// Collect all unique characters across all strings
	allChars := make(map[rune]struct{})
	for s := range input {
		for _, c := range s {
			allChars[c] = struct{}{}
		}
	}

	// Initialize possible characters for each centroid to all characters initially
	possibleChars := make(map[uint32]map[rune]struct{})
	// Track which centroids exist
	centroidExists := make(map[uint32]bool)
	for _, centroids := range input {
		for _, cent := range centroids {
			if !centroidExists[cent] {
				pc := make(map[rune]struct{})
				for c := range allChars {
					pc[c] = struct{}{}
				}
				possibleChars[cent] = pc
				centroidExists[cent] = true
			}
		}
	}

	// First pass: Intersect possible characters with each entry's string
	for s, centroids := range input {
		charsInString := make(map[rune]struct{})
		for _, c := range s {
			charsInString[c] = struct{}{}
		}

		for _, cent := range centroids {
			current := possibleChars[cent]
			newPossible := make(map[rune]struct{})
			for c := range current {
				if _, ok := charsInString[c]; ok {
					newPossible[c] = struct{}{}
				}
			}
			possibleChars[cent] = newPossible
		}
	}

	// Iteratively apply monotonic constraints
	changed := true
	for changed {
		changed = false

		for s, centroids := range input {
			chars := []rune(s)
			charIndex := make(map[rune]int)
			for i, c := range chars {
				charIndex[c] = i
			}

			for i := 0; i < len(centroids); i++ {
				cent := centroids[i]
				currentPossible := possibleChars[cent]
				if len(currentPossible) == 0 {
					continue
				}

				for c := range currentPossible {
					cPos, ok := charIndex[c]
					if !ok {
						continue // shouldn't happen after first pass
					}

					invalid := false
					// Check all subsequent centroids
					for j := i + 1; j < len(centroids); j++ {
						nextCent := centroids[j]
						nextPossible := possibleChars[nextCent]
						for d := range nextPossible {
							dPos := charIndex[d]
							if dPos < cPos {
								invalid = true
								break
							}
						}
						if invalid {
							break
						}
					}

					if invalid {
						delete(currentPossible, c)
						changed = true
					}
				}
			}
		}
	}

	// Output the results
	for cent, chars := range possibleChars {
		if len(chars) == 1 {
			for c := range chars {
				fmt.Printf("%d: %c\n", cent, c)
			}
		} else if len(chars) > 0 {
			fmt.Printf("%d: Possible characters: ", cent)
			for c := range chars {
				fmt.Printf("%c ", c)
			}
			fmt.Println()
		} else {
			fmt.Printf("%d: No possible characters\n", cent)
		}
	}
}
