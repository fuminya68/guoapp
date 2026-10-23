# Windows x64 security hardening and release gate

This is an independent security-review branch. It is **not a malware-free guarantee**.

## Threat model

- New devices discovered on the local network must not gain synchronization
  access solely because of discovery. An explicit confirmation prompt is
  required on the receiving device. Previously paired devices may reconnect
  only when their stored SHA-256 certificate fingerprint matches.
- The existing LAN transport still listens on a network interface **only if
  the user enables device linking**. Keep device linking disabled on
  untrusted/public Wi-Fi and do not grant inbound firewall exceptions unless
  pairing is needed.
- The local video proxy binds to 127.0.0.1 and uses unpredictable session IDs.
  It is not a general web proxy or a substitute for endpoint firewalling.
- Third-party content, URLs, and playback libraries are not assumed trustworthy.
  A clean source review does not certify prebuilt libmpv/ANGLE/FFmpeg binaries.

## Hardened GitHub Actions

This branch's Windows x64 build workflow is gated by:

1. OSV dependency advisory scan (Go and Dart lockfiles).
2. Flutter regression tests for LAN peer trust.
3. `go mod verify` checksums and `govulncheck` source reachability scan.
4. SHA-256 checked, packaged Windows binaries.
5. Microsoft Defender custom scan of extracted release ZIP.

Security tools can return false positives and false negatives. A build is **not
released** if any required scanner fails, is unavailable, or reports findings.
Review GitHub Actions logs before using an artifact. The CI runners are
ephemeral. These checks do not replace runtime dynamic analysis, signature
verification, or independent review of the bundled native media DLLs.

## Installation precautions

- Do not run as Administrator.
- Do not enable device linking on untrusted networks.
- First-run in Windows Sandbox or an isolated VM, monitor outbound connections.
- Keep Windows Defender and Windows Update enabled.
- Do not store exchange-wallet credentials, API keys, or privileged company
  data in the environment used for evaluating untrusted media applications.

## Open items for manual review

- Upstream repository contains Android key.properties and keystore files. The
  publisher should rotate any affected signing keys and remove the private
  material from its history; deletion in this fork cannot revoke leaked keys.
- Verify reproducibility of Dart lockfile resolution. In this build, the
  runner executes `flutter pub get` to resolve a stale upstream lockfile.
- **SSRF mitigation under review:** `native/core/media_ssrf.go` now validates
  untrusted HLS media URLs, rejects loopback/private/link-local/reserved IPs
  and hostnames, checks redirect targets, resolves and checks the full DNS
  address set at dial time, then connects directly to the checked IP.
  The local playback streaming proxy, HLS manifest fetching, and offline media
  downloads use this stricter transport. This guards against the original
  local-network and DNS-rebinding SSRF pathways in these media flows.
  The mitigation intentionally disables HTTP proxies and specialized
  TLS-browser-fingerprint transports for these media requests. Some providers
  may not work until there is an equivalent proxy-safe implementation.
  Security testing alone does not prove real playback compatibility.
  Other app-originated HTTP clients, including catalog and image paths, have
  NOT received a comprehensive SSRF audit and remain outside this fix.
- Audit the origin, maintenance and known vulnerabilities of pinned
  libmpv/FFmpeg/ANGLE Windows native libraries. SHA-256 pinning proves byte
  identity, not that downloaded bytes are benign.
- Conduct a dynamic network capture and endpoint security scan on the actual
  machine before production use. A CI pass does not conclusively prove safety.

## Security fix

- `lib/lan_controller.dart` rejects unapproved incoming first-time peers and
  disables automatic pairing to an unknown device.
- `lib/main.dart` asks for an explicit receiver-side first-time decision.
- `test/lan_security_test.dart` tests certificate fingerprint trust logic.

The changes are intentionally isolated from `main` until the security build
passes and is reviewed.
