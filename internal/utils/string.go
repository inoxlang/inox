package utils

import (
	"strings"
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
