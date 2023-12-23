package utils

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/third_party_stable/golang-levenshtein/levenshtein"
	"golang.org/x/exp/constraints"
)

var (
	MATCHALL_REGEX = regexp.MustCompile(".*")
)

func AddCarriageReturnAfterNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", "\n\r")
}

func IndentLines(s string, indent string) string {
	lines := strings.Split(s, "\n")

	for lineIndex, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		//ignore carriage returns
		injectionIndex := 0
		for i := 0; i < len(line); i++ {
			b := line[i]
			if b == '\r' {
				continue
			}
			injectionIndex = i
			break
		}

		lines[lineIndex] = line[:injectionIndex] + indent + line[injectionIndex:]
	}

	return strings.Join(lines, "\n")
}

func MinMaxPossibleRuneCount(byteCount int) (int, int) {
	//4 is the maximum number of bytes for a single character in UTF-8
	minPossibleRuneCount := byteCount / 4
	if (byteCount % 4) > 0 {
		minPossibleRuneCount++
	}

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
	IsRelevant               func(candidate T, distance int) bool //can be nil
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

		if distance < minDistance &&
			distance <= search.MaxDifferences &&
			(search.IsRelevant == nil || search.IsRelevant(src, distance)) {
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

	if maxDifferences <= 0 {
		panic(fmt.Errorf("invalid maxDifferences argument: %#v", maxDifferences))
	}

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
		IsRelevant: func(candidate []rune, distance int) bool {
			shortestLength := min(len(candidate), len(v))

			if shortestLength <= 2 && distance >= 2 {
				return false
			}

			return true
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

func CountPrevBackslashes[T constraints.Integer](s []T, i int32) int32 {
	index := i - 1
	count := int32(0)
	for ; index >= 0; index-- {
		if s[index] == '\\' {
			count += 1
		} else {
			break
		}
	}

	return count
}

