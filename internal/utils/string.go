package utils

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

func AddCarriageReturnAfterNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", "\n\r")
}

func MinMaxPossibleRuneCount(byteCount int) (int, int) {
	minPossibleRuneCount := byteCount / 4 //4 is the maximum number of bytes for a single character in UTF-8
	maxPossibleRuneCount := byteCount

	return minPossibleRuneCount, maxPossibleRuneCount
}

func FindLongestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}

	if len(strs) == 1 {
		return strs[0]
	}

	var runeSlices [][]rune
	for _, s := range strs {
		runeSlices = append(runeSlices, []rune(s))
	}

	var prefix []rune
	for i := 0; i < len(runeSlices[0]); i++ {
		for j := 1; j < len(runeSlices); j++ {
			if i < len(runeSlices[j]) && runeSlices[j][i] == runeSlices[0][i] {
				continue
			} else {
				return string(prefix)
			}
		}
		prefix = append(prefix, runeSlices[0][i])
	}

	return string(prefix)
}

type ClosestSearch[T any] struct {
	MaxDifferences           int
	Target                   T
	GetSourceI               func(i int) (source T, ok bool)
	OptionalFilter           func(source T, target T) bool
	ComputeLevenshteinMatrix func(source T, target T) [][]int
	Context                  context.Context
}

func FindClosest[T any](search ClosestSearch[T]) (sourceIndex int, minDistance int) {
	i := -1
	sourceIndex = -1
	minDistance = math.MaxInt

	lastContextCheck := time.Now()

	for {
		i++
		src, ok := search.GetSourceI(i)
		if !ok {
			break
		}

		matrix := search.ComputeLevenshteinMatrix(src, search.Target)
		distance := levenshtein.DistanceForMatrix(matrix)

		if distance < minDistance {
			minDistance = distance
			sourceIndex = i
		}

		now := time.Now()

		if search.Context != nil && now.Sub(lastContextCheck) > time.Millisecond {
			select {
			case <-search.Context.Done():
				panic(search.Context.Err())
			default:
			}
			lastContextCheck = now
		}
	}

	return
}

func FindClosestString(ctx context.Context, candidates []string, v string, maxDifferences int) (string, int, bool) {
	index, distance := FindClosest(ClosestSearch[[]rune]{
		Context:        ctx,
		MaxDifferences: maxDifferences,
		Target:         []rune(v),
		GetSourceI: func(i int) (sourceIndex []rune, ok bool) {
			if i >= len(candidates) {
				return nil, false
			}
			return []rune(candidates[i]), true
		},
		ComputeLevenshteinMatrix: func(source, target []rune) [][]int {
			return levenshtein.MatrixForStrings(source, target, levenshtein.Options{
				InsCost: 1,
				DelCost: 1,
				SubCost: 1,
				Matches: levenshtein.IdenticalRunes,
			})
		},
	})
	if index >= 0 {
		return candidates[index], distance, true
	}
	return "", -1, false
}

// FindDoubleLineSequence returns the index of a double line sequence (see further), and the sequence's length in bytes.
// If no double lines sequence is present, the index will be negative. A double line sequence is one of the following
// sequences: \r\r, \n\n, \r\n\n, \n\r\n, \r\n\r\n
func FindDoubleLineSequence(bytes []byte) (index int, length int) {
	nlCount := 0
	crCount := 0
	startIndex := -1

	for i, b := range bytes {
		switch b {
		case '\r':
			if startIndex < 0 {
				startIndex = i
				crCount++
				continue
			}
			crCount++
			if crCount == 2 {
				return startIndex, i - startIndex + 1
			}
		case '\n':
			if startIndex < 0 {
				startIndex = i
				nlCount++
				continue
			}
			nlCount++
			if nlCount == 2 {
				return startIndex, i - startIndex + 1
			}
		default:
			startIndex = -1
			crCount = 0
			nlCount = 0
			continue
		}
	}

	return -1, 0
}
