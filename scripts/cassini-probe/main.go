// Command cassini-probe checks a Cassini native build through the stock
// sherpa-onnx-go/v2 wrapper and this module's Go source:
//
//   - the runtime version must be the pinned Cassini version string;
//   - each Parakeet model directory (encoder/decoder/joiner .int8.onnx and
//     tokens.txt) must transcribe the public English sample at normal level
//     and attenuated by 60 dB (x0.001). Cassini's production Parakeet Ultra
//     export keeps the plain https://huggingface.co/moondream/parakeet-ultra
//     URL, which stock v2.0.1 rejects at load with an exit(-1); the fork
//     detects TDT from the joiner's output size instead;
//   - with -sortformer, the Nemotron-3-Diarization model must diarize the
//     -diar-wav clip through the wrapper's Segmentation.Sortformer member,
//     and with -diar-expect the turns must match a reference within
//     -diar-tolerance seconds per boundary.
//
// Usage: cassini-probe -wav WAV [-sortformer MODEL -diar-wav WAV [-diar-expect FILE]] MODEL_DIR...
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

const (
	wantVersion = "2.0.1+cassini.parakeet-tdt-joiner-v1.nemotron-diarization-v2"
	expected    = "ask what you can do for your country"
)

func readWav(path string) ([]float32, int) {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	sr := 0
	var pcm []float32
	for i := 12; i+8 <= len(b); {
		id := string(b[i : i+4])
		n := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		body := b[i+8 : min(len(b), i+8+n)]
		if id == "fmt " {
			sr = int(binary.LittleEndian.Uint32(body[4:8]))
			if binary.LittleEndian.Uint16(body[0:2]) != 1 || binary.LittleEndian.Uint16(body[2:4]) != 1 || binary.LittleEndian.Uint16(body[14:16]) != 16 {
				panic("need mono s16 PCM")
			}
		}
		if id == "data" {
			for j := 0; j+1 < len(body); j += 2 {
				pcm = append(pcm, float32(int16(binary.LittleEndian.Uint16(body[j:])))/32768)
			}
		}
		i += 8 + n + n%2
	}
	return pcm, sr
}

type turn struct {
	start, end float64
	speaker    int
}

func readTurns(path string) []turn {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var out []turn
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var t turn
		if _, err := fmt.Sscanf(line, "%f %f %d", &t.start, &t.end, &t.speaker); err != nil {
			panic(fmt.Sprintf("%s: %q: %v", path, line, err))
		}
		out = append(out, t)
	}
	return out
}

func transcribe(dir string, pcm []float32, sr int) bool {
	cfg := sherpa.OfflineRecognizerConfig{}
	cfg.FeatConfig.SampleRate = 16000
	cfg.FeatConfig.FeatureDim = 128
	cfg.ModelConfig.Transducer.Encoder = dir + "/encoder.int8.onnx"
	cfg.ModelConfig.Transducer.Decoder = dir + "/decoder.int8.onnx"
	cfg.ModelConfig.Transducer.Joiner = dir + "/joiner.int8.onnx"
	cfg.ModelConfig.Tokens = dir + "/tokens.txt"
	cfg.ModelConfig.ModelType = "nemo_transducer"
	cfg.ModelConfig.NumThreads = 4
	cfg.ModelConfig.Provider = "cpu"
	cfg.DecodingMethod = "greedy_search"
	r := sherpa.NewOfflineRecognizer(&cfg)
	if r == nil {
		fmt.Printf("FAIL: %s: recognizer creation failed\n", dir)
		return false
	}
	defer sherpa.DeleteOfflineRecognizer(r)
	ok := true
	for _, gain := range []float32{1, 0.001} {
		x := make([]float32, len(pcm))
		for i, v := range pcm {
			x[i] = v * gain
		}
		s := sherpa.NewOfflineStream(r)
		s.AcceptWaveform(sr, x)
		r.Decode(s)
		res := s.GetResult()
		text := strings.TrimSpace(res.Text)
		fmt.Printf("asr %s gain=%g tokens=%d text=%q\n", dir, gain, len(res.Tokens), text)
		if !strings.Contains(strings.ToLower(text), expected) {
			fmt.Printf("FAIL: %s gain=%g did not transcribe the sample\n", dir, gain)
			ok = false
		}
		sherpa.DeleteOfflineStream(s)
	}
	return ok
}

func diarize(model, wav, expectPath string, tol float64) bool {
	pcm, sr := readWav(wav)
	cfg := sherpa.OfflineSpeakerDiarizationConfig{}
	cfg.Segmentation.Sortformer.Model = model
	cfg.Segmentation.NumThreads = 4
	cfg.Segmentation.Provider = "cpu"
	sd := sherpa.NewOfflineSpeakerDiarization(&cfg)
	if sd == nil {
		fmt.Printf("FAIL: %s: diarizer creation failed through Segmentation.Sortformer\n", model)
		return false
	}
	defer sherpa.DeleteOfflineSpeakerDiarization(sd)
	if sd.SampleRate() != sr {
		fmt.Printf("FAIL: diarizer sample rate %d != wav %d\n", sd.SampleRate(), sr)
		return false
	}
	segs := sd.Process(pcm)
	speakers := map[int]bool{}
	var got []turn
	for _, s := range segs {
		fmt.Printf("diar %.3f %.3f %d\n", s.Start, s.End, s.Speaker)
		got = append(got, turn{float64(s.Start), float64(s.End), s.Speaker})
		speakers[s.Speaker] = true
	}
	fmt.Printf("diar wav=%s samples=%d turns=%d speakers=%d\n", wav, len(pcm), len(got), len(speakers))
	if len(got) == 0 {
		fmt.Println("FAIL: diarization returned no turns")
		return false
	}
	if expectPath == "" {
		return true
	}
	want := readTurns(expectPath)
	if len(want) != len(got) {
		fmt.Printf("FAIL: diarization gave %d turns, reference has %d\n", len(got), len(want))
		return false
	}
	ok := true
	maxd := 0.0
	for i := range want {
		d := math.Max(math.Abs(want[i].start-got[i].start), math.Abs(want[i].end-got[i].end))
		maxd = math.Max(maxd, d)
		if want[i].speaker != got[i].speaker || d > tol {
			fmt.Printf("FAIL: turn %d: got %.3f %.3f %d, reference %.3f %.3f %d\n", i,
				got[i].start, got[i].end, got[i].speaker, want[i].start, want[i].end, want[i].speaker)
			ok = false
		}
	}
	fmt.Printf("diar matches reference: %v (max boundary difference %.3f s, tolerance %.3f s)\n", ok, maxd, tol)
	return ok
}

func main() {
	wav := flag.String("wav", "", "ASR sample (mono 16-bit PCM)")
	sortformer := flag.String("sortformer", "", "Nemotron-3-Diarization ONNX model")
	diarWav := flag.String("diar-wav", "", "diarization sample (mono 16-bit PCM, 16 kHz)")
	diarExpect := flag.String("diar-expect", "", "reference turns, one 'start end speaker' per line")
	diarTol := flag.Float64("diar-tolerance", 0.08, "allowed boundary difference in seconds")
	flag.Parse()
	dirs := flag.Args()
	if *wav == "" && len(dirs) > 0 || *sortformer != "" && *diarWav == "" {
		flag.Usage()
		os.Exit(2)
	}
	version := sherpa.GetVersion()
	fmt.Printf("sherpa version=%s gitsha=%s\n", version, sherpa.GetGitSha1())
	failed := false
	if version != wantVersion {
		fmt.Printf("FAIL: version %q, want %q\n", version, wantVersion)
		failed = true
	}
	if len(dirs) > 0 {
		pcm, sr := readWav(*wav)
		fmt.Printf("asr wav=%s sr=%d samples=%d\n", *wav, sr, len(pcm))
		for _, d := range dirs {
			if !transcribe(d, pcm, sr) {
				failed = true
			}
		}
	}
	if *sortformer != "" && !diarize(*sortformer, *diarWav, *diarExpect, *diarTol) {
		failed = true
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("PASS")
}
