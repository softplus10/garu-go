#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
prefix=${1:?Usage: install-native.sh PREFIX}
sh "$root/scripts/build-native.sh"
mkdir -p "$prefix/lib" "$prefix/include"
cp "$root/native/target/release/libgaru_go.a" "$prefix/lib/"
cp "$root/native/garu_go.h" "$prefix/include/"
echo "Installed native library. Build Go consumers with CGO_ENABLED=1 and CGO_LDFLAGS=-L$prefix/lib"
