package main

import (
	"encoding/json"
	"fmt"
	"github.com/neurlang/classifier/parallel"
	"github.com/neurlang/gomel/phase"
	"math"
	"os"
	"strings"
)

type Stuffer int

func (numFreqs *Stuffer) doZeroStuff(audio []float64, sampleRate uint32, err error) ([]float64, int) {
	if err != nil {
		return nil, 0
	}
	switch sampleRate {
	case 8000, 16000, 48000:
		if *numFreqs != 384*2 {
			return nil, 0
		}
	case 11025, 22050, 44100:
		if *numFreqs != 418*2 {
			return nil, 0
		}
	}
	switch sampleRate {
	case 8000:
		return audio, 5
	case 11025:
		return audio, 3
	case 16000:
		return audio, 2
	case 22050:
		return audio, 1
	default:
		return audio, 0
	}
}

func zeroStuffing(audio []float64, zerosCount int) (result []float64) {
	if zerosCount == 0 {
		return audio
	}
	result = make([]float64, 0, len(audio)*(zerosCount+1))
	for _, v := range audio {
		result = append(result, v)
		for i := 0; i < zerosCount; i++ {
			result = append(result, 0)
		}
	}
	return
}

func centroids_unvocode(inputFile, centroidsFile, outputFile string) {

	var audio []float64
	var sampleRate uint32
	var err error

	if strings.HasSuffix(inputFile, ".flac") {
		// Load flac file
		audio, sampleRate, err = phase.LoadFlacSampleRate(inputFile)
		if err != nil {
			panic(fmt.Sprintf("Error loading audio: %v", err))
		}
	} else {
		// Load audio file
		audio, sampleRate, err = phase.LoadWavSampleRate(inputFile)
		if err != nil {
			panic(fmt.Sprintf("Error loading audio: %v", err))
		}
	}

	// Initialize phase converter
	m := phase.NewPhase()
	m.YReverse = true
	m.Window = 640 * 2
	m.Resolut = 2048 * 2
	m.VolumeBoost = 4
	var ranges []int

	// Determine frequency bands based on sample rate
	switch sampleRate {
	case 11025, 22050, 44100:
		m.NumFreqs = 418 * 2
		ranges = []int{0, 41, 95, 145, 200, 254, 400, 545, 418 * 2}
	case 8000, 16000, 48000:
		m.NumFreqs = 384 * 2
		ranges = []int{0, 38, 88, 134, 184, 234, 367, 501, 384 * 2}
	default:
		panic("Unsupported sample rate")
	}
	var s = Stuffer(m.NumFreqs)
	audio = zeroStuffing((&s).doZeroStuff(audio, sampleRate, err))

	// Convert to mel spectrogram
	melFrames, err := m.ToPhase(audio)
	if err != nil {
		panic(fmt.Sprintf("Error creating spectrogram: %v", err))
	}

	audio = nil

	// Load centroids
	var centroidData struct{ Centroids [][][]float64 }
	data, err := os.ReadFile(centroidsFile)
	if err != nil {
		panic(fmt.Sprintf("Error reading centroids: %v", err))
	}
	json.Unmarshal(data, &centroidData)

	println(len(melFrames))

	// Find nearest centroids for each frame
	frameSize := m.NumFreqs
	var indices = make([]uint64, 8*len(melFrames)/frameSize, 8*len(melFrames)/frameSize)
	parallel.ForEach(len(melFrames)/frameSize, 100, func(jj int) {
		j := jj * frameSize
		if j+frameSize > len(melFrames) {
			return
		}
		for rang := 0; rang < 8; rang++ {
			if len(centroidData.Centroids) <= rang {
				break
			}
			if len(centroidData.Centroids[rang]) == 0 {
				break
			}

			// Calculate key coordinates for current frame
			var keyCoords []float64
			for i := ranges[rang]; i < ranges[rang+1]; i++ {
				frame := melFrames[j+i]
				val1 := math.Sqrt(math.Pow(math.Exp2(frame[1]), 2) + math.Pow(math.Exp2(frame[2]), 2))
				val2 := math.Sqrt(math.Pow(math.Exp2(frame[0]), 2) + math.Pow(math.Exp2(frame[1]), 2))
				keyCoords = append(keyCoords, val1, val2)
			}

			// Find closest centroid
			minDist := math.MaxFloat64
			nearestIdx := 0
			for idx, centroid := range centroidData.Centroids[rang] {
				if len(centroid) == 0 {
					continue
				}
				var valueCoords []float64
				for i := 0; i < ranges[rang+1]-ranges[rang]; i++ {
					if 3*i+2 >= len(centroid) {
						println("exceeded len centroid", 3*i+2, len(centroid))
						return
					}
					c0 := centroid[3*i]
					c1 := centroid[3*i+1]
					c2 := centroid[3*i+2]

					val1 := math.Sqrt(math.Pow(math.Exp2(c1), 2) + math.Pow(math.Exp2(c2), 2))
					val2 := math.Sqrt(math.Pow(math.Exp2(c0), 2) + math.Pow(math.Exp2(c1), 2))
					valueCoords = append(valueCoords, val1, val2)
				}

				if len(keyCoords) != len(valueCoords) {
					println("keyCoords don't match valueCoords", len(keyCoords), len(valueCoords))
					return
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
			indices[8*jj+rang] = uint64(nearestIdx)
		}
	})

	// Output results
	jsonData, _ := json.Marshal(indices)
	if outputFile != "" {
		os.WriteFile(outputFile, jsonData, 0644)
		fmt.Printf("Encoded %d frames to %s\n", len(indices), outputFile)
	} else {
		fmt.Println(string(jsonData))
	}
}
