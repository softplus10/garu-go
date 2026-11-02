#!/usr/bin/env python3
"""Vendor an exact upstream tag, without modifying its Rust core or model."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent
REPOSITORY = 'https://github.com/ongjin/garu'


def run(*args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()


def sync(tag):
    if not re.fullmatch(r'v0\.\d+\.\d+', tag):
        raise SystemExit('Expected an upstream tag such as v0.12.1')
    previous = json.loads((ROOT / 'upstream.json').read_text())['tag']
    with tempfile.TemporaryDirectory(prefix='garu-sync-') as tmp:
        src = Path(tmp)
        subprocess.run(['git', 'init', '-q', str(src)], check=True)
        subprocess.run(['git', '-C', str(src), 'fetch', '-q', '--depth=1', REPOSITORY + '.git', 'refs/tags/' + tag], check=True)
        commit = run('git', '-C', str(src), 'rev-parse', 'FETCH_HEAD^{commit}')
        subprocess.run(['git', '-C', str(src), 'checkout', '-q', '--detach', commit], check=True)
        # Fail before replacing files when a historical tag has a different layout/API.
        core = src / 'crates/garu-core'
        model = src / 'js/models/base.gmdl'
        if not (core / 'src/model.rs').is_file() or not model.is_file():
            raise SystemExit('Unsupported upstream layout; no files replaced')
        for old, new in [(core, ROOT / 'third_party/garu-core'), (src / 'js/models', ROOT / 'models')]:
            shutil.rmtree(new, ignore_errors=True)
            shutil.copytree(old, new)
        for name in ['LICENSE', 'NOTICE']:
            origin = src / name
            target = ROOT / 'third_party/garu' / name
            if origin.exists():
                shutil.copy2(origin, target)
            elif target.exists():
                target.unlink()
        # Earlier tags may have no top-level NOTICE. Keep the wrapper's attribution.
        tags = [t.strip() for t in re.search(r'pub enum Pos \{(.*?)\}', (core / 'src/types.rs').read_text(), re.S).group(1).split(',') if t.strip()]
        (ROOT / 'pos.go').write_text('// Code generated from the upstream Sejong POS enum. DO NOT EDIT.\npackage garu\n\n// POS is an unchanged upstream Sejong part-of-speech tag.\ntype POS string\n\nconst (\n' + ''.join(f'\t{t} POS = "{t}"\n' for t in tags) + ')\n')
        version = tag[1:]
        readme = ROOT / 'README.md'
        readme.write_text(readme.read_text().replace(previous, tag).replace('garu-ko@' + previous[1:], 'garu-ko@' + version))
        cargo = ROOT / 'native/Cargo.toml'
        cargo.write_text(re.sub(r'(?m)^version = "[^"]+"', f'version = "{version}"', cargo.read_text(), count=1))
        (ROOT / 'version.go').write_text(
            'package garu\n\n// Version matches the upstream Garu tag.\n'
            f'const Version = "{version}"\n\n'
            '// UpstreamCommit identifies the unmodified Rust core and bundled model.\n'
            f'const UpstreamCommit = "{commit}"\n'
        )
        (ROOT / 'upstream.json').write_text(json.dumps({
            'repository': REPOSITORY, 'tag': tag, 'commit': commit,
            'model_sha256': hashlib.sha256((ROOT / 'models/base.gmdl').read_bytes()).hexdigest()
        }, indent=2) + '\n')
        subprocess.run(['cargo', 'generate-lockfile', '--manifest-path', str(ROOT / 'native/Cargo.toml')], check=True)
        subprocess.run(['gofmt', '-w', str(ROOT / 'version.go'), str(ROOT / 'pos.go')], check=True)
        print(f'Synced {tag} at {commit}; build and test before tagging this wrapper.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tag')
    sync(parser.parse_args().tag)
