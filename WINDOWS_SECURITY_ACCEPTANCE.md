# Windows x64 security acceptance: pinned mpv and functional smoke gates

Branch: `security/mpv-windows-integrations`. These are release gating tests; **they are not a guarantee against malware or every exploit**.

## Pinned Windows native media software

- `libmpv-2.dll` is taken from shinchiro/mpv-winbuild-cmake's **2026-10-21** x64-dev archive `mpv-dev-x86_64-20261021-git-e471496c56.7z`.
- SHA-256 pinned in `packages/media_kit_libs_windows_video/windows/CMakeLists.txt` and `scripts/verify_windows_dll_provenance.py`: `efc719eff3dc5954b4a5efc800632808bcec01cc9034940dd14e5eb7fe9ea633`.
- The GitHub Windows build fails if the archived source digest mismatches or the `libmpv-2.dll` in the final package differs byte-for-byte from the audited release.
- FFmpegKit remains on the verified FFmpeg 8.1.3 Windows overlay from the prior branch. All FFmpeg/ANGLE provenance, ASLR/NX, OSV, Go vulnerability and Defender CI checks stay enforced.

## Windows functional CI acceptance

1. Build the **same** Windows x64 all-sources ZIP that is offered for download, then unpack it and check native DLLs.
2. Start the **packaged executable** using its explicit `--package-smoke` entry point. Confirm Go native core initializes, FFmpegKit reads a real short MP4, performs a real remux into MKV, and libmpv decodes and advances playback for the resulting file. The smoke writes a JSON report; failure blocks upload.
3. Generate reproducible local MP4 and HTTP HLS fixtures from the checksum-verified FFmpeg tool bundled temporarily in CI. `integration_test/windows_media_test.dart` opens a **Windows Flutter desktop GUI**, verifies a synthetic catalog/detail episode, decodes MP4 video, switches episodes through GUI and decodes HLS via local-only HTTP.
4. Run the existing Flutter `downloads_test.dart` and `media_library_test.dart` regression suites. These are UI/data/merge-related tests, not proof that every real provider's download endpoint is reachable.
5. Upload build artifacts only after all security and functional checks pass.

No login credentials, private tokens or real provider video are required for these fixtures.

## Residual checks before production daily use

- Live source discovery, streaming, encrypted playback and actual downloads on each provider are not fully verified by synthetic fixtures.
- Executing inside a Windows CI runner is not a substitute for a clean VM with network/process/registry monitoring.
- Windows 10 should have ESU and all supported security updates; run the app as a standard user, avoid inbound firewall exceptions and initially disable LAN device linking.
- Continue to track shinchiro/mpv and FFmpeg native codec CVEs and check whether updated releases are available.

This branch is intentionally **not merged to main** pending a clean CI run and further review.
