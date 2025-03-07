// module for computing audio "fingerprints"
package fingerprint

import (
	"container/heap"
	"fmt"

	"github.com/go-audio/audio"
)

type Token struct {
	Time, Freq int
	Amp        float64
}

type TokenPairHash int64

// PriorityQueue implements a max-heap
type PriorityQueue []Token

// Len is the number of elements in the collection.
func (pq PriorityQueue) Len() int { return len(pq) }

// Less reports whether the element with index i should sort before the element with index j.
func (pq PriorityQueue) Less(i, j int) bool {
	// Max-heap: larger values are "smaller" so they come first
	return pq[i].Amp < pq[j].Amp
}

// Swap swaps the elements with indexes i and j.
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

// Push adds an element to the heap.
func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(Token))
}

// Pop removes and returns the smallest element from the heap.
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	x := old[n-1]
	*pq = old[0 : n-1]
	return x
}

func ComputeTokenPairHash(t1, t2 Token) TokenPairHash {
	return TokenPairHash(t1.Freq * absInt(t1.Freq-t2.Freq) * absInt(t2.Time-t1.Time))
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func iterNbhd[T any](mat [][]T, i0, j0 int, ri, rj int, condition func(T) bool) bool {
	for i := i0 - ri; i <= i0+ri; i++ {
		if i < 0 || i >= len(mat) {
			continue
		}

		for j := j0 - rj; j <= j0+rj; j++ {
			if i == i0 && j == j0 {
				continue
			}

			if j < 0 || j >= len(mat[i]) {
				continue
			}

			// Check the condition for the current neighbor
			if condition(mat[i][j]) {
				return false // If the condition is met (neighbor >= current), it's not a peak
			}
		}
	}
	return true // If no neighbor met the condition, it's a peak
}

func findPeaks(spectrogram Spectrogram, topN int) []Token {
	var peaks []Token
	pq := make(PriorityQueue, 0, topN)
	heap.Init(&pq)

	for t := 0; t < len(spectrogram); t++ {
		for f := 0; f < len(spectrogram[t]); f++ {
			isPeak := iterNbhd(spectrogram, t, f, 0, 100, func(nbhrAmp float64) bool {
				return nbhrAmp >= spectrogram[t][f]
			})

			if isPeak {
				newPeak := Token{t, f, spectrogram[t][f]}

				// If the heap is not full, push the new peak
				if pq.Len() < topN {
					heap.Push(&pq, newPeak)
				} else if newPeak.Amp > pq[0].Amp {
					// If the new peak is larger than the smallest peak in the heap, replace it
					heap.Pop(&pq)
					heap.Push(&pq, newPeak)
				}
			}
		}
	}

	// Convert the priority queue to a slice (sorted order by default)
	for pq.Len() > 0 {
		peaks = append(peaks, heap.Pop(&pq).(Token))
	}

	return peaks
}

type Fingerprint struct {
	Tokens []Token
	Hashes map[TokenPairHash]struct{}
}

func GetFingerPrint(audioBuff audio.Buffer, binSize, overlap int, hashTopN int, maxTokenTimeDiff int) Fingerprint {
	spectrogram := GetSpectrogram(audioBuff, binSize, overlap)

	fp := Fingerprint{
		Tokens: findPeaks(spectrogram, hashTopN),
		Hashes: make(map[TokenPairHash]struct{}),
	}

	//fmt.Printf("found peaks successfully. There are %d peak tokens. %v\n", len(fp.Tokens), fp.Tokens[:100])

	for i, t1 := range fp.Tokens {
		for _, t2 := range fp.Tokens[i:] {
			if t2.Time-t1.Time > maxTokenTimeDiff {
				break
			}

			fp.Hashes[ComputeTokenPairHash(t1, t2)] = struct{}{}
		}
	}

	return fp
}

func GetFingerPrint2(audioBuff audio.Buffer, binSize, overlap int, hashTopN int, maxTokenTimeDiff int, tokenPerWindow int) Fingerprint {
	spectrogram := GetSpectrogram(audioBuff, binSize, overlap)

	findPeaks2 := func() []Token {
		const (
			SIZE_FREQ_WINDOWS int = 200
			SIZE_TIME_WINDOWS int = 10
		)

		peaks := make([]Token, 0, tokenPerWindow*(len(spectrogram[0])/SIZE_FREQ_WINDOWS)*(len(spectrogram)/SIZE_TIME_WINDOWS))

		//fmt.Printf("%d, %d, %d", len(spectrogram)/SIZE_TIME_WINDOWS, len(spectrogram[0])/SIZE_FREQ_WINDOWS, len(spectrogram[0]))

		for tw := 0; tw < len(spectrogram)/SIZE_TIME_WINDOWS; tw++ {
			t0, t1 := tw*SIZE_TIME_WINDOWS, min((tw+1)*SIZE_TIME_WINDOWS, len(spectrogram)-1)
			for fw := 0; fw < len(spectrogram[0])/SIZE_FREQ_WINDOWS; fw++ {
				f0, f1 := fw*SIZE_FREQ_WINDOWS, min((fw+1)*SIZE_FREQ_WINDOWS, len(spectrogram[0])-1)

				pq := make(PriorityQueue, 0, tokenPerWindow)
				heap.Init(&pq)

				for i := t0; i < t1; i++ {
					for j := f0; j < f1; j++ {
						token := Token{i, j, spectrogram[i][j]}
						if len(pq) < tokenPerWindow {
							heap.Push(&pq, token)
						} else if token.Amp > pq[0].Amp {
							heap.Pop(&pq)
							heap.Push(&pq, token)
						}
					}
				}

				for pq.Len() > 0 {
					peaks = append(peaks, heap.Pop(&pq).(Token))
				}
			}
		}

		return peaks
	}

	fp := Fingerprint{
		Tokens: findPeaks2(),
		Hashes: make(map[TokenPairHash]struct{}),
	}

	fmt.Printf("found peaks successfully. There are %d peak tokens. %v\n", len(fp.Tokens), fp.Tokens[:10])

	for i, t1 := range fp.Tokens {
		for _, t2 := range fp.Tokens[i:] {
			if absInt(t2.Time-t1.Time) > maxTokenTimeDiff {
				break
			}

			if t1.Freq == t2.Freq {
				continue
			}

			fp.Hashes[ComputeTokenPairHash(t1, t2)] = struct{}{}
		}
	}

	return fp
}
