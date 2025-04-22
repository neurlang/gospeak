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
	"github.com/neurlang/gomel/mel"
	"io/ioutil"
	"os"
	"strconv"
)
import (
	rand "math/rand/v2"
)

func bigram_gen_next(bigrams map[string]map[string]int, current string, used map[int]struct{}) int {
	// Check if current number exists in the model
	nextOptions, exists := bigrams[current]
	if !exists {
		//fmt.Printf("No data for number '%s' in the bigram model\n", current)
		return -1
	}

	// Calculate total occurrences
	total := 0
	for num, count := range nextOptions {
		n, _ := strconv.Atoi(num)
		if _, ok := used[int(n)]; ok {
			continue
		}
		total += count
	}

	if total == 0 {
		return -1
	}

	// Prepare for probabilistic selection
	randomValue := rand.IntN(total)

	// Select the next number based on probability
	cumulative := 0
	var selected string
	for num, count := range nextOptions {
		n, _ := strconv.Atoi(num)
		if _, ok := used[int(n)]; ok {
			continue
		}
		cumulative += count
		if randomValue < cumulative {
			selected = num
			break
		}
	}

	//fmt.Printf("Current number: %s\n", current)
	//fmt.Printf("Possible next numbers and their probabilities:\n")
	//for num, count := range nextOptions {
	//	probability := float64(count) / float64(total) * 100
	//	fmt.Printf("- %s: %.2f%%\n", num, probability)
	//}
	//fmt.Printf("\nSelected next number: %s\n", selected)
	n, _ := strconv.Atoi(selected)
	return int(n)
}
func unpack_tokens_into_mels_centroids(n []uint32) (ret []uint32) {
	const mask = ((1 << 10) - 1)
	for _, num := range n {
		ret = append(ret, ((num >> 20) & mask), ((num >> 10) & mask), ((num >> 0) & mask))
	}
	return
}

func centroids_unpad(centroids []uint32) []uint32 {
	if len(centroids) > 0 && centroids[len(centroids)-1] == 0 {
		centroids = centroids[:len(centroids)-1]
	}
	return centroids
}

func centroids_vocode(centroids []uint32, all_centroids [][]float64, filename string) {
	m := mel.NewMel()
	m.MelFmin = 0
	m.MelFmax = 16000
	m.YReverse = true
	m.Window = 1280
	m.NumMels = 192
	m.Resolut = 4096
	m.GriffinLimIterations = 1

	fmt.Println(centroids)

	var buf [][2]float64
	for _, centroid := range centroids {
		var prev float64
		for i, float := range all_centroids[centroid] {
			if i&1 == 0 {
				prev = float
			} else {
				buf = append(buf, [2]float64{prev, float})
			}
		}
	}

	//fmt.Println(buf)

	speech, err := m.FromMel(buf)
	if err != nil {
		panic(err)
	}
	mel.SaveWav(filename, speech, 48000)
}

func predict_acoustic_codewords(line string, fanout1 int, bigrams map[string]map[string]int, net feedforward.FeedforwardNetwork) (ret []uint32) {
	var sample speak.Sample
	sample.Source = []rune(line)
	sample.Target = []uint32{0}
	sample.Dim = fanout1
	var used = make(map[int]struct{})
	for i := 0; i < 1024; i++ {
		var next int
		if len(sample.Target) > 1 {
			next = bigram_gen_next(bigrams, fmt.Sprint(sample.Target[len(sample.Target)-2]), used)
		} else {
			next = bigram_gen_next(bigrams, string(sample.Source[0]), used)
		}
		if next == -1 {
			continue
		}
		if _, was_used := used[next]; was_used {
			continue
		}
		const mask = ((1 << 10) - 1)
		println("testing", ((next >> 20) & mask), ((next >> 10) & mask), ((next >> 0) & mask))

		sample.SetOutput(uint32(next))
		sample.Target[len(sample.Target)-1] = uint32(next)
		var io = &sample
		//fmt.Println(io)

		var predicted = net.Infer2(io) & 1
		if predicted == 1 {

			println("predicted == 1")

			i = -1
			if len(ret) < len(sample.Target) {
				ret = sample.Target
			}
			sample.Target = append(sample.Target, 0)
			used = make(map[int]struct{})
			fmt.Println(sample.Target)

			continue
		} else {
			println("predicted == 0")
			used[next] = struct{}{}
			continue
		}
		if len(sample.Target) > 2*len(sample.Source) {
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

	err := net.ReadZlibWeightsFromFile("/home/m/go/src/example.com/repo.git/classifier/cmd/train_speak/output.99.json.t.lzw")
	// Code to measure
	duration := time.Since(start)

	// Formatted string, such as "2h3m0.5s" or "4.503μs"
	fmt.Println(duration)

	if err != nil {
		panic(err)
	}
	//centroids_vocode([]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0}, file.Centroids, "000.wav")
	//centroids_vocode([]uint32{447, 447, 364, 145, 184, 99, 309, 97, 74, 49, 403, 430, 3, 296, 87, 87, 281, 177, 200, 142, 156, 206, 206, 306, 57, 124, 20, 295, 295, 295, 295, 71, 131, 131, 131}, file.Centroids, "amsterdam.wav")

	// Create a new scanner to read the file line by line
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()

		fmt.Println([]rune(line))

		start := time.Now()
		var centroids = unpack_tokens_into_mels_centroids(predict_acoustic_codewords(line, fanout1, bigrams, net))

		fmt.Println(centroids)

		if len(centroids) == 0 {
			continue
		}

		centroids = centroids_unpad(centroids_unpad(centroids))
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
