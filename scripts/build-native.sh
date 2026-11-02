#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cargo build --locked --release --manifest-path "$root/native/Cargo.toml" --target-dir "$root/native/target" "$@"
