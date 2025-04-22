# gospeak
A Golang Text to Speech System

## Features

- Already much faster than realtime synthesis on CPU (real time factor of 0.055, TODO: one-time startup cost 15 seconds due to slow model parsing/loading)
- Goal of small model (currently the model is ~226 MB per voice and still growing, much larger than the original goal of ~3MB per voice)
- Already speech of higher speech quality than comparable footprint competitor systems (espeak, etc...)
- TODO: Generalize well to out-of-training set words (currently it generates well mostly just the words which were seen during training)

## Speech samples

- [gospeak TTS-Synthesized Speech Samples](https://github.com/neurlang/gospeak/wiki/TTS-Synthesized-Speech-Samples)
