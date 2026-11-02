#!/usr/bin/env python3
"""Pin the Garu submodule to an exact upstream tag and update binding metadata."""
import argparse
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parent.parent
UPSTREAM = ROOT / 'upstream'


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def sync(tag):
    if not re.fullmatch(r'v0\.\d+\.\d+', tag):
        raise SystemExit('Expected an upstream tag such as v0.12.1')
    subprocess.run(['git', '-C', str(ROOT), 'submodule', 'update', '--init', '--recursive'], check=True)
    if run('git', '-C', str(UPSTREAM), 'status', '--porcelain'):
        raise SystemExit('Upstream has local changes; refusing to switch it')
    subprocess.run(['git', '-C', str(UPSTREAM), 'fetch', 'origin', 'refs/tags/' + tag], check=True)
    commit = run('git', '-C', str(UPSTREAM), 'rev-parse', 'FETCH_HEAD^{commit}')
    # Check the new layout before switching the submodule.
    types = run('git', '-C', str(UPSTREAM), 'show', commit + ':crates/garu-core/src/types.rs')
    run('git', '-C', str(UPSTREAM), 'cat-file', '-e', commit + ':js/models/base.gmdl')
    tags = [t.strip() for t in re.search(r'pub enum Pos \{(.*?)\}', types, re.S).group(1).split(',') if t.strip()]
    subprocess.run(['git', '-C', str(UPSTREAM), 'checkout', '--detach', commit], check=True)
    version = tag[1:]
    (ROOT / 'pos.go').write_text('// Code generated from the upstream Sejong POS enum. DO NOT EDIT.\npackage garu\n\n// POS is an unchanged upstream Sejong part-of-speech tag.\ntype POS string\n\nconst (\n' + ''.join(f'\t{t} POS = "{t}"\n' for t in tags) + ')\n')
    (ROOT / 'version.go').write_text(
        'package garu\n\n// Version matches the upstream Garu tag.\n'
        f'const Version = "{version}"\n\n'
        '// UpstreamCommit identifies the unmodified Rust core and bundled model.\n'
        f'const UpstreamCommit = "{commit}"\n'
    )
    cargo = ROOT / 'native/Cargo.toml'
    cargo.write_text(re.sub(r'(?m)^version = "[^"]+"', f'version = "{version}"', cargo.read_text(), count=1))
    subprocess.run(['cargo', 'generate-lockfile', '--manifest-path', str(cargo)], check=True)
    subprocess.run(['gofmt', '-w', str(ROOT / 'version.go'), str(ROOT / 'pos.go')], check=True)
    print(f'Pinned upstream to {tag} ({commit}); build and test before committing and tagging.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tag')
    sync(parser.parse_args().tag)
