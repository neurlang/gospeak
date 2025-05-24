package main

import (
	"encoding/json"
	"fmt"
	"github.com/neurlang/classifier/hash"
	"github.com/neurlang/classifier/parallel"
	"io/ioutil"
	"os"
	"sync"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage: go run bigram_generator.go <input.json> <input2.json> <output.json> <output2.json>")
		return
	}

	inputFile := os.Args[1]
	inputFile2 := os.Args[2]
	outputFile := os.Args[3]
	outputFile2 := os.Args[4]

	// Read the JSON file
	content, err := ioutil.ReadFile(inputFile)
	if err != nil {
		panic(err)
	}

	var data map[string][]uint32
	err = json.Unmarshal(content, &data)
	if err != nil {
		panic(err)
	}

	// Read the JSON file
	content2, err := ioutil.ReadFile(inputFile2)
	if err != nil {
		panic(err)
	}

	var data2 map[string]string
	err = json.Unmarshal(content2, &data2)
	if err != nil {
		panic(err)
	}

	for k := range data {
		if l, ok := data2[k]; ok && len([]rune(l)) > 0 {
			continue
		}
		if len(k) >= 5 {
			if _, okFlac := data2[k[:len(k)-5]]; okFlac {
				data2[k] = data2[k[:len(k)-5]]
				continue
			}
		}
		if len(k) >= 4 {
			if _, okWav := data2[k[:len(k)-4]]; okWav {
				data2[k] = data2[k[:len(k)-4]]
				continue
			}
		}
		println("No IPA found for file: ", k)
		delete(data, k)
	}

	var hashdata = make(map[[8]uint32]struct{})
	for _, v := range data {
		for i := 0; i < len(v); i += 8 {
			hashdata[[8]uint32{v[i], v[i+1], v[i+2], v[i+3], v[i+4], v[i+5], v[i+6], v[i+7]}] = struct{}{}
		}
	}
	var framedata [][8]uint32
	for v := range hashdata {
		framedata = append(framedata, v)
	}

	var sol uint32
	parallel.Loop(1000).LoopUntil(func(i uint32, _ parallel.LoopStopper) bool {
		var usedLock sync.Mutex
		var used = make(map[uint32]struct{})
		parallel.Loop(1000).LoopUntil(func(j uint32, _ parallel.LoopStopper) bool {
			if uint64(j) >= uint64(len(framedata)) {
				return true
			}
			var h = i
			for k := 0; k < 8; k++ {
				h = hash.Hash(h, framedata[j][k], (1<<32)-1)
			}
			usedLock.Lock()
			defer usedLock.Unlock()
			if _, ok := used[h]; ok {
				return true
			} else {
				used[h] = struct{}{}
			}
			return false
		})

		usedLock.Lock()
		if len(used) == len(framedata) {
			sol = i
			usedLock.Unlock()
			return true
		}
		usedLock.Unlock()
		return false
	})

	var odata = make(map[string][]uint32)
	for j, v := range data {
		var buffer []uint32
		for i := 0; i < len(v); i += 8 {
			var h = sol
			for k := 0; k < 8; k++ {
				h = hash.Hash(h, v[i+k], (1<<32)-1)
			}
			buffer = append(buffer, h)
		}
		var key = data2[j]
		for _, ok := odata[key]; ok; _, ok = odata[key] {
			key += " "
		}
		odata[key] = buffer
	}
	{
		// Convert to JSON
		jsonData, err := json.MarshalIndent(odata, "", "  ")
		if err != nil {
			panic(err)
		}

		// Save to file
		err = ioutil.WriteFile(outputFile2, jsonData, 0644)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Odata saved to %s\n", outputFile2)
	}

	var bigrams struct {
		Hash    uint32
		Bigrams map[string]map[string]int
	}
	bigrams.Bigrams = make(map[string]map[string]int)
	bigrams.Hash = sol

	for k, v := range data {
		var keys []string
		for i := 0; i < len(v); i += 8 {
			var key = fmt.Sprintf("%d %d %d %d %d %d %d %d",
				v[i+0], v[i+1], v[i+2], v[i+3],
				v[i+4], v[i+5], v[i+6], v[i+7])
			keys = append(keys, key)
		}

		// put initial letter bigram
		initial := string([]rune(data2[k])[0])
		if _, exists := bigrams.Bigrams[initial]; !exists {
			bigrams.Bigrams[initial] = make(map[string]int)
		}
		bigrams.Bigrams[initial][keys[0]]++

		// put actual integer-to-integer bigrams
		for i := 0; i < len(keys)-1; i++ {
			current := keys[i]
			next := keys[i+1]

			if _, exists := bigrams.Bigrams[current]; !exists {
				bigrams.Bigrams[current] = make(map[string]int)
			}
			bigrams.Bigrams[current][next]++
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
