# Cassini release maintenance

This fork follows
[codemyriad/sherpa-onnx-go-linux](https://github.com/codemyriad/sherpa-onnx-go-linux).
Each Cassini line is a branch named after the upstream release it starts from,
and every release is an immutable tag on it. Never move a published tag:
publish a new version and update the consumer's go.sum.

| Branch | Upstream base | Tags | Native source |
|---|---|---|---|
| `v2.0.1-cassini` | `v2.0.1` | `v2.0.1-cassini.N` | codemyriad/sherpa-onnx `v2.0.1-cassini-1` |
| `v1.13.7-cassini` | `v1.13.7` | `v1.13.7-cassini.8` | codemyriad/sherpa-onnx `v1.13.7-cassini-parakeet-ultra-v1` |

The module path stays upstream's (`github.com/k2-fsa/sherpa-onnx-go-macos/v2`
on the v2 line). Consumers keep the unmodified
`github.com/k2-fsa/sherpa-onnx-go/v2 v2.0.1` wrapper and redirect the platform
module with a `replace` directive:

```
replace github.com/k2-fsa/sherpa-onnx-go-macos/v2 => github.com/codemyriad/sherpa-onnx-go-macos/v2 v2.0.1-cassini.2
```

## v2.0.1-cassini.2

v2.0.1-cassini.1 with upstream's C++ wrapper library
(`lib/*/libsherpa-onnx-cxx-api.dylib`) removed. The C API library, ONNX
Runtime, `sherpa_onnx.go` and `c-api.h` are byte-identical to v2.0.1-cassini.1.

That wrapper was upstream's stock v2.0.1 build, compiled against upstream's C
header. #4032 appends the `sortformer` member to
`SherpaOnnxOfflineSpeakerSegmentationModelConfig`, so the stock wrapper and this
C API disagree on the layout of the whole speaker diarization config (on 64-bit
the struct grows from 80 to 96 bytes and the embedding config moves from offset
32 to 48). `OfflineSpeakerDiarization::Create` through it would hand the C API
the embedding model path as `segmentation.sortformer.model` and every later
field at the wrong offset, which breaks ordinary Pyannote diarization as well as
Sortformer. Nothing in Go links the C++ wrapper (`build_darwin_*.go` link only
`sherpa-onnx-c-api` and `onnxruntime`), so the module now ships only the two
libraries it uses. C++ consumers must build `sherpa-onnx-cxx-api` from the
source commit above.

`scripts/cassini-check-module.sh` (run it from the repository root before
tagging) fails if `lib/` holds anything but the C API library and ONNX
Runtime per architecture, or if `CASSINI-SHA256SUMS` does not list exactly those
files with matching checksums.

## v2.0.1-cassini.1

Upstream `v2.0.1` with:

- `libsherpa-onnx-c-api.dylib` for arm64 and x86_64 built from
  [`c810dfcf409b2a72a13e9f0ac961ff55473e350d`](https://github.com/codemyriad/sherpa-onnx/commit/c810dfcf409b2a72a13e9f0ac961ff55473e350d)
  (annotated tag `v2.0.1-cassini-1`: upstream v2.0.1 plus k2-fsa/sherpa-onnx#4032
  @6a4ad68b, NeMo TDT detection from the joiner output size, and the Cassini
  runtime markers). It reports
  `2.0.1+cassini.parakeet-tdt-joiner-v1.nemotron-diarization-v2`:
  - `.parakeet-tdt-joiner-v1`: a NeMo transducer whose joiner has more outputs
    than the vocabulary is decoded as TDT, so Parakeet Ultra exports that keep
    the plain `https://huggingface.co/moondream/parakeet-ultra` URL load
    (stock v2.0.1 exits the process at load with `vocab_size: 8193 !=
    output_size: 8198`);
  - `.nemotron-diarization-v2`: NVIDIA Nemotron-3-Diarization (interface
    version 2) through `OfflineSpeakerDiarizationConfig.Segmentation.Sortformer`.
  The quiet-audio fix for Parakeet comes from upstream #3999 (in v2.0.1); the
  1.13.7 line's reference frontend and its `Segmentation.Pyannote.Model` slot
  for Sortformer are gone.
- `sherpa_onnx.go` and `c-api.h` copied from the same source commit
  (`scripts/go/sherpa_onnx.go`, `sherpa-onnx/c-api/c-api.h`). They add
  `OfflineSpeakerSegmentationSortformerModelConfig` and the `Sortformer`
  member at the end of `OfflineSpeakerSegmentationModelConfig`, which changes
  the C layout of the diarization config. Do not use this Go source with
  upstream's library.

ONNX Runtime (1.28.2, the archive upstream v2.0.1 pins in `cmake/onnxruntime-osx-{arm64,x86_64}.cmake`) is upstream
v2.0.1's, unchanged. The workflow downloads the same pinned archive and compares it with the committed `libonnxruntime.dylib`: byte-identical on both architectures (arm64 `b0613d0a…`, x86_64 `2bf27180…`).
v2.0.1-cassini.1 also kept upstream's `libsherpa-onnx-cxx-api.dylib`; v2.0.1-cassini.2
removes it (see below).

As in the Linux builds, text-to-speech is compiled out: the C API keeps its TTS
entry points, which report that TTS is unavailable, so the wrapper still links.

`.github/workflows/cassini-native-macos.yml` builds both libraries on the
GitHub `macos-15-intel` runner with `scripts/build-native.sh`, passing
`CMAKE_OSX_ARCHITECTURES` and upstream's deployment targets (11.0 arm64, 10.15
x86_64): **x86_64 natively, arm64 cross-compiled by Apple clang on Intel**
(sherpa-onnx's cmake then pins the `osx-arm64` ONNX Runtime). Hosted Apple
silicon runners (`macos-14`, `macos-latest`) were not acquired on 2026-10-28
(eight jobs cancelled for capacity), so the arm64 library was not built or run
on Apple silicon. The job then sets upstream's install name
`@rpath/libsherpa-onnx-c-api.dylib`, keeps `@loader_path` as the only rpath
and re-signs the library ad hoc. `scripts/cassini-check-dylib.sh` checks, for
both architectures: `lipo` architecture, `codesign --verify`, the dependencies
(libSystem, libc++, `@rpath/libonnxruntime.dylib`), install name, rpath,
deployment target, the runtime version string, the exported C entry points and
the `OfflineSpeakerSegmentationSortformerModel` symbols.

On x86_64 the job builds `scripts/cassini-probe` against this checkout through
the stock `sherpa-onnx-go/v2` wrapper and runs:

- the version check (the exact string above);
- the public English sample (`en.wav`) at normal level and multiplied by
  `0.001`, with Cassini's production Parakeet v3 int8 and Parakeet Ultra int8
  bundles from dist.gocassini.com (the Ultra encoder keeps the plain Moondream
  URL); every transcript must contain the full sentence;
- Nemotron-3-Diarization int8 (dist.gocassini.com, sha256 `47c221ea…`) on
  sherpa-onnx's public `0-four-speakers-zh.wav` through
  `Segmentation.Sortformer`; the turns must match
  `scripts/cassini-probe/four-speakers-zh.turns` (9 turns, 4 speakers, from the
  Linux x86_64 build of the same source) within 80 ms per boundary.

For arm64 the same probe program is cross-compiled and linked against the
arm64 library (`GOARCH=arm64`, `CC="clang -arch arm64"`), which proves every C
symbol the wrapper uses resolves, but it is not run.

Dispatching the workflow with `mode=probe-arm64` on a tag runs the full probe
above on an Apple silicon runner against the committed arm64 library (after
checking it against `CASSINI-SHA256SUMS`).

### Results for v2.0.1-cassini.1 (run 40732315259)

| | x86_64 (native, `macos-15-intel`) | arm64 (cross-built on `macos-15-intel`) |
|---|---|---|
| `libsherpa-onnx-c-api.dylib` sha256 | `ea9b85c4…` | `e6ef7c70…` |
| architecture, ad-hoc signature, deps, install name, rpath | pass | pass |
| deployment target | 10.15 | 11.0 |
| runtime version string in the library | pass | pass |
| Sortformer symbols present | 22 | 22 |
| Go probe builds and links | pass | pass (cross-linked, `GOARCH=arm64`) |
| version via `GetVersion()` | pass | **not run** |
| `en.wav` v3 int8, gain 1 / 0.001 | 27 / 27 tokens, full sentence | **not run** |
| `en.wav` Ultra int8 (plain URL), gain 1 / 0.001 | 27 / 27 tokens, full sentence | **not run** |
| Sortformer, `0-four-speakers-zh.wav` | 9 turns, 4 speakers, max boundary diff 0.010 s | **not run** |

**The arm64 library has not been executed on Apple silicon.** Its source,
flags and ONNX Runtime are the same as x86_64's and every structural check
passes, but treat it as unverified at runtime until a `mode=probe-arm64` run
passes; record that run here when it does.

The release libraries come from run
[40732315259](https://github.com/codemyriad/sherpa-onnx-go-macos/actions/runs/40732315259).
Run the workflow from the Actions tab to rebuild; it uploads the library, build
info, `otool` output, ONNX Runtime checksums and probe log as artifacts.

`CASSINI-SHA256SUMS` records the released libraries. Verify with
`shasum -a 256 -c CASSINI-SHA256SUMS` (or `sha256sum -c`) from the repository
root.
