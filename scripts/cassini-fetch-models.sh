#!/usr/bin/env bash
# Fetch the pinned production models and public samples the Cassini probe uses.
# Usage: cassini-fetch-models.sh MODELS_DIR
set -euo pipefail
MODELS=${1:?usage: cassini-fetch-models.sh MODELS_DIR}
command -v zstd >/dev/null || brew install zstd
fetch() { curl --fail --location --retry 3 -o "$2" "$1"; echo "$3  $2" | shasum -a 256 -c -; }
# dist.gocassini.com artifacts are seekable zstd (standard frames plus a
# skippable seek table); check the decompressed file against the catalogue.
fetchz() { curl --fail --location --retry 3 -o "$2.zst" "$1"; zstd -q -d --rm -f "$2.zst" -o "$2"; echo "$3  $2" | shasum -a 256 -c -; }
dist=https://dist.gocassini.com/models/files
hf=https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/resolve/2bda32ec70b097a55adaa07d9a7173915b43cc78
mkdir -p "$MODELS/v3-int8" "$MODELS/ultra-int8" "$MODELS/diar"
fetch $hf/test_wavs/en.wav "$MODELS/en.wav" 148b936b43ce7c546a866e64da059f0458aee2d65e617f16e9d94f06e8d99ed6
fetch https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/0-four-speakers-zh.wav \
  "$MODELS/four-speakers-zh.wav" bedf036caed208386c67b4ef4b11f83d74dd0d420b102163a1c33cd09cde7010
# parakeet-tdt-0.6b-v3-int8 (Cassini catalogue before Ultra replaced it).
fetchz $dist/cb9e98257d3bc2611b43d9897e67454e5893aff1ffc90229ca3fa7f9ed5bb22e/encoder.int8.onnx.zst "$MODELS/v3-int8/encoder.int8.onnx" acfc2b4456377e15d04f0243af540b7fe7c992f8d898d751cf134c3a55fd2247
fetchz $dist/b14c3967107f0a5e4e9c0bde770c204202bde8d1a6255c27a863930e8c887c3d/decoder.int8.onnx.zst "$MODELS/v3-int8/decoder.int8.onnx" 179e50c43d1a9de79c8a24149a2f9bac6eb5981823f2a2ed88d655b24248db4e
fetchz $dist/1ef3c78137d39d45cfad961be5a45bf7abd3c13fa721c7da2327b0eb3d0e8ce6/joiner.int8.onnx.zst "$MODELS/v3-int8/joiner.int8.onnx" 3164c13fc2821009440d20fcb5fdc78bff28b4db2f8d0f0b329101719c0948b3
fetchz $dist/5b3811f0c23fefc86835981f0a23ecfb817999df272ad744c597e5b669ea1b4a/tokens.txt.zst "$MODELS/v3-int8/tokens.txt" d58544679ea4bc6ac563d1f545eb7d474bd6cfa467f0a6e2c1dc1c7d37e3c35d
# parakeet-ultra-0.6b-int8 production bundle: encoder url is the plain
# https://huggingface.co/moondream/parakeet-ultra, which stock v2.0.1 rejects.
fetchz $dist/caf7a38f2ad3224c1863564fb6e975a11aa3ae6ad1e7ad0bee5c4f5e018babb4/encoder.int8.onnx.zst "$MODELS/ultra-int8/encoder.int8.onnx" 299617fefd4664d1c115b691dcb6413cabcdbcc694cdf3cb4464b72c5fc3ae69
fetchz $dist/69e2e91d01225bc552f8b0113630562fc52622cb2a3c61ef4445ee779f06b295/decoder.int8.onnx.zst "$MODELS/ultra-int8/decoder.int8.onnx" 1fab98fe6c12aded87d2da66272cc9e148d0a0044ce3850a12fe56302ec4a922
fetchz $dist/639fda62a71242a8d7cefb6647f2094eb8ac783925ce91cbd047a0de6988f396/joiner.int8.onnx.zst "$MODELS/ultra-int8/joiner.int8.onnx" 8a71aaccfdba3d451775507a889afd2257d2e754463fd81095a8c8f290f924c2
fetchz $dist/5b3811f0c23fefc86835981f0a23ecfb817999df272ad744c597e5b669ea1b4a/tokens.txt.zst "$MODELS/ultra-int8/tokens.txt" d58544679ea4bc6ac563d1f545eb7d474bd6cfa467f0a6e2c1dc1c7d37e3c35d
grep -aq 'https://huggingface.co/moondream/parakeet-ultra' "$MODELS/ultra-int8/encoder.int8.onnx"
# nemotron-3-diarization-int8 (interface version 2).
fetchz $dist/794720a59892189484d38a186d2b350018b4194cae55292f501b2849d6313290/model.int8.onnx.zst "$MODELS/diar/model.int8.onnx" 47c221ea9b4d4e7f6c108bd098e769bc706cdc986c29f6133c93cae335fe6779
