package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/neurlang/classifier/hash"
	"io/ioutil"
	"log"
)

func main() {
	// Parse command line flags
	inputFile := flag.String("i", "", "Path to input.json")
	sttFile := flag.String("s", "", "Path to stt.json")
	flag.Parse()

	if *inputFile == "" || *sttFile == "" {
		flag.Usage()
		log.Fatal("Both -i and -s flags are required")
	}

	// Parse STT file
	sttMap, reversedSTT, hash, err := parseSTT(*sttFile)
	if err != nil {
		log.Fatal(err)
	}

	// Check for empty string key
	if _, ok := sttMap[""]; !ok {
		log.Fatal("hash not present")
	}

	// Parse input file and process
	if err := processInput(*inputFile, reversedSTT, hash); err != nil {
		log.Fatal(err)
	}
}

func parseSTT(path string) (map[string][]uint32, map[uint32]map[string]struct{}, uint32, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("error reading STT file: %v", err)
	}

	var sttMap map[string][]uint32
	if err := json.Unmarshal(data, &sttMap); err != nil {
		return nil, nil, 0, fmt.Errorf("error parsing STT JSON: %v", err)
	}

	var hash = sttMap[""]
	if len(hash) == 0 {
		return nil, nil, 0, fmt.Errorf("error loading hash from STT JSON, rerun isotonic1: %v", err)
	}

	// Reverse the STT map
	reversed := make(map[uint32]map[string]struct{})
	for key, values := range sttMap {
		for _, v := range values {
			if reversed[v] == nil {
				reversed[v] = make(map[string]struct{})
			}
			reversed[v][key] = struct{}{}
		}
	}

	return sttMap, reversed, hash[0], nil
}

func processInput(path string, reversedSTT map[uint32]map[string]struct{}, hash uint32) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading input file: %v", err)
	}

	// Try to parse as single input first
	var single []uint32
	if err := json.Unmarshal(data, &single); err == nil {
		return processSingle("", single, reversedSTT, hash)
	}

	// Try to parse as multiple inputs
	var multiple map[string][]uint32
	if err := json.Unmarshal(data, &multiple); err == nil {
		for f, nums := range multiple {
			if err := processSingle(f+":", nums, reversedSTT, hash); err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("input JSON format not recognized")
}

func processSingle(file string, nums []uint32, reversedSTT map[uint32]map[string]struct{}, hsh uint32) error {
	if len(nums)%8 != 0 {
		return fmt.Errorf("input length %d is not a multiple of 8", len(nums))
	}
	fmt.Print(file)
	for i := 0; i < len(nums); i += 8 {
		group := nums[i : i+8]
		var h = hsh
		for _, n := range group {
			h = hash.Hash(h, n, (1<<32)-1)
		}

		possible, ok := reversedSTT[h]
		if !ok {
			continue
		}
		if len(possible) == 1 {
			for str := range possible {
				fmt.Print(str)
			}
		}
	}
	fmt.Println()
	return nil
}
