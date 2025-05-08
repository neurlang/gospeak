package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		handleHelp()
	}

	switch os.Args[1] {
	case "decode":
		handleDecode()
	case "encode":
		handleEncode()
	case "-h", "help":
		handleHelp()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func handleHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  decode - Decode JSON code to WAV audio")
	fmt.Println("  encode - Encode WAV audio to JSON code")
	os.Exit(1)
}

func handleDecode() {
	start := time.Now()
	cmd := flag.NewFlagSet("decode", flag.ExitOnError)
	inputFile := cmd.String("i", "", "Input JSON file path")
	rawFile := cmd.String("r", "", "Raw JSON or comma separated sequence of integers")
	outputFile := cmd.String("o", "", "Output WAV file path")
	centroidsFile := cmd.String("v", "", "Centroids JSON file path")

	cmd.Parse(os.Args[2:])

	if (*inputFile == "" && *rawFile == "") || *outputFile == "" || *centroidsFile == "" {
		fmt.Println("All flags are required for decode:")
		cmd.PrintDefaults()
		os.Exit(1)
	}

	// Read input tokens
	var tokens []uint64
	if inputFile != nil && *inputFile != "" {
		data, err := os.ReadFile(*inputFile)
		if err != nil {
			panic(fmt.Sprintf("Error reading input file: %v", err))
		}

		if err := json.Unmarshal(data, &tokens); err != nil {
			panic(fmt.Sprintf("Error parsing JSON: %v", err))
		}
	} else {
		var vector = strings.Trim(*rawFile, "[] ")
		vector = strings.ReplaceAll(vector, " ", ",")
		for strings.Contains(vector, ",,") {
			vector = strings.ReplaceAll(vector, ",,", ",")
		}
		if err := json.Unmarshal([]byte("["+vector+"]"), &tokens); err != nil {
			panic(fmt.Sprintf("Error parsing JSON: %v", err))
		}
		fmt.Println(tokens)
	}

	// Convert to uint32 slice
	var tokens32 []uint32
	for _, t := range tokens {
		tokens32 = append(tokens32, uint32(t))
	}

	// Load centroids
	var centroids struct{ Centroids [][]float64 }
	data, err := os.ReadFile(*centroidsFile)
	if err != nil {
		panic(fmt.Sprintf("Error reading centroids file: %v", err))
	}

	if err := json.Unmarshal(data, &centroids); err != nil {
		panic(fmt.Sprintf("Error parsing centroids JSON: %v", err))
	}

	// Generate audio
	centroids_vocode(tokens32, centroids.Centroids, *outputFile)
	fmt.Printf("Decoding completed in %v\n", time.Since(start))
}

func handleEncode() {
	cmd := flag.NewFlagSet("encode", flag.ExitOnError)
	inputFile := cmd.String("i", "", "Input WAV file path")
	outputFile := cmd.String("o", "", "Output JSON file path")
	centroidsFile := cmd.String("v", "", "Centroids JSON file path")

	cmd.Parse(os.Args[2:])

	if *inputFile == "" || *centroidsFile == "" {
		fmt.Println("Input file and centroids file are required")
		cmd.PrintDefaults()
		os.Exit(1)
	}

	// Process audio
	centroids_unvocode(*inputFile, *centroidsFile, *outputFile)
}
