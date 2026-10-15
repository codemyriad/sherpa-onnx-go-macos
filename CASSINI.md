# Cassini release maintenance

This fork follows
[codemyriad/sherpa-onnx-go-linux](https://github.com/codemyriad/sherpa-onnx-go-linux):
the `v1.13.7-cassini` branch is upstream `v1.13.7` with Cassini's native C API
library, and every release is an immutable `v1.13.7-cassini.N` tag. Never move
a published tag: publish a new version and update the consumer's go.sum.
Consumers pair it with the unmodified `github.com/k2-fsa/sherpa-onnx-go v1.13.7`
wrapper through a `replace` directive.

`v1.13.7-cassini.8` (the first macOS release; its number matches the Linux
module) replaces only `libsherpa-onnx-c-api.dylib` for arm64 and x86_64. It is
built from
[`0c2ee3f3f5f16bb06d234a0f98632810296ba0eb`](https://github.com/codemyriad/sherpa-onnx/commit/0c2ee3f3f5f16bb06d234a0f98632810296ba0eb)
(`v1.13.7-cassini-parakeet-ultra-v1` source tag), the same source as the
Linux `.7`/`.8` libraries, and reports
`1.13.7+cassini-parakeet-v3-reference-v1.nemotron-diarization-v2.parakeet-ultra-v1`:

- the Parakeet v3 reference frontend (`+cassini-parakeet-v3-reference-v1`);
- NVIDIA Nemotron-3-Diarization through the stock `OfflineSpeakerDiarization`
  API with `Segmentation.Pyannote.Model` (`.nemotron-diarization-v2`);
- reference-frontend and TDT selection for Moondream Parakeet Ultra exports
  that keep their own model URL (`.parakeet-ultra-v1`).

As in the Linux builds, text-to-speech is compiled out: the C API keeps its TTS
entry points, which report that TTS is unavailable, so the stock wrapper still
links (the libraries are about 1 MB smaller than upstream's).
ONNX Runtime (1.28.1), the C++ wrapper libraries, the Go source and the C header
are upstream v1.13.7's, unchanged. The ONNX Runtime archive pinned by the
source's CMake files is byte-identical to upstream's `libonnxruntime.dylib` on
both architectures.

`.github/workflows/cassini-native-macos.yml` builds the library natively on
GitHub `macos-14` (arm64) and `macos-15-intel` (x86_64) runners with
`scripts/build-native.sh` (byte-identical to sherpa-onnx-go-linux's), passing
`CMAKE_OSX_ARCHITECTURES` and upstream's deployment targets (11.0 arm64, 10.15
x86_64). It then sets upstream's install name `@rpath/libsherpa-onnx-c-api.dylib`,
keeps `@loader_path` as the only rpath and re-signs the library ad hoc. The same
job checks the dependencies (libSystem, libc++, `@rpath/libonnxruntime.dylib`),
the rpath, the deployment target and the runtime version, then builds
`scripts/cassini-probe` against the library through the stock Go wrapper and
decodes the public English sample at normal level and multiplied by `0.001`
with Parakeet v3 int8 and the tpmaerk Parakeet Ultra int8 export with its
original metadata. All transcripts on both runners are the full sentence (on
Linux, the attenuated Ultra case decodes to nothing with the legacy frontend).
The release
libraries come from run
[38670697737](https://github.com/codemyriad/sherpa-onnx-go-macos/actions/runs/38670697737).
Run the workflow from the Actions tab to rebuild; it uploads the library, build
info, `otool` output and probe log as artifacts.

`CASSINI-SHA256SUMS` records the released libraries. Verify with
`sha256sum -c CASSINI-SHA256SUMS` (or `shasum -a 256 -c`) from the repository root.
