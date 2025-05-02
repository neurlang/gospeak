package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

func compress_numbers_into_tokens(n []string) (ret []string) {
	var dst_uints = make([]uint32, (len(n)+1)/2, (len(n)+1)/2)
	for i, tok := range n {
		n, _ := strconv.Atoi(tok)
		shift := (1 - (i % 2)) * 15 // Reverse the shift direction
		dst_uints[i/2] |= uint32(n+1) << shift
	}
	for _, r := range dst_uints {
		ret = append(ret, fmt.Sprint(r))
	}
	return
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run bigram_generator.go <input.tsv> <output.json>")
		return
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	// Read the TSV file
	content, err := ioutil.ReadFile(inputFile)
	if err != nil {
		panic(err)
	}

	// Parse the TSV file
	lines := strings.Split(string(content), "\n")
	bigrams := make(map[string]map[string]int)

	for _, line := range lines {
		if line == "" {
			continue
		}
		columns := strings.Split(line, "\t")
		if len(columns) < 2 {
			continue
		}

		numbers := compress_numbers_into_tokens(strings.Fields(columns[1]))

		// put initial letter bigram
		initial := string([]rune(columns[0])[0])
		if _, exists := bigrams[initial]; !exists {
			bigrams[initial] = make(map[string]int)
		}
		bigrams[initial][numbers[0]]++

		// put actual integer-to-integer bigrams
		for i := 0; i < len(numbers)-1; i++ {
			current := numbers[i]
			next := numbers[i+1]

			if _, exists := bigrams[current]; !exists {
				bigrams[current] = make(map[string]int)
			}
			bigrams[current][next]++
		}
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(bigrams, "", "  ")
	if err != nil {
		panic(err)
	}

	// Save to file
	err = ioutil.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Bigrams saved to %s\n", outputFile)
}
