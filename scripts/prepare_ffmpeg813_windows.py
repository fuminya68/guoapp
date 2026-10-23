#!/usr/bin/env python3
"""Prepare a *test-only* FFmpegKit Windows bundle with upgraded FFmpeg core DLLs.

FFmpegKit C ABI is retained from the upstream min-gpl 8.1.2 bundle; only
same-major FFmpeg libav* DLLs are replaced with a pinned FFmpeg 8.1.3 branch
build. Fail closed if hashes, versions, ABI or runtime checks fail in CI.
This is a compatibility experiment, not a claim that every codec is safe.
"""
import argparse
import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.request
import zipfile

FFMPEGKIT_SOURCE = (
    "https://github.com/sk3llo/ffmpeg_kit_flutter/releases/download/"
    "8.1.2-min-gpl/ffmpeg-kit-windows-x86_64-min-gpl-8.1.2.zip"
)
FFMPEGKIT_SHA256 = "c3a69388f32ff86e4622baada470d2acc55d577a51b6ec15b68560c1f1b6e37b"
FFMPEG_SOURCE = (
    "https://github.com/BtbN/FFmpeg-Builds/releases/download/"
    "autobuild-2026-10-22-13-06/"
    "ffmpeg-n8.1.3-23-g5c4f5744cd-win64-gpl-shared-8.1.zip"
)
FFMPEG_SHA256 = "da8e09a49faf8eb45dd41bf11d04ab49816df42b72c0850c65843a31f81d2b70"

FFMPEG_CORE = (
    "avcodec-62.dll", "avdevice-62.dll", "avfilter-11.dll",
    "avformat-62.dll", "avutil-60.dll", "swresample-6.dll",
    "swscale-9.dll",
)
EXPECTED_DLL_VERSIONS = {"avcodec-62.dll": (62, 28), "avutil-60.dll": (60, 26)}


def filehash(path):
    h = hashlib.sha256()
    with open(path, "rb") as fd:
        for data in iter(lambda: fd.read(1048576), b""):
            h.update(data)
    return h.hexdigest()


def download(url, expected, target):
    request = urllib.request.Request(url, headers={"User-Agent": "guoapp-ffmpeg-security-builder/1"})
    with urllib.request.urlopen(request, timeout=180) as response, open(target, "wb") as output:
        shutil.copyfileobj(response, output)
    actual = filehash(target)
    if actual.lower() != expected.lower():
        raise RuntimeError(f"Release digest mismatch: {target.name}: {actual}")
    print(f"Verified {target.name}: {actual}", flush=True)


def extract(archive, folder):
    with zipfile.ZipFile(archive) as src:
        for member in src.infolist():
            p = Path(member.filename)
            if p.is_absolute() or ".." in p.parts:
                raise RuntimeError("Archive traversal detected: " + member.filename)
        src.extractall(folder)


def unique_file(root, basename):
    candidates = [p for p in root.rglob("*") if p.is_file() and p.name.lower() == basename.lower()]
    if len(candidates) != 1:
        raise RuntimeError(f"Expected one {basename} in {root}, found {len(candidates)}")
    return candidates[0]


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--evidence", type=Path, required=True)
    args = p.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    args.evidence.mkdir(parents=True, exist_ok=True)
    if any(args.output.iterdir()):
        raise RuntimeError("Refusing to reuse previously staged runtime directory")
    with tempfile.TemporaryDirectory(prefix="ffmpeg-update-") as t:
        temp = Path(t)
        kit_zip, ff_zip = temp / "ffmpegkit.zip", temp / "ffmpeg813.zip"
        download(FFMPEGKIT_SOURCE, FFMPEGKIT_SHA256, kit_zip)
        download(FFMPEG_SOURCE, FFMPEG_SHA256, ff_zip)
        kit, ff = temp / "kit", temp / "ff"
        kit.mkdir()
        ff.mkdir()
        extract(kit_zip, kit)
        extract(ff_zip, ff)
        kit_bin = unique_file(kit, "libffmpegkit.dll").parent
        new_bin = unique_file(ff, "avcodec-62.dll").parent
        output_bin = args.output / "bin"
        shutil.copytree(kit_bin, output_bin)
        # The newer FFmpeg is not an entire FFmpegKit build; keep the plugin
        # wrapper and old codec-support dependencies unchanged.
        for name in FFMPEG_CORE:
            source = new_bin / name
            if not source.is_file() or not (output_bin / name).is_file():
                raise RuntimeError("Missing old/new same-major FFmpeg library: " + name)
            shutil.copy2(source, output_bin / name)
            if filehash(source) != filehash(output_bin / name):
                raise RuntimeError("Failed byte-for-byte check after copy: " + name)
        # Preserve license and FFmpegKit notices, add new FFmpeg binary origin.
        notices = kit_bin.parent / "LICENSES"
        if notices.is_dir():
            shutil.copytree(notices, args.output / "LICENSES")
        (args.evidence / "binary-provenance.txt").write_text(
            "FFmpegKit 8.1.2-min-gpl (wrapper / codec dependencies): " + FFMPEGKIT_SOURCE +
            "\nSHA256: " + FFMPEGKIT_SHA256 +
            "\nReplacement FFmpeg 8.1.3+23-g5c4f5744cd (7 av* DLLs): " + FFMPEG_SOURCE +
            "\nSHA256: " + FFMPEG_SHA256 + "\n", encoding="utf-8")
        lines = []
        for name in FFMPEG_CORE:
            lines.append(filehash(output_bin / name) + "  " + name)
        (args.evidence / "ffmpeg-8.1.3-dll-sha256.txt").write_text("\n".join(lines) + "\n")
        # Build provenance is *not* enough: verify a functioning vendor ffmpeg
        # binary and actual runtime library versions in the Windows validation step.
        original_exe = unique_file(ff, "ffmpeg.exe")
        smoke = subprocess.run([str(original_exe), "-version"], capture_output=True, text=True, timeout=60)
        if smoke.returncode != 0 or "n8.1.3" not in smoke.stdout:
            raise RuntimeError("FFmpeg 8.1.3 vendor self-identification failed: " + smoke.stderr)
        (args.evidence / "upstream-ffmpeg-version.txt").write_text(smoke.stdout[:2500], encoding="utf-8")
        print("Prepared 8.1.3 same-major overlay:", args.output.resolve(), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"FFmpeg security upgrade FAILED: {exc}", file=sys.stderr)
        sys.exit(1)
