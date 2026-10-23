import argparse
import json
import platform
import re
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path

from app_build import BuildVariant, add_variant_argument

root = Path(__file__).resolve().parents[1]


def main():
    # Windows GitHub runners default to cp1252, while native Flutter/mpv output
    # may contain Unicode. Always preserve diagnostics instead of crashing.
    sys.stdout.reconfigure(errors='backslashreplace')
    sys.stderr.reconfigure(errors='backslashreplace')
    parser = argparse.ArgumentParser()
    add_variant_argument(parser)
    variant = BuildVariant(parser.parse_args().all_sources)
    if platform.system() != 'Windows':
        raise SystemExit('此检查需要 Windows。')
    version = re.search(r'^version:\s*(\S+)', (root / 'pubspec.yaml').read_text(encoding='utf-8'), re.MULTILINE).group(1)
    package = root / 'dist' / 'windows' / f'{variant.slug}-{version}-windows-x64.zip'
    with tempfile.TemporaryDirectory(prefix='zhenguojian-smoke-') as temporary:
        directory = Path(temporary)
        with zipfile.ZipFile(package) as archive:
            archive.extractall(directory)
        media = directory / 'fixture.mp4'
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi',
                        '-i', 'testsrc2=size=160x90:rate=12', '-t', '3',
                        '-c:v', 'libx264', '-threads', '1', str(media)], check=True)
        report = directory / 'result.json'
        command = [str(directory / (variant.slug + '.exe')), '--package-smoke', str(report), str(media)]
        try:
            process = subprocess.run(command, cwd=directory, check=False,
                                     timeout=90, capture_output=True, text=True,
                                     errors='replace')
        except subprocess.TimeoutExpired as error:
            print('PACKAGED_EXE_TIMEOUT: ' + str(error), flush=True)
            raise
        if process.stdout:
            print('PACKAGED_EXE_STDOUT:\\n' + process.stdout[-16000:], flush=True)
        if process.stderr:
            print('PACKAGED_EXE_STDERR:\\n' + process.stderr[-16000:], flush=True)
        output = root / 'build' / 'windows-package-smoke.json'
        output.parent.mkdir(parents=True, exist_ok=True)
        if report.is_file():
            evidence = json.loads(report.read_text(encoding='utf-8'))
        else:
            evidence = {'ok': False, 'error': 'Missing report from app', 'exitCode': process.returncode}
        output.write_text(json.dumps(evidence, indent=2) + '\n', encoding='utf-8')
        print('PACKAGED_EXE_RESULT: ' + json.dumps(evidence, ensure_ascii=False), flush=True)
        if process.returncode != 0 or evidence.get('ok') is not True:
            raise SystemExit('Packaged Windows smoke rejected: ' + str(evidence.get('error') or process.returncode))


if __name__ == '__main__':
    main()
