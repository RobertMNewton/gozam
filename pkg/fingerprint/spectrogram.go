package fingerprint

import (
	"math/cmplx"

	"github.com/go-audio/audio"
	"github.com/mjibson/go-dsp/fft"
)

type Spectrogram [][]float64

func GetSpectrogram(audioBuff audio.Buffer, binSize, overlap int) Spectrogram {
	buff := audioBuff.AsFloatBuffer()

	numFrames := len(buff.Data)
	stepSize := binSize - overlap

	numChunks := (numFrames - overlap + stepSize - 1) / stepSize
	if numChunks < 0 {
		numChunks = 0
	}

	spectrogram := make(Spectrogram, 0, numChunks)
	for _, audioBin := range chunkAndNormaliseAudio(buff, binSize, overlap) {
		spectrogram = append(spectrogram, complex128ArrToMagArr(
			fft.FFT(
				float64ArrToComplex128Arr(audioBin),
			),
		))
	}
	return spectrogram
}

func float64ArrToComplex128Arr(arr []float64) []complex128 {
	res := make([]complex128, len(arr))
	for i, val := range arr {
		res[i] = complex(val, 0)
	}
	return res
}

func complex128ArrToMagArr(arr []complex128) []float64 {
	averaging_window_size := 5

	res := make([]float64, len(arr)/averaging_window_size+1)
	for i, c := range arr {
		bin := i / averaging_window_size
		res[bin] += cmplx.Abs(c)

		if bin%averaging_window_size == averaging_window_size-1 {
			res[bin] /= float64(averaging_window_size)
		}
	}

	return res
}

func chunkAndNormaliseAudio(audioBuff audio.Buffer, binSize, overlap int) [][]float64 {
	res := make([][]float64, 0, audioBuff.NumFrames()/(binSize-overlap))

	pcm, stepSize := audioBuff.AsFloatBuffer().Data, binSize-overlap
	for i := 0; i < len(pcm); i += stepSize {
		j := min(i+binSize, len(pcm)-1)
		res = append(res, pcm[i:j])
	}

	return res
}
