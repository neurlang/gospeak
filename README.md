# gospeak
A Golang Text to Speech System

## Features

- Goal of much faster than realtime synthesis on CPU (currently model load slows it down significantly)
- Goal of small model (currently the model is ~226 MB per voice and still growing, much larger than the original goal of ~3MB per voice)
- Already speech of higher speech quality than comparable footprint competitor systems (espeak, etc...)
- TODO: Generalize well to out-of-training set words (currently it generates well mostly just the words which were seen during training)

## Speech samples

- [gospeak TTS-Synthesized Speech Samples](https://github.com/neurlang/gospeak/wiki/TTS-Synthesized-Speech-Samples)
