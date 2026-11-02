#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ ! -f "$root/upstream/crates/garu-core/Cargo.toml" ] || [ ! -f "$root/upstream/js/models/base.gmdl" ]; then
  echo 'Initialize the pinned upstream first: git submodule update --init --recursive' >&2
  exit 1
fi
cargo build --locked --release --manifest-path "$root/native/Cargo.toml" --target-dir "$root/native/target" "$@"
