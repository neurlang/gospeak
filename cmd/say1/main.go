package main

import "time"
import "github.com/neurlang/classifier/datasets/speak"
import "github.com/neurlang/classifier/layer/sum"
import "github.com/neurlang/classifier/layer/sochastic"
import "github.com/neurlang/classifier/layer/crossattention"
import "github.com/neurlang/classifier/net/feedforward"
import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/neurlang/gomel/phase"
	"io/ioutil"
	"os"
	"strconv"
)

func bigram_gen_next(bigrams map[string]map[string]int, current string) (out []uint32) {
	// Check if current number exists in the model
	nextOptions, exists := bigrams[current]
	if !exists {
		fmt.Printf("No data for number '%s' in the bigram model\n", current)
		return nil
	}

	// Load all next possibilities
	for num := range nextOptions {
		n, _ := strconv.Atoi(num)
		out = append(out, uint32(n))
	}

	return
}
func unpack_tokens_into_mels_centroids(n []uint32) (ret []uint32) {
	const mask = ((1 << 15) - 1)
	for _, num := range n {
		ret = append(ret, ((num >> 15) & mask), ((num >> 0) & mask))
	}
	return
}

func centroids_unpad(centroids []uint32) []uint32 {
	if len(centroids) > 0 && centroids[len(centroids)-1] == 0 {
		centroids = centroids[:len(centroids)-1]
	}
	if len(centroids) > 0 && centroids[len(centroids)-1] == 0 {
		centroids = centroids[:len(centroids)-1]
	}
	for i := range centroids {
		centroids[i]--
	}
	return centroids
}

func centroids_vocode(centroids []uint32, all_centroids [][]float64, filename string) {

	var samplerate int
	var freqs = len(all_centroids[0])/3
	switch freqs {
	case 384 * 2:
		samplerate = 48000
	case 418 * 2:
		samplerate = 44000
	default:
		println("freqs:", freqs)
		panic("unsupported sample rate")
	}

	m := phase.NewPhase()
	m.YReverse = true
	m.Window = 640 * 2
	m.NumFreqs = freqs
	m.Resolut = 2048 * 2
	m.VolumeBoost = 4

	fmt.Println(centroids)

	var buf [][3]float64
	for _, centroid := range centroids {
		var prev0, prev1 float64
		for i, float := range all_centroids[centroid] {
			if i%3 == 0 {
				prev0 = float
			} else if i%3 == 1 {
				prev1 = float
			} else {
				buf = append(buf, [3]float64{prev0, prev1, float})
			}
		}
	}

	//fmt.Println(buf)

	speech, err := m.FromPhase(buf)
	if err != nil {
		panic(err)
	}
	phase.SaveWav(filename, speech, samplerate)
}

func predict_acoustic_codewords(line string, fanout1 int, bigrams map[string]map[string]int, net feedforward.FeedforwardNetwork) (ret []uint32) {
	var sample speak.Sample2
	sample.Source = []rune(line)
	sample.Target = []uint32{0}
	sample.Dim = fanout1
	for i := 0; i < 1; i++ {
		var nexts []uint32
		var higerst uint32
		if len(sample.Target) > 1 {
			nexts = bigram_gen_next(bigrams, fmt.Sprint(sample.Target[len(sample.Target)-2]))
		} else {
			nexts = bigram_gen_next(bigrams, string(sample.Source[0]))
		}
		if len(nexts) == 0 {
			println("no next, ending")
			return
		} else if len(nexts) == 1 {
			higerst = nexts[0]
		} else {

			for _, next := range nexts {

				const mask = ((1 << 15) - 1)
				println("testing", ((next>>15)&mask)-1, ((next>>0)&mask)-1)

				sample.SetOutput(uint32(next))
				sample.Target[len(sample.Target)-1] = uint32(next)
				var io = &sample
				//fmt.Println(io)

				var predicted = net.Infer2(io) & 1
				if predicted == 1 {
					println("predicted == 1")
					if next > higerst {
						higerst = next
					}
				}
			}
		}

		sample.Target[len(sample.Target)-1] = higerst

		i = -1
		if len(ret) < len(sample.Target) {
			ret = sample.Target
		}
		sample.Target = append(sample.Target, 0)
		fmt.Println(sample.Target)

		if len(sample.Target) > 6*len(sample.Source) {
			println("target sequence is long, breaking")
			break
		}
	}

	if len(ret) == 0 {
		println("no codewords")
		return nil
	}

	return
}

func main() {
	start := time.Now()
	modeldir := `../../dict/slovak/`

	var file struct {
		Centroids [][]float64
	}
	{
		data, err := ioutil.ReadFile(modeldir + `centroids.json`)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(data, &file)
		if err != nil {
			panic(err)
		}
	}
	var bigrams map[string]map[string]int
	{
		// Load the bigram model
		content, err := ioutil.ReadFile(modeldir + `bigram.json`)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(content, &bigrams)
		if err != nil {
			panic(err)
		}
	}

	const fanout1 = 16
	const fanout2 = 4
	const fanout3 = 4

	var net feedforward.FeedforwardNetwork
	net.NewLayer(fanout1*fanout2, 0)
	for i := 0; i < fanout3; i++ {
		net.NewCombiner(crossattention.MustNew(fanout1, fanout2))
		net.NewLayerPI(fanout1*fanout2, 0, 0)
		net.NewCombiner(sochastic.MustNew(fanout1*fanout2, 8*byte(i), uint32(i)))
		net.NewLayerPI(fanout1*fanout2, 0, 0)
	}
	net.NewCombiner(sochastic.MustNew(fanout1*fanout2, 32, fanout3))
	net.NewLayer(fanout1*fanout2, 0)
	net.NewCombiner(sum.MustNew([]uint{fanout1 * fanout2}, 0))
	net.NewLayer(1, 0)

	err := net.ReadZlibWeightsFromFile(modeldir + `output.99.json.t.lzw`)
	if err != nil {
		panic(err)
	}
	// Code to measure
	duration := time.Since(start)

	// Formatted string, such as "2h3m0.5s" or "4.503μs"
	fmt.Println(duration)

	//centroids_vocode([]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0}, file.Centroids, "000.wav")
	centroids_vocode([]uint32{
		5870, 17390, 5089, 2148, 7879, 16094, 2754, 8719, 11767, 2723, 10786, 3223, 2593, 1248, 363, 63, 47, 15849, 14221, 31292},
		file.Centroids, "robot.wav")

	// Create a new scanner to read the file line by line
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 {
			continue
		}

		fmt.Println([]rune(line))

		start := time.Now()
		var centroids = unpack_tokens_into_mels_centroids(predict_acoustic_codewords(line, fanout1, bigrams, net))

		fmt.Println(centroids)

		if len(centroids) == 0 {
			continue
		}

		centroids = centroids_unpad(centroids)
		if len(centroids) == 0 {
			continue
		}

		centroids_vocode(centroids, file.Centroids, "test.wav")

		// Code to measure
		duration := time.Since(start)

		// Formatted string, such as "2h3m0.5s" or "4.503μs"
		fmt.Println(duration)
	}
}
