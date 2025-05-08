# KMeans1 - Acoustic Speech Codec Generator

## Overview

KMeans1 is a program that generates an acoustic speech codec based on a collection
of voice utterances from a single speaker. The tool uses k-means clustering to create 
a compact representation of vocal characteristics that can be used for speech encoding and synthesis.

## Features

- Processes audio files to create a speaker-specific codec
- Supports multiple processing stages with progress tracking
- Sample rates auto detection
- Automatically chooses cluster counts
- Optional debug execution mode

## Requirements

- Linux environment
- Audio files in FLAC or WAV formats
- Sufficient disk space for processing

## Speech Corpus Requirements

- Supported format: FLAC (preferred) or WAV
- Recommended at minimum 1024 files (split files on silence if few too long files)
- If less than 1024 files, a lower quality and lower bitrate codec will be generated
- One speaker per directory
- All files share the same sample rate
- Supported sample rate per directory:

| Option A     | Option B     |
|--------------|--------------|
| 8000 Hz      | 11025 Hz     |
| 16000 Hz     | 22050 Hz     |
| **48000 Hz** | **44100 Hz** |

Note: Codec native sample rate is in bold.

## Installation

1. Ensure you have the necessary build tools installed (go, etc.)
2. Clone or download the kmeans1 source code
3. Compile the program using (`go build`)

## Usage

Basic command format:
```
./kmeans1 --srcdir <source_directory> --dstdir <destination_directory> [optional options]
```

### Example Run
```
$ ./kmeans1 --srcdir ~/Corpus/flac \
            --dstdir ~/Corpus \
            --execute 'echo STAGE_NUMBER of TOTAL_STAGES' \
            --executedbg
```

### Output Explanation
The program provides progress information including:
- Sample rates (input and codec native)
- Number of files processed
- Chunk information
- K-means cluster counts
- Progress bars for each stage
- Stage execution commands

### Options

| Option         | Description |
|----------------|-------------|
| `--srcdir`     | Source directory containing audio files |
| `--dstdir`     | Destination directory for completed codec output |
| `--execute`    | Command to execute at each stage (use STAGE_NUMBER, TOTAL_STAGES placeholders) |
| `--executedbg` | Enable debug mode for executed commands |

## Processing Stages

1. **Initialization**: Sets up processing environment and analyzes input files
2. **Initial Clusterings**: Processes audio files using k-means on a subset of the files each time
3. **Final Clustering**: Performs k-means clustering to create the codec representation
4. **Finalization**: Completes codec generation, encodes all audio files using the codec and verifies success

## Output

Upon successful completion, the program will:
- Create codec files in the destination directory
- Display "Codec solved: true" confirmation
- Report the total number of clusters created

## Troubleshooting

- Ensure all audio files are from the same speaker
- Ensure all audio files do have the same sample rate
- Verify sufficient disk space is available in dstdir (e.g. 600MB)
- Check file permissions for source and destination directories
- For debugging the custom command, use the `--executedbg` flag

## License

MIT license 

## Author

neurlang
