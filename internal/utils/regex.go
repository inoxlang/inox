package utils

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// RegexForRange returns a regex (not between ^$) that matches all numbers in the range [min, max].
// The main logic of the implementation is the same as https://github.com/voronind/range-regex (by Dmitry Voronin, BSD 2-Clause license).
func RegexForRange(min, max int) string {

	splitToRanges := func(min, max int) []int {
		stopsSet := map[int]struct{}{max: {}}

		ninesCount := 1
		stop := replaceEndWithNines(min, ninesCount)
		for min <= stop && stop < max {
			stopsSet[stop] = struct{}{}

			ninesCount++
			stop = replaceEndWithNines(min, ninesCount)
		}

		zerosCount := 1
		stop = replaceEndWithZeros(max+1, zerosCount) - 1
		for min < stop && stop <= max {
			stopsSet[stop] = struct{}{}

			zerosCount++
			stop = replaceEndWithZeros(max+1, zerosCount) - 1
		}

		var stops []int
		for stop := range stopsSet {
			stops = append(stops, stop)
		}

		sort.Slice(stops, func(i, j int) bool { return stops[i] < stops[j] })
		return stops
	}

	splitToPatterns := func(min, max int) []string {
		subPatterns := []string{}
		start := min
		for _, stop := range splitToRanges(min, max) {
			subPatterns = append(subPatterns, rangeToPattern(start, stop))
			start = stop + 1
		}

		return subPatterns
	}

	var positiveSubpatterns []string
	var negativeSubpatterns []string

	if min < 0 {
		absMin := 1
		if max < 0 {
			absMin = Abs(max)
		}
		absMax := Abs(min)

		negativeSubpatterns = splitToPatterns(absMin, absMax)
	}

	if max >= 0 {
		if min <= 0 {
			positiveSubpatterns = splitToPatterns(0, max)
		} else {
			positiveSubpatterns = splitToPatterns(min, max)
		}
	}

	negativeOnlySubpatterns := MapSlice(getSharedUnsharedElements(negativeSubpatterns, positiveSubpatterns, false), func(s string) string {
		return "-" + s
	})
	positiveOnlySubpatterns := getSharedUnsharedElements(positiveSubpatterns, negativeSubpatterns, false)
	intersectedSubpatterns := MapSlice(getSharedUnsharedElements(negativeSubpatterns, positiveSubpatterns, true), func(s string) string {
		return "-?" + s
	})

	subpatterns := append(append(negativeOnlySubpatterns, intersectedSubpatterns...), positiveOnlySubpatterns...)
	return "(" + strings.Join(subpatterns, "|") + ")"
}

func rangeToPattern(start, stop int) string {
	digits := func(n int) []int {
		if n == 0 {
			return []int{0}
		}

		var digits []int
		for n > 0 {
			remainder := n % 10
			digits = append([]int{remainder}, digits...)
			n = n / 10
		}

		return digits
	}

	pattern := ""
	anyDigitCount := 0
	startDigits := digits(start)
	stopDigits := digits(stop)

	for i := 0; i < len(stopDigits); i++ {
		if startDigits[i] == stopDigits[i] {
			pattern += strconv.Itoa(startDigits[i])
		} else if startDigits[i] != 0 || stopDigits[i] != 9 {
			pattern += fmt.Sprintf("[%d-%d]", startDigits[i], stopDigits[i])
		} else {
			anyDigitCount++
		}
	}

	if anyDigitCount != 0 {
		pattern += `\d`
	}

	if anyDigitCount > 1 {
		pattern += fmt.Sprintf("{%d}", anyDigitCount)
	}

	return pattern
}

func getSharedUnsharedElements(slice1, slice2 []string, shared bool) []string {
	var res []string
	for _, val1 := range slice1 {
		areShared := false
		for _, val2 := range slice2 {
			if val1 == val2 {
				areShared = true
				break
			}
		}
		if areShared == shared {
			res = append(res, val1)
		}
	}
	return res
}

func replaceEndWithNines(integer int, ninesCount int) int {
	integerStr := strconv.Itoa(integer)
	prefix := ""
	if len(integerStr) >= ninesCount {
		prefix = integerStr[:len(integerStr)-ninesCount]
	}
	suffix := ""
	for i := 0; i < ninesCount; i++ {
		suffix += "9"
	}
	result, _ := strconv.Atoi(prefix + suffix)
	return result
}

func replaceEndWithZeros(integer int, zerosCount int) int {
	return integer - integer%int(math.Pow10(zerosCount))
}
