package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

type Entry struct {
	String    string
	Centroids []uint32
	Collapsed []rune
}

func collapseRuns(s string) []rune {
	var collapsed []rune
	for _, r := range s {
		if len(collapsed) == 0 || r != collapsed[len(collapsed)-1] {
			collapsed = append(collapsed, r)
		}
	}
	return collapsed
}

func main() {
	// Parse command line flags
	inputFile := flag.String("i", "", "Path to input file study.json")
	sttFile := flag.String("o", "", "Path to output file stt.json")
	verbose := flag.Bool("v", false, "Verbose output")
	radius := flag.Int("r", 0, "Radius of IPA presence (higher radius relaxes the forced alignment)")
	flag.Parse()

	if *inputFile == "" || *sttFile == "" {
		flag.Usage()
		log.Fatal("Both -i and -s flags are required")
	}

	file, err := os.Open(*inputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	data := make(map[string][]uint32)
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		log.Fatal(err)
	}

	var hash = data[""]
	delete(data, "")

	var stringData []Entry
	allCentroids := make(map[uint32]bool)

	for s, centroids := range data {
		collapsed := collapseRuns(s)
		entry := Entry{
			String:    s,
			Centroids: centroids,
			Collapsed: collapsed,
		}
		stringData = append(stringData, entry)
		for _, cent := range centroids {
			allCentroids[cent] = true
		}
	}

	possibleChars := make(map[uint32]map[rune]bool)
	for _, entry := range stringData {
		for _, cent := range entry.Centroids {
			if _, ok := possibleChars[cent]; !ok {
				possibleChars[cent] = make(map[rune]bool)
			}
			for _, c := range entry.Collapsed {
				possibleChars[cent][c] = true
			}
		}
	}

	changed := true
	maxIterations := 20
	for iter := 0; iter < maxIterations && changed; iter++ {
		changed = false
		newPossible := make(map[uint32]map[rune]bool)

		for _, entry := range stringData {
			centroids := entry.Centroids
			runs := entry.Collapsed
			R := len(runs)
			L := len(centroids)

			if R > L {
				fmt.Printf("Warning: Not enough centroids for %s\n", entry.String)
				continue
			}

			minStart := make([]int, R)
			maxEnd := make([]int, R)
			for i := 0; i < R; i++ {
				minStart[i] = i
				maxEnd[i] = L - 1 - i
			}

			current := 0
			for i := 0; i < R; i++ {
				c := runs[i]
				for current < L && !possibleChars[centroids[current]][c] {
					current++
				}
				if current >= L {
					fmt.Printf("Error: No valid start for %c in %s\n", c, entry.String)
					return
				}
				minStart[i] = current
				current++
			}

			current = L - 1
			for i := R - 1; i >= 0; i-- {
				c := runs[i]
				for current >= 0 && !possibleChars[centroids[current]][c] {
					current--
				}
				if current < 0 {
					fmt.Printf("Error: No valid end for %c in %s\n", c, entry.String)
					return
				}
				maxEnd[i] = current
				current--
			}

			for i := 0; i < R; i++ {
				c := runs[i]
				for j := minStart[i]; j <= maxEnd[i]; j++ {
					if ((i*L)/R)/(2 * *radius + 1) == j/(2 * *radius + 1) {
						cent := centroids[j]
						if _, ok := newPossible[cent]; !ok {
							newPossible[cent] = make(map[rune]bool)
						}
						newPossible[cent][c] = true
					}
				}
			}
		}

		for cent := range allCentroids {
			newSet, ok := newPossible[cent]
			if !ok {
				continue
			}
			currentSet := possibleChars[cent]
			updatedSet := make(map[rune]bool)
			for c := range newSet {
				if currentSet[c] {
					updatedSet[c] = true
				}
			}
			if !equal(updatedSet, currentSet) {
				possibleChars[cent] = updatedSet
				changed = true
			}
		}
	}

	out := make(map[string][]uint32)
	out[""] = hash

	invertedMap := make(map[string][]uint32)

	var centroids []uint32
	for cent := range possibleChars {
		centroids = append(centroids, cent)
	}

	for _, cent := range centroids {
		chars := possibleChars[cent]
		var charSlice []string
		for r := range chars {
			charSlice = append(charSlice, string(r))
		}
		if verbose != nil && *verbose {
			sort.Strings(charSlice)
			fmt.Printf("%d: %s\n", cent, strings.Join(charSlice, ", "))
		}
		for _, ch := range charSlice {
			invertedMap[ch] = append(invertedMap[ch], cent)
		}
	}

	for ch, cents := range invertedMap {
		out[ch] = cents
	}

	outputFile, err := os.Create(*sttFile)
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	encoder := json.NewEncoder(outputFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		log.Fatal(err)
	}
}

func equal(a, b map[rune]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
