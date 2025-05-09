from pathlib import Path
from argparse import ArgumentParser
from ffmpeg_normalize import FFmpegNormalize
from pydub import AudioSegment
import shutil
import tempfile
from tqdm import tqdm


def get_args():
    parser = ArgumentParser()
    parser.add_argument('src', type=Path)
    parser.add_argument('dst', type=Path)
    return parser.parse_args()

def normalize(paths: list[Path], dst_dir: Path):
    normalizer = FFmpegNormalize()
    temp_folder = Path(tempfile.gettempdir()) / 'gospeak_normalize'
    temp_folder.mkdir(exist_ok=True)

    for src_path in tqdm(paths, desc='Padding short files'):
        segment: AudioSegment = AudioSegment.from_wav(str(src_path))
        if segment.duration_seconds < 3:
            # Path
            silent = AudioSegment.silent(duration=3000)
            temp_path = temp_folder.joinpath(src_path.name)
            padded: AudioSegment = segment + silent
            padded.export(temp_path, format='wav')
            src_path = temp_path
        # Add to pool
        dst_path = dst_dir.joinpath(src_path.name)
        normalizer.add_media_file(str(src_path), str(dst_path))
        # Remove temp file
    # Run normalize
    normalizer.run_normalization()

    # Clean temp folder
    if temp_folder.exists():
        shutil.rmtree(temp_folder)


def main():
    args = get_args()
    src_dir: Path = args.src.resolve()
    dst_dir: Path = args.dst.resolve()

    dst_dir.mkdir(exist_ok=True, parents=True)
    files = list(src_dir.glob('*.wav'))
    print(f'Found {len(files)} in {src_dir.name} folder')
    normalize(files, dst_dir)
    

main()