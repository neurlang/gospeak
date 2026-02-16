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
		if *numFreqs != 384*2*2 {
			return nil, 0
		}
	case 11025, 22050, 44100:
		if *numFreqs != 418*2*2 {
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

func centroids_load(centroidsFile string) [][][]float64 {

	// Load centroids
	var centroidData struct{ Centroids [][][]float64 }
	data, err := os.ReadFile(centroidsFile)
	if err != nil {
		panic(fmt.Sprintf("Error reading centroids: %v", err))
	}
	json.Unmarshal(data, &centroidData)

	// Precompute valueCoords for all centroids
	precomputedCentroidValueCoords := make([][][]float64, 8) // 8 ranges
	for rang := 0; rang < 8; rang++ {
		if rang >= len(centroidData.Centroids) {
			continue
		}
		centroidsInRange := centroidData.Centroids[rang]
		if len(centroidsInRange) == 0 {
			continue
		}
		precomputedCentroidValueCoords[rang] = make([][]float64, len(centroidsInRange))
		for idx, centroid := range centroidsInRange {
			var valueCoords []float64
			for i := 0; 3*i+2 < len(centroid); i++ {
				c0 := centroid[3*i]
				c1 := centroid[3*i+1]
				c2 := centroid[3*i+2]
				valueCoords = append(valueCoords, c0, c1, c2)
			}
			precomputedCentroidValueCoords[rang][idx] = valueCoords
		}
	}
	return precomputedCentroidValueCoords
}

func centroids_unvocode(inputFile string, centroids [][][]float64) (data []byte, err error) {

	var audio []float64
	var sampleRate uint32

	if strings.HasSuffix(inputFile, ".flac") {
		// Load flac file
		audio, sampleRate, err = phase.LoadFlacSampleRate(inputFile)
		if err != nil {
			return nil, fmt.Errorf("Error loading audio: %v", err)
		}
	} else {
		// Load audio file
		audio, sampleRate, err = phase.LoadWavSampleRate(inputFile)
		if err != nil {
			return nil, fmt.Errorf("Error loading audio: %v", err)
		}
	}

	// Initialize phase converter
	m := phase.NewPhase()
	m.YReverse = true
	m.Window = 640 * 2
	m.Resolut = 2048 * 2
	var ranges []int

	// Determine frequency bands based on sample rate
	switch sampleRate {
	case 11025, 22050, 44100:
		m.NumFreqs = 418 * 2 * 2
		ranges = []int{0, m.NumFreqs}
	case 8000, 16000, 48000:
		m.NumFreqs = 384 * 2 * 2
		ranges = []int{0, m.NumFreqs}
	default:
		return nil, fmt.Errorf("Unsupported sample rate")
	}
	var s = Stuffer(m.NumFreqs)
	audio = zeroStuffing((&s).doZeroStuff(audio, sampleRate, err))

	// Convert to mel spectrogram
	melFrames, err := m.ToPhase(audio)
	if err != nil {
		return nil, fmt.Errorf("Error creating spectrogram: %v", err)
	}

	audio = nil
	const total_rang = 1
	//println(len(melFrames))

	// Find nearest centroids for each frame
	frameSize := m.NumFreqs
	var indices = make([]uint64, total_rang*len(melFrames)/frameSize, total_rang*len(melFrames)/frameSize)
	parallel.ForEach(len(melFrames)/frameSize, 100, func(jj int) {
		j := jj * frameSize
		if j+frameSize > len(melFrames) {
			return
		}
		for rang := 0; rang < total_rang; rang++ {
			if len(centroids) <= rang {
				break
			}
			if len(centroids[rang]) == 0 {
				break
			}

			// Calculate key coordinates for current frame
			var keyCoords []float64
			for i := ranges[rang]; i < ranges[rang+1]; i++ {
				keyCoords = append(keyCoords, melFrames[j+i][0], melFrames[j+i][1], melFrames[j+i][2])
			}

			// Find closest centroid using precomputed valueCoords
			minDist := math.MaxFloat64
			nearestIdx := 0
			for idx, valueCoords := range centroids[rang] {
				if len(valueCoords) == 0 {
					continue
				}
				if len(keyCoords) != len(valueCoords) {
					println(len(keyCoords), len(valueCoords))
					panic("keyCoords don't match valueCoords")
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
			indices[total_rang*jj+rang] = uint64(nearestIdx)
		}
	})

	// Output results
	return json.Marshal(indices)
}
