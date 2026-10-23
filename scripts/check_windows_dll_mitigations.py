#!/usr/bin/env python3
"""Fail closed when FFmpeg native PE modules have ASLR or NX protection disabled."""
import argparse
import csv
import pathlib
import struct
import sys

FFMPEG_RUNTIME = {
    "avcodec-62.dll", "avdevice-62.dll", "avfilter-11.dll",
    "avformat-62.dll", "avutil-60.dll", "libbz2-1.dll",
    "libffmpegkit.dll", "libfftw3-3.dll", "libgcc_s_seh-1.dll",
    "libgomp-1.dll", "libiconv-2.dll", "liblzma-5.dll",
    "librubberband-2.dll", "libsamplerate-0.dll", "libstdc++-6.dll",
    "libva.dll", "libva_win32.dll", "libvidstab.dll",
    "libwinpthread-1.dll", "libx264-165.dll", "libx265-216.dll",
    "swresample-6.dll", "swscale-9.dll", "xvidcore.dll", "zlib1.dll",
}

def protections(path):
    with path.open("rb") as stream:
        mz = stream.read(64)
        if len(mz) < 64 or mz[:2] != b"MZ":
            raise ValueError("not a Windows PE file")
        header = struct.unpack_from("<I", mz, 0x3c)[0]
        stream.seek(header)
        if stream.read(4) != b"PE\\0\\0":
            raise ValueError("invalid PE header")
        stream.seek(header + 24)
        magic = struct.unpack("<H", stream.read(2))[0]
        if magic != 0x20b:
            raise ValueError("not x64 PE32+")
        stream.seek(header + 24 + 70)
        flags = struct.unpack("<H", stream.read(2))[0]
        return {
            "dynamic_base": bool(flags & 0x40),
            "high_entropy_va": bool(flags & 0x20),
            "nx_compat": bool(flags & 0x100),
        }

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("bundle", type=pathlib.Path)
    parser.add_argument("--report", type=pathlib.Path, required=True)
    args = parser.parse_args()
    rows = []
    for name in sorted(FFMPEG_RUNTIME):
        target = args.bundle / name
        if not target.exists():
            raise SystemExit("Required FFmpeg runtime missing: " + name)
        result = protections(target)
        rows.append({"file": name, **result})
    args.report.parent.mkdir(parents=True, exist_ok=True)
    with args.report.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=["file", "dynamic_base", "high_entropy_va", "nx_compat"])
        writer.writeheader()
        writer.writerows(rows)
    unsafe = [x["file"] for x in rows if not all((x["dynamic_base"], x["high_entropy_va"], x["nx_compat"]))]
    if unsafe:
        raise SystemExit("FFmpeg DLLs lacking ASLR/DEP mitigations: " + ", ".join(unsafe))
    print("All", len(rows), "FFmpeg runtime DLLs have ASLR + high-entropy VA + NX")

if __name__ == "__main__":
    main()
