#!/usr/bin/env bash
# Structural checks on a Cassini macOS C API library, valid on any macOS host
# whatever the library's architecture.
# Usage: cassini-check-dylib.sh LIB ARCH MINOS RUNTIME_VERSION
set -euo pipefail
lib=$1; arch=$2; minos_want=$3; runtime=$4
file "$lib"
lipo -info "$lib"
lipo -archs "$lib" | grep -qx "$arch"
codesign --verify --verbose "$lib"
otool -L "$lib"
# Same dependencies, install name, rpath and deployment target as upstream v2.0.1's library.
deps=$(otool -L "$lib" | tail -n +2 | awk '{print $1}' | sort | tr '\n' ' ')
test "$deps" = "/usr/lib/libSystem.B.dylib /usr/lib/libc++.1.dylib @rpath/libonnxruntime.dylib @rpath/libsherpa-onnx-c-api.dylib "
test "$(otool -D "$lib" | tail -n 1)" = @rpath/libsherpa-onnx-c-api.dylib
rpaths=$(otool -l "$lib" | awk '/cmd LC_RPATH/{getline; getline; print $2}' | tr '\n' ' ')
test "$rpaths" = "@loader_path "
minos=$(otool -l "$lib" | awk '/cmd LC_BUILD_VERSION/{f=1} f&&/minos/{print $2; exit}')
echo "minos $minos"
test "$minos" = "$minos_want"
strings -a "$lib" | grep -F "$runtime"
# The C entry points the Go wrapper's diarization path calls, and the
# Sortformer segmentation model behind Segmentation.Sortformer (#4032).
exported=$(nm -gU "$lib")
syms=$(nm "$lib")
for s in SherpaOnnxGetVersionStr SherpaOnnxCreateOfflineSpeakerDiarization \
  SherpaOnnxOfflineSpeakerDiarizationProcess SherpaOnnxCreateOfflineRecognizer; do
  grep -q " _$s\$" <<<"$exported" || { echo "missing exported symbol $s" >&2; exit 1; }
done
n=$(grep -c Sortformer <<<"$syms" || true)
echo "Sortformer symbols: $n"
test "$n" -gt 0
c++filt <<<"$syms" | grep 'OfflineSpeakerSegmentationSortformerModel::' | head -n 5 || true
