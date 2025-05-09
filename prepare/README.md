# Prepare

Scripts to prepare data for training

## Setup

```console
pip install uv
uv sync
```

## 1. Normalize

```console
uv run normalize ./wav_dir ./out_dir
```
alternatively (if you can't use python)
```console
ffmpeg-normalize *.wav -of . -ext wav -f -ar 48000 -t -15
```
## 2. Split on silence if too few files

Run in flac dir
```
mkdir ../flacsplit/
for file in *.flac ; do \
sox $file ../flacsplit/$file.burst.flac silence 1 1 0.1% 1 1 0.1% : newfile : restart ; \
done
```
Then result appears in nearby flacsplit/ dir
