#!/usr/bin/env python3
"""Compare shipped DLL bytes with SHA-256 pinned upstream release assets.

Only reads the Windows ZIP contents and official archives; never executes a DLL.
"""
import argparse
import csv
import hashlib
import pathlib
import shutil
import subprocess
import sys
import tempfile
import urllib.request
import zipfile

ARCHIVES = (
    {
        "name": "ffmpeg813",
        "url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-10-22-13-06/ffmpeg-n8.1.3-23-g5c4f5744cd-win64-gpl-shared-8.1.zip",
        "sha256": "da8e09a49faf8eb45dd41bf11d04ab49816df42b72c0850c65843a31f81d2b70",
        "suffix": ".zip",
        "dlls": (
            "avcodec-62.dll", "avdevice-62.dll", "avfilter-11.dll",
            "avformat-62.dll", "avutil-60.dll", "swresample-6.dll",
            "swscale-9.dll",
        ),
    },
    {
        "name": "ffmpegkit",
        "url": "https://github.com/sk3llo/ffmpeg_kit_flutter/releases/download/8.1.2-min-gpl/ffmpeg-kit-windows-x86_64-min-gpl-8.1.2.zip",
        "sha256": "c3a69388f32ff86e4622baada470d2acc55d577a51b6ec15b68560c1f1b6e37b",
        "suffix": ".zip",
        "dlls": (
              
              "libbz2-1.dll",
            "libffmpegkit.dll", "libfftw3-3.dll", "libgcc_s_seh-1.dll",
            "libgomp-1.dll", "libiconv-2.dll", "liblzma-5.dll",
            "librubberband-2.dll", "libsamplerate-0.dll",
            "libstdc++-6.dll", "libva.dll", "libva_win32.dll",
            "libvidstab.dll", "libwinpthread-1.dll", "libx264-165.dll",
            "libx265-216.dll",  
            "xvidcore.dll", "zlib1.dll",
        ),
    },
    {
        "name": "mpv",
        "url": "https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/20260921/mpv-dev-x86_64-20260921-git-e76a35ec95.7z",
        "sha256": "361392d64a231830ac79d1c0f91ddb68c6a709accd282b2c87b13889d382fdec",
        "dlls": ("libmpv-2.dll",),
    },
    {
        "name": "angle",
        "url": "https://github.com/alexmercerind/flutter-windows-ANGLE-OpenGL-ES/releases/download/v1.0.1/ANGLE.7z",
        "sha256": "cc5911bb15d596fd5a2b362613ad35b7093b427117269a7359054a65746a5f9a",
        "dlls": (
            "d3dcompiler_47.dll", "libc++.dll", "libEGL.dll",
            "libGLESv2.dll", "vk_swiftshader.dll", "vulkan-1.dll", "zlib.dll"
        ),
    },
)


def digest(path):
    value = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            value.update(chunk)
    return value.hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("bundle", type=pathlib.Path)
    parser.add_argument("--evidence", type=pathlib.Path, required=True)
    args = parser.parse_args()
    if not args.bundle.is_dir():
        raise SystemExit("Windows DLL bundle directory not found")
    args.evidence.mkdir(parents=True, exist_ok=True)
    tool = shutil.which("7z") or shutil.which("7z.exe") or shutil.which("7za")
    if not tool:
        raise SystemExit("7-Zip CLI is unavailable: upstream archive provenance cannot be verified")

    rows = []
    with tempfile.TemporaryDirectory(prefix="dll-provenance-") as temporary:
        tmp = pathlib.Path(temporary)
        for source in ARCHIVES:
            print("Fetching upstream release:", source["name"], flush=True)
            archive = tmp / (source["name"] + source.get("suffix", ".7z"))
            request = urllib.request.Request(source["url"], headers={"User-Agent": "guoapp-dll-provenance-check/1.0"})
            with urllib.request.urlopen(request, timeout=120) as response, archive.open("wb") as output:
                shutil.copyfileobj(response, output)
            actual_archive = digest(archive)
            if actual_archive != source["sha256"]:
                raise SystemExit("UPSTREAM ARCHIVE SHA256 MISMATCH: " + source["name"])
            extracted = tmp / source["name"]
            extracted.mkdir()
            if archive.suffix == ".zip":
                with zipfile.ZipFile(archive) as stream:
                    stream.extractall(extracted)
            else:
                subprocess.run([tool, "x", "-y", str(archive), "-o" + str(extracted)], check=True, stdout=subprocess.DEVNULL)
            for name in source["dlls"]:
                choices = [p for p in extracted.rglob("*") if p.is_file() and p.name.lower() == name.lower()]
                if len(choices) != 1:
                    raise SystemExit("Missing or ambiguous archive DLL: " + name)
                target = args.bundle / name
                if not target.is_file():
                    raise SystemExit("Missing release DLL: " + name)
                expected, actual = digest(choices[0]), digest(target)
                if actual != expected:
                    raise SystemExit("DLL DOES NOT MATCH UPSTREAM: " + name)
                rows.append({
                    "filename": name,
                    "upstream": source["name"],
                    "release_url": source["url"],
                    "upstream_archive_sha256": source["sha256"],
                    "dll_sha256": actual,
                    "result": "byte-identical",
                })
                print("BYTE MATCH", name, actual, flush=True)
    with (args.evidence / "source-matches.csv").open("w", newline="", encoding="utf-8") as out:
        writer = csv.DictWriter(out, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    with (args.evidence / "all-binaries-sha256.csv").open("w", newline="", encoding="utf-8") as out:
        writer = csv.writer(out)
        writer.writerow(("filename", "size_bytes", "sha256"))
        for path in sorted(args.bundle.iterdir()):
            if path.is_file() and path.suffix.lower() in (".dll", ".exe"):
                writer.writerow((path.name, path.stat().st_size, digest(path)))
    print("Vendor byte-identical files:", len(rows))


if __name__ == "__main__":
    main()
