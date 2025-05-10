package main

import (
	"fmt"
	"github.com/neurlang/gomel/phase"
)

func centroids_vocode(centroids []uint32, all_centroids [][][]float64, filename string) {

	var samplerate int
	const padframes = 0
	var freqs = padframes
	for _, v := range all_centroids {
		freqs += len(v[0]) / 3
	}
	switch freqs {
	case 384 * 2:
		samplerate = 48000
	case 418 * 2:
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

	fmt.Println(centroids)

	var buf [][3]float64
	for iii, centroid := range centroids {
		ii := iii % 8
		if ii == 0 {
			buf = append(buf, make([][3]float64, padframes, padframes)...)
		}
		var prev0, prev1 float64
		for i, float := range all_centroids[ii][centroid] {
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
