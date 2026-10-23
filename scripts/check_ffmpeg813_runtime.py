#!/usr/bin/env python3
"""Fail closed unless the packaged Windows FFmpeg DLLs load with FFmpeg >=8.1.3.

The upstream FFmpegKit wrapper must load with the replaced DLLs. This detects
missing transitive DLL dependencies and obvious DLL ABI mismatches, but is not
a substitute for GUI integration or codec fuzzing.
"""
import argparse
import ctypes
import os
from pathlib import Path
import sys

CORE = (
    "avutil-60.dll", "swresample-6.dll", "swscale-9.dll",
    "avcodec-62.dll", "avformat-62.dll", "avfilter-11.dll",
    "avdevice-62.dll",
)

def main():
    p=argparse.ArgumentParser()
    p.add_argument("bundle",type=Path)
    args=p.parse_args()
    if sys.platform != "win32":
        raise RuntimeError("DLL load-time check requires a Windows runner")
    directory=args.bundle.resolve()
    if not directory.is_dir():
        raise RuntimeError("Packaged DLL directory is missing")
    with os.add_dll_directory(str(directory)):
        loaded={}
        for name in CORE + ("libffmpegkit.dll",):
            if not (directory/name).is_file():
                raise RuntimeError("Missing DLL: "+name)
            try:
                loaded[name]=ctypes.WinDLL(str(directory/name))
            except OSError as error:
                raise RuntimeError("Windows loader failed for "+name+": "+str(error)) from error
        for name, func, major, minor, micro in [
            ("avcodec-62.dll","avcodec_version",62,28,103),
            ("avutil-60.dll","avutil_version",60,26,103),
            ("avformat-62.dll","avformat_version",62,12,103),
        ]:
            method=getattr(loaded[name],func)
            method.restype=ctypes.c_uint
            value=method()
            triplet=(value>>16,(value>>8)&255,value&255)
            print(name,func,triplet,flush=True)
            if triplet[:2] != (major,minor) or triplet[2]<micro:
                raise RuntimeError(f"Outdated or incompatible FFmpeg DLL: {name}: {triplet}")
    print("All upgraded FFmpeg DLLs and legacy FFmpegKit wrapper loaded successfully",flush=True)

if __name__=="__main__":
    try:
        main()
    except Exception as exc:
        print("FFmpeg ABI/runtime verification FAILED: "+str(exc),file=sys.stderr)
        sys.exit(1)
