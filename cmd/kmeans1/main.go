package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans/plotter"
	"github.com/neurlang/classifier/parallel"
	"github.com/neurlang/gomel/mel"
	"github.com/neurlang/kmeans"
	"io/fs"
	"io/ioutil"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

import (
	"math/rand/v2"
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
func progressbar(pos, max uint64) {
	const progressBarWidth = 40
	if max > 0 {
		progress := int(pos * progressBarWidth / max)
		percent := int(pos * 100 / max)
		fmt.Printf("\r[%s%s] %d%% ", progressBar(progress, progressBarWidth), emptySpace(progressBarWidth-progress), percent)
	}
}

// ShuffleSlice shuffles the slice.
func ShuffleSlice[T any](slice []T) {
	for i := len(slice) - 1; i >= 0; i-- {
		j := rand.IntN(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// Remove low-energy frames (silence) before clustering
func isSilence(frame []float64, logEnergyThreshold float64) bool {
	energy := 0.0
	for i := range frame {
		energy += frame[i] * frame[i] // Sum squared magnitudes
	}
	logEnergy := math.Log(energy + 1e-10) // Avoid log(0)
	return logEnergy < logEnergyThreshold
}

// L2-normalize each mel frame before clustering
func normalize(frame []float64) []float64 {
	mean, stddev := 0.0, 0.0
	for i := range frame {
		mean += frame[i]
	}
	mean /= float64(len(frame)) // 192 bins × 2 components
	for i := range frame {
		stddev += (frame[i] - mean) * (frame[i] - mean)
	}
	stddev = math.Sqrt(stddev / float64(len(frame)))
	if stddev < 1e-10 {
		return frame
	}
	for i := range frame {
		frame[i] = (frame[i] - mean) / stddev
	}
	return frame
}

func main() {

	const limit = 99999999
	const kmeanz = 512

	// 1. Load FLAC file and convert to mel spectrogram
	m := mel.NewMel()
	m.MelFmin = 0
	m.MelFmax = 16000
	m.YReverse = true
	m.Window = 1280
	m.NumMels = 192
	m.Resolut = 4096

	var files []string
	modeldir := `../../dict/slovak/`
	dirname := modeldir + "flac/"
	filepath.Walk(dirname, func(path string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".flac") {
			return nil
		}
		if len(files) < limit {
			files = append(files, path)
		}
		return nil
	})
	// 2. Prepare dataset for K-means
	var dataset clusters.Observations
	var dataset_mut sync.Mutex
	var dataset_progress atomic.Uint64
	var dataset_discarded atomic.Uint64

	parallel.ForEach(len(files), 1000, func(i int) {

		// Load audio samples
		audioSamples := mel.LoadFlac(files[i])

		// Convert to mel spectrogram (returns [][2]float64 where each element is [192]float64 for sine and cosine)
		melFrames, err := m.ToMel(audioSamples)
		if err != nil {
			panic(err)
		}

		var discarded uint64
		for j := 0; j < len(melFrames); j += 192 {
			// Convert [192][2]float64 to a flat []float64 (384 dimensions)
			var coords []float64
			for i := 0; i < 192; i++ {
				coords = append(coords, melFrames[j+i][0]) // sine component
				coords = append(coords, melFrames[j+i][1]) // cosine component
			}
			//coords = normalize(coords)
			if isSilence(coords, 8) {
				discarded++
				continue
			}
			dataset_mut.Lock()
			dataset = append(dataset, clusters.Coordinates(coords))
			dataset_mut.Unlock()
		}
		dataset_discarded.Add(discarded)
		dataset_progress.Add(1)
		progressbar(dataset_progress.Load(), uint64(len(files)))
		//println(discarded)
	})

	ShuffleSlice(dataset)

	// 3. Run K-means clustering
	km, err := kmeans.NewWithOptions(0.05, plotter.SimplePlotter{})
	km.Threads = 1000
	if err != nil {
		panic(err)
	}

	clu, err := km.Partition(dataset, kmeanz)
	if err != nil {
		panic(err)
	}

	sort.Slice(clu, func(i, j int) bool {
		return len(clu[i].Observations) > len(clu[j].Observations)
	})

	var file struct {
		Centroids [][]LPFloat
	}

	// 4. Print cluster info
	for i, c := range clu {
		//fmt.Printf("Cluster %d - Centroid (%d dimensions) - frames: %d\n", i, len(c.Center), len(c.Observations))
		file.Centroids = append(file.Centroids, []LPFloat{})
		for _, p := range c.Center {
			file.Centroids[i] = append(file.Centroids[i], LPFloat{Value: p, Digits: 5})
		}
	}
	{
		data, err := json.Marshal(file)
		if err != nil {
			panic(err)
		}
		data = bytes.ReplaceAll(data, []byte(`],`), []byte("],\n"))
		err = ioutil.WriteFile(modeldir+`centroids.json`, data, 0755)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Total clusters: %d\n", len(clu))
	}
	// 5. convert wavs to codewords

	parallel.ForEach(len(files), 1000, func(i int) {

		// Load audio samples
		audioSamples := mel.LoadFlac(files[i])

		// Convert to mel spectrogram (returns [][2]float64 where each element is [192]float64 for sine and cosine)
		melFrames, err := m.ToMel(audioSamples)
		if err != nil {
			panic(err)
		}

		var vec []uint32

		for j := 0; j < len(melFrames); j += 192 {
			// Convert [192][2]float64 to a flat []float64 (384 dimensions)
			var coords []float64
			for i := 0; i < 192; i++ {
				coords = append(coords, melFrames[j+i][0]) // sine component
				coords = append(coords, melFrames[j+i][1]) // cosine component
			}
			var sample = clusters.Coordinates(coords)
			codeword := clu.Nearest(sample)
			vec = append(vec, uint32(codeword))
		}
		fmt.Println(files[i], vec)
	})
}
