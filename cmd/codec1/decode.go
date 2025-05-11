package main

import (
	//"fmt"
	"github.com/neurlang/gomel/phase"
)

func centroids_vocode(centroids []uint32, all_centroids [][][]float64, filename string) {

	var samplerate int
	var freqs int
	switch len(all_centroids[0][0]) / 3 {
	case 38:
		freqs = 384 * 2
		samplerate = 48000
	case 41:
		freqs = 418 * 2
		samplerate = 44100
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

	//fmt.Println(centroids)

	var buf [][3]float64
	var done int
	for iii, centroid := range centroids {
		ii := iii % 8
		if ii == 0 {
			buf = append(buf, make([][3]float64, freqs-done, freqs-done)...)
			done = 0
		}
		if ii >= len(all_centroids) {
			continue
		}
		var prev0, prev1 float64
		for i, float := range all_centroids[ii][centroid] {
			if i%3 == 0 {
				prev0 = float
			} else if i%3 == 1 {
				prev1 = float
			} else {
				buf = append(buf, [3]float64{prev0, prev1, float})
				done++
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
