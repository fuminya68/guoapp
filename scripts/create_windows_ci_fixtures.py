"""Generate short deterministic MP4 and HLS footage for Windows CI.

Use only a previously SHA256-verified FFmpeg executable, no live providers.
"""
import argparse
from pathlib import Path
import subprocess


def run(*args):
    subprocess.run(args, check=True, timeout=65)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    mp4 = output / "clear.mp4"
    run("ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
        "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=12:duration=8",
        "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100:duration=8",
        "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p",
        "-g", "24", "-c:a", "aac", "-shortest", "-movflags", "+faststart", str(mp4))
    run("ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
        "-i", str(mp4), "-map", "0:v:0", "-map", "0:a:0",
        "-c", "copy", "-hls_time", "2", "-hls_list_size", "0",
        "-hls_segment_filename", str(output / "segment-%03d.ts"),
        str(output / "index.m3u8"))
    if mp4.stat().st_size < 5000 or not (output / "index.m3u8").is_file():
        raise RuntimeError("Synthetic CI video generation failed")
    print("Windows MP4 and HLS fixtures generated:", output, flush=True)


if __name__ == "__main__":
    main()
