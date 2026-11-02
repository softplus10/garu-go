# garu-go

Thin Go/cgo bindings for [Garu](https://github.com/ongjin/garu), a lightweight Korean morphological analyzer.
The Rust analyzer and bundled model are **unmodified**. This package adds a small C ABI and Go API; it does not reimplement tokenization or train a new model.

## Build

Requires Go 1.23+, Rust stable, a C toolchain, and `CGO_ENABLED=1`.

```sh
git clone --recurse-submodules https://github.com/softplus10/garu-go.git
cd garu-go
sh scripts/build-native.sh
go test ./...
go run ./examples/search
```

On Debian/Ubuntu install `build-essential`; on Alpine install `build-base`. Rust must also be installed in the build environment.
The Rust static library is linked into the Go executable. Rust/Cargo are **build-time** tools, not running services.
The model is embedded in the Rust static library directly from the pinned submodule; startup does not download anything.

For an existing checkout, initialize the exact pinned revision with `git submodule update --init --recursive`. The build script reports this command if the submodule is missing.

### Use from another Go project

First build and install the matching native library into a prefix you control:

```sh
# In the garu-go checkout, at the same revision as the Go dependency:
sh scripts/install-native.sh "$HOME/.local"

# In your application:
go get github.com/softplus10/garu-go@main
CGO_ENABLED=1 CGO_LDFLAGS="-L$HOME/.local/lib" go build ./...
```

`go get` alone does not compile Rust or install the native library. Keep the library and Go module at the same version; initialization rejects a mismatched version.

## Usage

```go
package main

import (
    "fmt"
    "log"

    garu "github.com/softplus10/garu-go"
)

func main() {
    analyzer, err := garu.New()
    if err != nil { log.Fatal(err) }
    defer analyzer.Close()

    nouns, err := analyzer.Nouns("도서관의 책과 AI 문서", garu.NounsOptions{
        IncludeSL: true,
    })
    if err != nil { log.Fatal(err) }
    fmt.Println(nouns)
}
```

- `New(options ...Options)`: load the bundled upstream model.
- `NewWithModel(bytes, options ...Options)`: load a trusted custom model. Bytes may be released after initialization.
- `Analyze(text)`: token text, unchanged Sejong POS tags, start/end offsets, scores and elapsed time.
- `AnalyzeTopN(text, n)`: up to `n` upstream candidates, with `n` from 1 to 100.
- `Tokenize(text)`: all token surfaces in upstream order.
- `Nouns(text, options ...NounsOptions)`: NNG/NNP; optional SL, exactly like `garu-ko`'s noun filter.
- `Close()`: release native memory; safe to call repeatedly.

All upstream POS names are exported as typed constants (`garu.NNG`, `garu.NNP`, `garu.SL`, etc.).
Offsets are Unicode code point/rune offsets inherited from the Rust core, **not Go byte indices**.
No lowercasing, deduplication, particle stripping or extra normalization is applied.
`Options{NormalizeJamo: true}` passes the upstream jamo normalization option through.
JavaScript-only helpers for sentence splitting and spacing are not included.

An analyzer can be shared across goroutines. Calls and `Close` are serialized per instance; do not copy an analyzer value.
Reuse one instance rather than reloading the model per request. Explicitly call `Close`: native model memory is outside the Go heap and its memory limit.

## Alpine / PocketBase

Build both Rust and Go against **musl** for an Alpine deployment; a glibc static archive is not interchangeable with a musl archive.
`Dockerfile.alpine` demonstrates a native build, Go tests, and an executable running in a minimal Alpine runtime:

```sh
docker build -f Dockerfile.alpine -t garu-go-alpine .
docker run --rm garu-go-alpine
```

PocketBase can keep an analyzer in memory and call `Nouns` from Go routes. This repository supplies the analyzer binding only, not the student encoder or search ranking API.

## Upstream submodule and versions

```text
garu-go/
├── upstream/       # ongjin/garu Git submodule, pinned to one commit
├── native/         # C ABI adapter; depends on upstream/crates/garu-core
├── garu.go         # Go API
└── pos.go          # constants generated from upstream Sejong POS tags
```

There are no copied engine or model directories. The native build uses the core and
`js/models/base.gmdl` directly from `upstream/`. The submodule gitlink records the
upstream commit; `Version` and `UpstreamCommit` expose that identity to Go callers.
CI checks out submodules recursively. Docker builds use the already initialized
submodule from the build context.

Updating is explicit; ordinary clones/builds do not follow upstream's latest branch:

```sh
python3 scripts/sync-upstream.py v0.12.1
sh scripts/build-native.sh
go test -race ./...
# Review and commit the submodule pointer and generated version/POS metadata.
```

The sync script checks out the exact requested tag and updates binding metadata;
it does not copy or modify upstream files. It refuses a dirty upstream checkout.
The manual **Sync upstream tag** workflow tests the change against the matching
npm package and pushes an `upstream/vX.Y.Z` branch for review.

Go version tags follow upstream Garu version tags. All published versions from
`v0.9.14` through `v0.12.1` use this submodule layout, each pinning its matching
upstream source and model. Build the native library from the same tag/revision as
the Go dependency. An existing search encoder should keep the Garu version/model
used in training.

During this migration, the original `v0.9.14` and `v0.12.1` tags were rewritten.
Go module proxies/checksum databases can retain their previous contents; rewriting
Git tags cannot invalidate those caches. Consumers affected by that history should
use a new immutable version or a commit/pseudo-version.

## Verification

```sh
go test -race ./...
go vet ./...
go test -run '^$' -bench BenchmarkNouns -benchmem

# Compare with the actual matching JavaScript/WASM distribution:
npm install --prefix /tmp/garu-parity --ignore-scripts garu-ko@0.12.1
go build -o /tmp/garu-parity-cli ./cmd/parity
node scripts/check-parity.mjs /tmp/garu-parity/node_modules/garu-ko /tmp/garu-parity-cli
```

CI checks Linux, macOS and Alpine. Parity covers Korean search phrases, English SL tokens, spacing, empty input, emoji and embedded NUL.

## License

Wrapper code: Apache-2.0. Garu engine and model licensing are retained in the
upstream submodule: `upstream/LICENSE`, `upstream/NOTICE` (where present), and
`upstream/js/models/` notices, source licenses and provenance. No model data is
relicensed by this wrapper. Include the applicable notices when distributing a
compiled native library or executable; the Alpine example image carries them in
`/usr/local/share/garu/`.
