# Prepare

Scripts to prepare data for training

## Setup

```console
pip install uv
uv sync
```

## Normalize

```console
uv run normalize ./wav_dir ./out_dir
```
alternatively (if you can't use python)
```console
ffmpeg-normalize *.wav -of . -ext wav -f -ar 48000 -t -15
```
