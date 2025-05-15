package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func emptySpace(space int) string {
	emptySpace := ""
	for i := 0; i < space; i++ {
		emptySpace += " "
	}
	return emptySpace
}
func progressBar(progress, width int) string {
	progressBar := ""
	for i := 0; i < progress; i++ {
		progressBar += "="
	}
	return progressBar
}
func progressbar(stage, stages int, pos, max uint64) {
	const progressBarWidth = 40
	if max > 0 {
		progress := int(pos * progressBarWidth / max)
		percent := int(pos * 100 / max)
		fmt.Printf("\r%d/%d [%s%s] %d%% ",
			stage, stages, progressBar(progress, progressBarWidth),
			emptySpace(progressBarWidth-progress), percent)
	}
}

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
	fmt.Println("  encode - Encode WAV/FLAC file/folder to JSON code")
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
	var centroids struct{ Centroids [][][]float64 }
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

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
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

	if is, _ := isDirectory(*inputFile); is {
		fmt.Println("Scanning directory...")
		var files []string
		filepath.Walk(*inputFile, func(path string, info fs.FileInfo, err error) error {
			var isFlac = strings.HasSuffix(path, ".flac")
			var isWav = strings.HasSuffix(path, ".wav")
			if !isFlac && !isWav {
				return nil
			}
			files = append(files, path)
			return nil
		})
		c := centroids_load(*centroidsFile)
		var output = make(map[string]json.RawMessage)
		progressbar(0, len(files), 0, uint64(len(files)))
		for i, file := range files {
			// Process audio
			jsonData, err := centroids_unvocode(file, c)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			if *outputFile != "" {
				output[filepath.Base(file)] = json.RawMessage(jsonData)
			} else {
				fmt.Println(string(jsonData))
			}
			progressbar(i+1, len(files), uint64(i+1), uint64(len(files)))
		}
		if *outputFile != "" {
			fatJson, err := json.MarshalIndent(output, "", " ")
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			os.WriteFile(*outputFile, fatJson, 0644)
		}
	} else {
		// Process audio
		jsonData, _ := centroids_unvocode(*inputFile, centroids_load(*centroidsFile))

		if *outputFile != "" {
			os.WriteFile(*outputFile, jsonData, 0644)
		} else {
			fmt.Println(string(jsonData))
		}
	}
}
