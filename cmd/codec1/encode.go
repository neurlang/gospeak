package main

import (
	"encoding/json"
	"fmt"
	"github.com/neurlang/gomel/phase"
	"math"
	"os"
)

func centroids_unvocode(inputFile, centroidsFile, outputFile string) {

	// Load audio file
	audio, sampleRate, err := phase.LoadWavSampleRate(inputFile)
	if err != nil {
		panic(fmt.Sprintf("Error loading audio: %v", err))
	}

	// Initialize phase converter
	m := phase.NewPhase()
	m.YReverse = true
	m.Window = 640 * 2
	m.Resolut = 2048 * 2
	m.VolumeBoost = 4

	// Determine frequency bands based on sample rate
	switch sampleRate {
	case 44100:
		m.NumFreqs = 418 * 2
	case 48000:
		m.NumFreqs = 384 * 2
	default:
		panic("Unsupported sample rate")
	}

	// Convert to mel spectrogram
	melFrames, err := m.ToPhase(audio)
	if err != nil {
		panic(fmt.Sprintf("Error creating spectrogram: %v", err))
	}

	// Load centroids
	var centroidData struct{ Centroids [][]float64 }
	data, err := os.ReadFile(centroidsFile)
	if err != nil {
		panic(fmt.Sprintf("Error reading centroids: %v", err))
	}
	json.Unmarshal(data, &centroidData)

	// Find nearest centroids for each frame
	var indices []uint64
	frameSize := m.NumFreqs
	for j := 0; j < len(melFrames); j += frameSize {
		if j+frameSize > len(melFrames) {
			break
		}

		// Calculate key coordinates for current frame
		var keyCoords []float64
		for i := 0; i < frameSize; i++ {
			frame := melFrames[j+i]
			val1 := math.Sqrt(math.Pow(math.Exp2(frame[1]), 2) + math.Pow(math.Exp2(frame[2]), 2))
			val2 := math.Sqrt(math.Pow(math.Exp2(frame[0]), 2) + math.Pow(math.Exp2(frame[1]), 2))
			keyCoords = append(keyCoords, val1, val2)
		}

		// Find closest centroid
		minDist := math.MaxFloat64
		nearestIdx := 0
		for idx, centroid := range centroidData.Centroids {
			var valueCoords []float64
			for i := 0; i < frameSize; i++ {
				if 3*i+2 >= len(centroid) {
					break
				}
				c0 := centroid[3*i]
				c1 := centroid[3*i+1]
				c2 := centroid[3*i+2]

				val1 := math.Sqrt(math.Pow(math.Exp2(c1), 2) + math.Pow(math.Exp2(c2), 2))
				val2 := math.Sqrt(math.Pow(math.Exp2(c0), 2) + math.Pow(math.Exp2(c1), 2))
				valueCoords = append(valueCoords, val1, val2)
			}

			if len(keyCoords) != len(valueCoords) {
				continue
			}

			var dist float64
			for k := range keyCoords {
				diff := keyCoords[k] - valueCoords[k]
				dist += diff * diff
			}
			if dist < minDist {
				minDist = dist
				nearestIdx = idx
			}
		}

		indices = append(indices, uint64(nearestIdx))
	}

	// Output results
	jsonData, _ := json.Marshal(indices)
	if outputFile != "" {
		os.WriteFile(outputFile, jsonData, 0644)
		fmt.Printf("Encoded %d frames to %s\n", len(indices), outputFile)
	} else {
		fmt.Println(string(jsonData))
	}
}
