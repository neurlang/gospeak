package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {
	// Check command line arguments
	if len(os.Args) != 3 {
		fmt.Println("Usage: phon <input.csv> <output.json>")
		os.Exit(1)
	}

	// Open the CSV file
	csvFile, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer csvFile.Close()

	// Create a custom CSV reader with pipe delimiter
	reader := csv.NewReader(csvFile)
	reader.Comma = '|'         // Set delimiter to pipe
	reader.FieldsPerRecord = 2 // Expect exactly 2 fields per record

	// Create a map to store the key-value pairs
	data := make(map[string]string)

	// Read and process each record
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Failed to read CSV record: %v", err)
		}

		// Trim whitespace from fields
		key := strings.TrimSpace(record[0])
		value := strings.TrimSpace(record[1])

		// Add to map
		data[key] = value
	}

	// Convert map to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Open the JSON file
	jsonFile, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatalf("Failed to open JSON file: %v", err)
	}
	defer jsonFile.Close()

	// Print the JSON
	fmt.Fprintln(jsonFile, string(jsonData))
}
