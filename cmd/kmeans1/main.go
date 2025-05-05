package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/muesli/clusters"
	"github.com/neurlang/classifier/parallel"
	"github.com/neurlang/gomel/phase"
	"github.com/neurlang/kmeans"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	//"math/cmplx"
	"math"
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

// Remove low-energy frames (silence) before clustering
func isSilence(frame []float64, logEnergyThreshold float64) bool {
	energy := 0.0
	for i := range frame {
		energy += frame[i] * frame[i] // Sum squared magnitudes
	}
	return energy < logEnergyThreshold
}

// ShuffleSlice shuffles the slice. Now copied straight from the manual.
// Note: Tested that the randomness source is not deterministic (different shufflings across program runs).
func ShuffleSlice[T any](slice []T) {
	rand.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

type plotter struct {
	del float64
	itr uint64
	pro uint64
	cur uint64
}

func (p *plotter) Plot(cc clusters.Clusters, iteration int) error {
	if iteration < 0 && p.del != 0 {
		if p.itr == 0 {
			p.pro = uint64(-iteration)
			p.cur = uint64(-iteration)
		} else {
			p.cur = uint64(-iteration)
		}

		target := (65536*int64(len(cc)) * int64(65536*p.del))
		// Calculate percentage (integer math) - now float64 log2 progress
		percent := 100 - int64( math.Log2(1+float64(int64(p.cur)-target)) * 100 / math.Log2(1+float64(int64(p.pro)-target)))
		if percent < 0 {
			percent = 0
		}
		if percent < int64(p.itr) {
			percent = int64(p.itr)
		}
		progressbar(uint64(percent), 96)
	} else {
		progressbar(p.itr, 96)
	}
	p.itr++
	return nil
}

func main() {
	const chunks = 64
	const limit = 9999999999999999
	const kmeanz = 4096 // 32767
	const masterkmeanz = 32767
	const freqs = 384 * 2

	// 1. Load FLAC file and convert to phase spectrogram
	m := phase.NewPhase()
	m.YReverse = true
	m.Window = 640 * 2
	m.NumFreqs = freqs
	m.Resolut = 2048 * 2

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

	// dataset for master problem
	var master clusters.Observations

	for chunk := 0; chunk < chunks; chunk++ {

		// 2. Prepare dataset for K-means
		var dataset clusters.Observations
		var dataset_mut sync.Mutex
		var dataset_progress atomic.Uint64
		//var dataset_discarded atomic.Uint64
		var dataset_total atomic.Uint64

		parallel.ForEach(len(files), 1000, func(i int) {

			if i%chunks != chunk {
				return
			}

			// Load audio samples
			audioSamples := phase.LoadFlac(files[i])

			// Convert to mel spectrogram (returns [][3]float64 where each element is [freqs]float64 for sine and cosine and real)
			melFrames, err := m.ToPhase(audioSamples)
			if err != nil {
				panic(err)
			}

			//var discarded uint64
			for j := 0; j < len(melFrames); j += freqs {
				// Convert [freqs][3]float64 to a flat []float64 (1152 dimensions)
				/*
					var coords []float64
					for i := 0; i < freqs; i++ {
						coords = append(coords, melFrames[j+i][0]) // first component
						coords = append(coords, melFrames[j+i][1]) // second component
						coords = append(coords, melFrames[j+i][2]) // third component
					}
				*/
				var keycoords []float64
				for i := 0; i < freqs; i++ {
					//keycoords = append(keycoords, math.Log(math.Pow(melFrames[j+i][0], 2) + math.Pow(melFrames[j+i][1], 2)))
					keycoords = append(keycoords, (math.Sqrt(math.Pow(math.Exp(melFrames[j+i][1]), 2) + math.Pow(math.Exp(melFrames[j+i][2]), 2))))
					keycoords = append(keycoords, (math.Sqrt(math.Pow(math.Exp(melFrames[j+i][0]), 2) + math.Pow(math.Exp(melFrames[j+i][1]), 2))))
					//keycoords = append(keycoords, math.Sqrt(math.Pow(math.Exp(melFrames[j+i][0]),2)+math.Pow(math.Exp(melFrames[j+i][2]),2)))
					//keycoords = append(keycoords, math.Log(math.Exp(melFrames[j+i][0]) + math.Exp(melFrames[j+i][2])))

					/*
						keycoords = append(keycoords, cmplx.Abs(complex(math.Exp(melFrames[j+i][0]), math.Exp(melFrames[j+i][1]))))
						keycoords = append(keycoords, cmplx.Abs(complex(math.Exp(melFrames[j+i][1]), math.Exp(melFrames[j+i][2]))))
						keycoords = append(keycoords, cmplx.Abs(complex(math.Exp(melFrames[j+i][0]), math.Exp(melFrames[j+i][2]))))
					*/
				}
				//coords = normalize(coords)
				/*
					if isSilence(keycoords, 100) {
						discarded++
						continue
					}
				*/
				var coords = clusters.Coordinates(keycoords)
				dataset_mut.Lock()
				dataset = append(dataset, coords)
				dataset_mut.Unlock()
			}
			//dataset_discarded.Add(discarded)
			dataset_total.Add(uint64(len(melFrames)) / freqs)
			dataset_progress.Add(uint64(chunks))
			progressbar(dataset_progress.Load(), uint64(len(files)))
			//println(discarded)
		})

		fmt.Println()

		//println("Silence discarded: ", dataset_discarded.Load() * 100 / dataset_total.Load() , "%")

		ShuffleSlice(dataset)

		progressbar(0, 1)

		plotter := &plotter{del: 0.05}

		// 3. Run K-means clustering
		km, err := kmeans.NewWithOptions(0.05, plotter)
		km.Threads = 1000
		if err != nil {
			panic(err)
		}

		clu, err := km.Partition(dataset, kmeanz)
		if err != nil {
			panic(err)
		}

		for _, c := range clu {
			master = append(master, c.Center)
		}
	}

	ShuffleSlice(master)

	fmt.Println()
	progressbar(0, 1)

	plotter := &plotter{del: 0.05}

	// 4. Run master K-means clustering
	km, err := kmeans.NewWithOptions(0.05, plotter)
	km.Threads = 1000
	if err != nil {
		panic(err)
	}
	clu, err := km.Partition(master, masterkmeanz)
	if err != nil {
		panic(err)
	}

	sort.Slice(clu, func(i, j int) bool {
		return len(clu[i].Observations) > len(clu[j].Observations)
	})

	var fileMutex sync.Mutex
	var file struct {
		minDists  []float64
		Centroids [][]LPFloat
	}
	// 5. Init cluster info
	for range clu {
		//fmt.Printf("Cluster %d - Centroid (%d dimensions) - frames: %d\n", i, len(c.Center), len(c.Observations))
		file.Centroids = append(file.Centroids, []LPFloat{})
		file.minDists = append(file.minDists, math.MaxFloat64)
	}

	// 6. convert wavs to codewords

	parallel.ForEach(len(files), 1000, func(i int) {

		// Load audio samples
		audioSamples := phase.LoadFlac(files[i])

		// Convert to mel spectrogram (returns [][3]float64 where each element is [freqs]float64 for sine and cosine and real)
		melFrames, err := m.ToPhase(audioSamples)
		if err != nil {
			panic(err)
		}

		var vec []uint32

		for j := 0; j < len(melFrames); j += freqs {
			// Convert [freqs][3]float64 to a flat []float64 (1152 dimensions)
			var coords []LPFloat
			for i := 0; i < freqs; i++ {
				coords = append(coords, LPFloat{Value: melFrames[j+i][0], Digits: 3}) // first component
				coords = append(coords, LPFloat{Value: melFrames[j+i][1], Digits: 3}) // second component
				coords = append(coords, LPFloat{Value: melFrames[j+i][2], Digits: 3}) // third component
			}
			var keycoords []float64
			for i := 0; i < freqs; i++ {
				keycoords = append(keycoords, (math.Sqrt(math.Pow(math.Exp(melFrames[j+i][1]), 2) + math.Pow(math.Exp(melFrames[j+i][2]), 2))))
				keycoords = append(keycoords, (math.Sqrt(math.Pow(math.Exp(melFrames[j+i][0]), 2) + math.Pow(math.Exp(melFrames[j+i][1]), 2))))
				//keycoords = append(keycoords, math.Sqrt(math.Pow(math.Exp(melFrames[j+i][0]),2)+math.Pow(math.Exp(melFrames[j+i][2]),2)))
				//keycoords = append(keycoords, math.Log(math.Exp(melFrames[j+i][0]) + math.Exp(melFrames[j+i][2])))
				/*
					keycoords = append(keycoords, cmplx.Abs(complex(math.Exp(melFrames[j+i][0]), math.Exp(melFrames[j+i][1]))))
					keycoords = append(keycoords, cmplx.Abs(complex(math.Exp(melFrames[j+i][1]), math.Exp(melFrames[j+i][2]))))
					keycoords = append(keycoords, cmplx.Abs(complex(math.Exp(melFrames[j+i][0]), math.Exp(melFrames[j+i][2]))))
				*/
			}
			var sample = clusters.Coordinates(keycoords)
			codeword := clu.Nearest(sample)
			dist := sample.Distance(clu[codeword].Center)
			vec = append(vec, uint32(codeword))

			fileMutex.Lock()
			// update solution's nearest Centroids
			if dist < file.minDists[codeword] {
				file.minDists[codeword] = dist
				file.Centroids[codeword] = coords
			}
			fileMutex.Unlock()
		}
		fmt.Println(files[i], vec)
	})
	// Output to file
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

}
