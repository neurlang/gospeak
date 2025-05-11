package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/neurlang/classifier/parallel"
	"github.com/neurlang/clusters"
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
import (
	"flag"
	"os"
	"os/exec"
	"runtime"
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
func progressbar(stage, stages int, pos, max uint64, name string) {
	const progressBarWidth = 40
	if max > 0 {
		progress := int(pos * progressBarWidth / max)
		percent := int(pos * 100 / max)
		fmt.Printf("\r%d/%d [%s%s] %d%% %s | %d%% ",
			stage, stages, progressBar(progress, progressBarWidth),
			emptySpace(progressBarWidth-progress), percent, name,
			calculateOverallProgress(stage, stages, percent))
	}
}
func calculateOverallProgress(currentStage, totalStages, currentStagePercent int) int {
	if totalStages <= 0 {
		return 0
	}
	if currentStage < 1 {
		currentStage = 1
	} else if currentStage > totalStages {
		currentStage = totalStages
	}
	if currentStagePercent < 0 {
		currentStagePercent = 0
	} else if currentStagePercent > 100 {
		currentStagePercent = 100
	}

	// Calculate progress from completed stages (0-10000 for 0.00%-100.00%)
	completedStages := currentStage - 1
	completedProgress := completedStages * 10000 / totalStages

	// Calculate progress from current stage (0-10000 for 0.00%-100.00%)
	currentStageContribution := currentStagePercent * 100 / totalStages

	// Total progress (sum both parts)
	totalProgress := completedProgress + currentStageContribution

	// Convert back from 0-10000 scale to 0-100%
	return totalProgress / 100
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
	stage  int
	stages int

	del float64
	bas float64
	itr uint64
	pro uint64
	cur uint64

	// New fields
	execString   string
	executedbg   bool
	execDetailed bool

	msg string
}

func (p *plotter) Plot(cc clusters.Clusters, iteration int) error {
	if iteration < 0 && p.del != 0 {
		if p.itr == 0 {
			p.pro = uint64(-iteration)
			p.cur = uint64(-iteration)
			p.bas = 2
		} else if p.itr == 1 {
			p.cur = uint64(-iteration)
			if p.cur != 0 {
				p.bas = float64(p.pro) / float64(p.cur)
			}
		} else {
			p.cur = uint64(-iteration)
		}

		target := (int64(len(cc)) * int64(65536*p.del) / 65536)
		// Calculate percentage (integer math) - now float64 log2 progress
		percent := 96 - int64(p.bas*math.Log2(1+float64(int64(p.cur)-target))*96/(p.bas*math.Log2(1+float64(int64(p.pro)-target))))
		if percent < 0 {
			percent = 0
		}
		if percent < int64(p.itr) {
			percent = int64(p.itr)
		}
		progressbar(p.stage, p.stages, uint64(percent), 96, p.msg)
		if p.execDetailed {
			command(p.execString, p.stage, p.stages, false, p.executedbg, byte(percent), p.msg)
		}
	} else {
		progressbar(p.stage, p.stages, p.itr, 96, p.msg)
		if p.execDetailed {
			command(p.execString, p.stage, p.stages, false, p.executedbg, byte(p.itr), p.msg)
		}
	}
	p.itr++
	return nil
}

func which(n int, possibilities []int) (int, int) {
	for i, v := range possibilities {
		if n < v {
			return i, n
		}
		n -= v
	}
	return -1, -1
}

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

func chunksKmeanzMasterkmeanz(filesCount, qualityBoost int) (int, int, int) {
	var chunks = 64
	var kmeanz = 4096 << qualityBoost
	var masterkmeanz = (32768 << qualityBoost) - 1
	for filesCount > 16384 {
		chunks *= 2
		filesCount /= 2
	}
	for filesCount < 8192 && kmeanz > (512<<qualityBoost) {
		kmeanz /= 2
		filesCount *= 2
	}
	for filesCount < 8192 {
		masterkmeanz /= 2
		if chunks > 1 {
			chunks /= 2
		}
		filesCount *= 2
	}
	return chunks, kmeanz, masterkmeanz
}

func command(execString string, chunk, chunks int, wait, debug bool, percentage byte, status string) {
	rescaledPercent := (int(percentage) * 100) / 96
	if rescaledPercent < 0 {
		rescaledPercent = 0
	} else if rescaledPercent > 100 {
		rescaledPercent = 100
	}
	for _, prefix := range []string{"%", ""} {
		if strings.Contains(execString, prefix+`TOTAL_PERCENTAGE`) {
			execString = strings.ReplaceAll(execString, prefix+"TOTAL_PERCENTAGE", fmt.Sprint(calculateOverallProgress(chunk, chunks, int(percentage))))
		}
		if strings.Contains(execString, prefix+`STATUS`) {
			execString = strings.ReplaceAll(execString, prefix+"STATUS", status)
		}
		if strings.Contains(execString, prefix+`PERCENTAGE`) {
			execString = strings.ReplaceAll(execString, prefix+"PERCENTAGE", fmt.Sprint(rescaledPercent))
		}
		if strings.Contains(execString, prefix+`STAGE_NUMBER`) {
			execString = strings.ReplaceAll(execString, prefix+`STAGE_NUMBER`, fmt.Sprint(chunk))
		}
		if strings.Contains(execString, prefix+`TOTAL_STAGES`) {
			execString = strings.ReplaceAll(execString, prefix+`TOTAL_STAGES`, fmt.Sprint(chunks))
		}
	}
	var execVect = strings.Fields(execString)
	fmt.Println("\nRunning:", execVect)
	if len(execVect) != 0 {
		cmd := exec.Command(execVect[0], execVect[1:]...)
		if debug {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if !wait {
			cmd.Start()
		} else {
			cmd.Run()
		}
	}
}

func main() {
	srcDir := flag.String("srcdir", "", "path to directory containing wav or flac files to generate codec for")
	dstDir := flag.String("dstdir", "", "path to directory to write generated codec to")
	execute := flag.String("execute", "", "a command to run after each phase gets solved")
	executedbg := flag.Bool("executedbg", false, "debug execute command, attaches stdout/stderr")
	execDetailed := flag.Bool("exec-detailed", false, "execute detailed command after each progress update")
	threads := flag.Int("threads", runtime.NumCPU(), "number of threads (default NumCPU() at startup)")
	quality := flag.Int("quality", 0, "quality increase factor (small integer, default 0)")
	flag.Parse()
	if srcDir == nil || *srcDir == "" {
		println("srcdir is mandatory")
		return
	}
	if dstDir == nil || *dstDir == "" {
		println("dstdir is mandatory")
		return
	}

	// Check if dstDir exists and is a directory
	dstInfo, err := os.Stat(*dstDir)
	if os.IsNotExist(err) {
		fmt.Printf("Error: destination directory '%s' does not exist\n", *dstDir)
		return
	}
	if !dstInfo.IsDir() {
		fmt.Printf("Error: '%s' is not a directory\n", *dstDir)
		return
	}

	const limit = 9999999999999999

	// 1. Load FLAC file and convert to phase spectrogram
	m := phase.NewPhase()
	m.YReverse = true
	m.Window = 640 * 2
	m.NumFreqs = 0 // unknown
	m.Resolut = 2048 * 2
	var ranges []int

	var filesFlac, filesWav []string
	filepath.Walk(*srcDir, func(path string, info fs.FileInfo, err error) error {
		var isFlac = strings.HasSuffix(path, ".flac")
		var isWav = strings.HasSuffix(path, ".wav")
		if !isFlac && !isWav {
			return nil
		}
		if len(filesFlac)+len(filesWav) < limit {
			if m.NumFreqs == 0 {
				var audio []float64
				var sr uint32
				var err error
				if isFlac {
					audio, sr, err = phase.LoadFlacSampleRate(path)
				}
				if isWav {
					audio, sr, err = phase.LoadWavSampleRate(path)
				}
				if err != nil {
					fmt.Println(err.Error())
					return nil
				}
				if len(audio) == 0 || sr == 0 {
					return nil
				}
				println("Sample rate:", sr)
				switch sr {
				case 8000, 16000, 48000:
					println("Codec native sample rate: 48000")
					m.NumFreqs = 384 * 2
					ranges = []int{0, 38, 88, 134, 184, 234, 367, 501, 384 * 2}
				case 11025, 22050, 44100:
					println("Codec native sample rate: 44100")
					m.NumFreqs = 418 * 2
					ranges = []int{0, 41, 95, 145, 200, 254, 400, 545, 418 * 2}
				}
			}
			if isFlac {
				filesFlac = append(filesFlac, path)
			}
			if isWav {
				filesWav = append(filesWav, path)
			}
		}
		return nil
	})

	if m.NumFreqs == 0 {
		panic("couldn't figure out project sample rate - no relevant files found?")
	}
	var s Stuffer = Stuffer(m.NumFreqs)

	var chunks, kmeanz, masterkmeanz = chunksKmeanzMasterkmeanz(len(filesFlac)+len(filesWav), *quality)
	println("Files:", len(filesFlac)+len(filesWav))
	println("Chunks:", chunks)
	println("Kmeans:", kmeanz)
	println("Master Kmeans:", masterkmeanz)

	execString := ""
	if execute != nil {
		execString = *execute
	}
	execdbg := false
	if executedbg != nil {
		execdbg = *executedbg
	}
	execDet := false
	if execDetailed != nil {
		execDet = *execDetailed
	}

	var fileMutex sync.Mutex
	var file struct {
		minDists  []float64
		Centroids [][][]LPFloat
	}

	for rang := 0; rang < 8; rang++ {
		file.Centroids = append(file.Centroids, nil)
		file.minDists = nil
		// dataset for master problem
		var master clusters.Observations
		for chunk := 0; chunk < chunks; chunk++ {
			fmt.Println()

			// 2. Prepare dataset for K-means
			var dataset clusters.Observations
			var dataset_mut sync.Mutex
			var dataset_progress atomic.Uint64
			//var dataset_discarded atomic.Uint64
			var dataset_total atomic.Uint64

			parallel.ForEach(len(filesFlac)+len(filesWav), *threads, func(i int) {

				if i%chunks != chunk {
					return
				}

				var audioSamples []float64
				// Load audio samples
				switch index, pos := which(i, []int{len(filesFlac), len(filesWav)}); index {
				case 0:
					audioSamples = zeroStuffing(s.doZeroStuff(phase.LoadFlacSampleRate(filesFlac[pos])))
				case 1:
					audioSamples = zeroStuffing(s.doZeroStuff(phase.LoadWavSampleRate(filesWav[pos])))
				default:
					return
				}

				// Convert to mel spectrogram (returns [][3]float64 where each element is [m.NumFreqs]float64 for sine and cosine and real)
				melFrames, err := m.ToPhase(audioSamples)
				if err != nil {
					panic(err)
				}

				//var discarded uint64
				for j := 0; j < len(melFrames); j += m.NumFreqs {
					var keycoords []float64
					for i := ranges[rang]; i < ranges[rang+1]; i++ {
						keycoords = append(keycoords, (math.Sqrt(math.Pow(verifyFloat(math.Exp2(melFrames[j+i][1])), 2) + math.Pow(verifyFloat(math.Exp2(melFrames[j+i][2])), 2))))
						keycoords = append(keycoords, (math.Sqrt(math.Pow(verifyFloat(math.Exp2(melFrames[j+i][0])), 2) + math.Pow(verifyFloat(math.Exp2(melFrames[j+i][1])), 2))))

					}
					var coords = clusters.Coordinates(keycoords)
					dataset_mut.Lock()
					dataset = append(dataset, coords)
					dataset_mut.Unlock()
				}
				//dataset_discarded.Add(discarded)
				dataset_total.Add(uint64(len(melFrames)) / uint64(m.NumFreqs))
				dataset_progress.Add(uint64(chunks))
				if dataset_progress.Load() > uint64(len(filesFlac)+len(filesWav)) {
					progressbar(2*chunk+1+(2*chunks+2)*rang, (2*chunks+2)*8, 1, 1, "loading")
				} else {
					progressbar(2*chunk+1+(2*chunks+2)*rang, (2*chunks+2)*8, dataset_progress.Load(), uint64(len(filesFlac)+len(filesWav)), "loading")
				}
				//println(discarded)
			})
			progressbar(2*chunk+1+(2*chunks+2)*rang, (2*chunks+2)*8, 1, 1, "loading")
			if execute != nil && *execute != "" {
				command(*execute, 2*chunk+1+(2*chunks+2)*rang, (2*chunks+2)*8, false, executedbg != nil && *executedbg, 96, "loading")
			}

			fmt.Println()

			//println("Silence discarded: ", dataset_discarded.Load() * 100 / dataset_total.Load() , "%")

			if len(dataset) < kmeanz {
				ShuffleSlice(dataset)
				for i := 0; len(dataset) < kmeanz; i++ {
					dataset = append(dataset, dataset[i])
				}
			}

			ShuffleSlice(dataset)

			progressbar(2*chunk+2+(2*chunks+2)*rang, (2*chunks+2)*8, 0, 1, "kmeans")

			plotter := &plotter{
				stage:        2*chunk + 2 + (2*chunks+2)*rang,
				stages:       (2*chunks + 2) * 8,
				del:          0.05,
				execString:   execString,
				executedbg:   execdbg,
				execDetailed: execDet,
				msg:          "kmeans",
			}

			// 3. Run K-means clustering
			km, err := kmeans.NewWithOptions(0.05, plotter)
			km.Threads = *threads * *threads
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
			progressbar(2*chunk+2+(2*chunks+2)*rang, (2*chunks+2)*8, 1, 1, "kmeans")
			if execute != nil && *execute != "" {
				command(*execute, 2*chunk+2+(2*chunks+2)*rang, (2*chunks+2)*8, false, executedbg != nil && *executedbg, 96, "kmeans")
			}
		}

		ShuffleSlice(master)
		progressbar(2*chunks+(2*chunks+2)*rang, (2*chunks+2)*8, 1, 1, "kmeans")
		fmt.Println()
		progressbar(2*chunks+1+(2*chunks+2)*rang, (2*chunks+2)*8, 0, 1, "final")

		plotter := &plotter{
			stage:        2*chunks + 1 + (2*chunks+2)*rang,
			stages:       (2*chunks + 2) * 8,
			del:          0.05,
			execString:   execString,
			executedbg:   execdbg,
			execDetailed: execDet,
			msg:          "final",
		}

		// 4. Run master K-means clustering
		km, err := kmeans.NewWithOptions(0.05, plotter)
		km.Threads = *threads * *threads
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

		// 5. Init cluster info
		for range clu {
			//fmt.Printf("Cluster %d - Centroid (%d dimensions) - frames: %d\n", i, len(c.Center), len(c.Observations))
			file.Centroids[rang] = append(file.Centroids[rang], []LPFloat{})
			file.minDists = append(file.minDists, math.MaxFloat64)
		}

		// 6. convert wavs to codewords
		var final_dump_progress atomic.Uint64
		progressbar(2*chunks+1+(2*chunks+2)*rang, (2*chunks+2)*8, 1, 1, "final")
		if execute != nil && *execute != "" {
			command(*execute, 2*chunks+1, (2*chunks+2)*8, false, executedbg != nil && *executedbg, 96, "final")
		}
		fmt.Println()
		progressbar(2*chunks+2+(2*chunks+2)*rang, (2*chunks+2)*8, 0, 1, "dumping")

		parallel.ForEach(len(filesFlac)+len(filesWav), *threads, func(i int) {

			var audioSamples []float64
			var fileName string
			// Load audio samples
			switch index, pos := which(i, []int{len(filesFlac), len(filesWav)}); index {
			case 0:
				fileName = filesFlac[pos]
				audioSamples = zeroStuffing(s.doZeroStuff(phase.LoadFlacSampleRate(fileName)))
			case 1:
				fileName = filesWav[pos]
				audioSamples = zeroStuffing(s.doZeroStuff(phase.LoadWavSampleRate(fileName)))
			default:
				return
			}

			// Convert to mel spectrogram (returns [][3]float64 where each element is [m.NumFreqs]float64 for sine and cosine and real)
			melFrames, err := m.ToPhase(audioSamples)
			if err != nil {
				panic(err)
			}

			for j := 0; j < len(melFrames); j += m.NumFreqs {
				// Convert [m.NumFreqs][3]float64 to a flat []float64 (1152 dimensions)
				var coords []LPFloat
				for i := ranges[rang]; i < ranges[rang+1]; i++ {
					coords = append(coords, LPFloat{Value: melFrames[j+i][0], Digits: 3}) // first component
					coords = append(coords, LPFloat{Value: melFrames[j+i][1], Digits: 3}) // second component
					coords = append(coords, LPFloat{Value: melFrames[j+i][2], Digits: 3}) // third component
				}
				var keycoords []float64
				for i := ranges[rang]; i < ranges[rang+1]; i++ {
					keycoords = append(keycoords, (math.Sqrt(math.Pow(verifyFloat(math.Exp2(melFrames[j+i][1])), 2) + math.Pow(verifyFloat(math.Exp2(melFrames[j+i][2])), 2))))
					keycoords = append(keycoords, (math.Sqrt(math.Pow(verifyFloat(math.Exp2(melFrames[j+i][0])), 2) + math.Pow(verifyFloat(math.Exp2(melFrames[j+i][1])), 2))))
				}
				var sample = clusters.Coordinates(keycoords)
				for codeword := range clu {
					dist := sample.Distance(clu[codeword].Center)

					fileMutex.Lock()
					// update solution's nearest Centroids
					if dist < file.minDists[codeword] {
						file.minDists[codeword] = dist
						file.Centroids[rang][codeword] = coords
					}
					fileMutex.Unlock()
				}
			}
			progressbar(2*chunks+2+(2*chunks+2)*rang, (2*chunks+2)*8, final_dump_progress.Load(), uint64(len(filesFlac)+len(filesWav)), "dumping")
			final_dump_progress.Add(1)
		})
		progressbar(2*chunks+2+(2*chunks+2)*rang, (2*chunks+2)*8, 1, 1, "dumping")
		fmt.Println()
		// Output to file
		{
			data, err := json.Marshal(file)
			if err != nil {
				panic(err)
			}
			data = bytes.ReplaceAll(data, []byte(`],`), []byte("],\n"))
			err = ioutil.WriteFile(*dstDir+string(os.PathSeparator)+`centroids`+fmt.Sprint(rang)+`.json`, data, 0755)
			if err != nil {
				panic(err)
			}
		}
	}
	fmt.Println("Codec solved: true")
	if execute != nil && *execute != "" {
		command(*execute, (2*chunks+2)*8, (2*chunks+2)*8, true, executedbg != nil && *executedbg, 96, "completed")
	}
}
